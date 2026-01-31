package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"pdf-renderer/internal/config"
	"pdf-renderer/internal/infra/chrome"
	"pdf-renderer/internal/infra/logging"
)

// PDFRequestParams holds validated input parameters.
type PDFRequestParams struct {
	HTML        string
	URL         string
	Format      string
	Orientation string
	Margin      float64
	Filename    string
	Paper       config.PaperSize
}

// PDFService bundles configuration and dependencies for PDF rendering.
type PDFService struct {
	Config *config.Config
	Redis  *redis.Client

	poolMu  sync.Mutex
	pool    *chrome.Pool
	poolErr error
}

// HandlePDFConversion returns a Fiber handler for PDF conversion requests.
func HandlePDFConversion(cfg config.Config, rdb *redis.Client) fiber.Handler {
	svc := NewPDFService(cfg, rdb)
	return svc.HandleConversion
}

// HandlePDFURL returns a Fiber handler for URL-based PDF conversion requests.
func HandlePDFURL(cfg config.Config, rdb *redis.Client) fiber.Handler {
	svc := NewPDFService(cfg, rdb)
	return svc.HandleURLConversion
}

// NewPDFService creates a new PDFService instance.
func NewPDFService(cfg config.Config, rdb *redis.Client) *PDFService {
	return &PDFService{
		Config: &cfg, // convert value to pointer
		Redis:  rdb,
	}
}

func (svc *PDFService) getChromePool() (*chrome.Pool, error) {
	svc.poolMu.Lock()
	defer svc.poolMu.Unlock()

	if svc.Config.PDF.ChromePoolSize <= 0 {
		return nil, nil
	}
	if svc.pool != nil {
		return svc.pool, nil
	}
	pool, err := chrome.NewPool(*svc.Config)
	if err != nil {
		svc.poolErr = err
		return nil, err
	}
	svc.pool = pool
	return svc.pool, nil
}

// HandleConversion generates a new PDF or serves a cached copy.
func (svc *PDFService) HandleConversion(c *fiber.Ctx) error {
	params, err := validateAndExtractPDFParams(c, *svc.Config)
	if err != nil {
		return err
	}
	return svc.processPDFGeneration(c, params)
}

// HandleURLConversion fetches HTML from a URL and generates a PDF.
func (svc *PDFService) HandleURLConversion(c *fiber.Ctx) error {
	params, err := validateAndExtractURLParams(c, *svc.Config)
	if err != nil {
		return err
	}
	return svc.processPDFGeneration(c, params)
}

// processPDFGeneration handles caching and PDF rendering.
func (svc *PDFService) processPDFGeneration(c *fiber.Ctx, params *PDFRequestParams) error {
	cacheKey := computePDFCacheKey(params)

	// Try to serve from Redis cache
	if svc.Redis != nil && svc.Config.Cache.PDFCacheEnabled {
		if cached, err := getCachedPDF(c, svc.Redis, cacheKey, params.Filename); err == nil && cached != nil {
			return c.Send(cached)
		}
	}

	// Generate PDF
	pdfBuf, err := svc.renderPDF(params)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// Log the underlying error so we can distinguish between:
			// - Chrome pool init warmup timeout
			// - Pool acquire timeout (no free tab)
			// - Actual render timeout
			logging.Error("PDF generation timeout", "timeout_secs", svc.Config.PDF.TimeoutSecs, "error", err.Error())
			return fiber.NewError(fiber.StatusRequestTimeout, "PDF rendering took too long")
		}
		if chrome.IsSessionInterrupted(err) {
			logging.Error("Chrome session interrupted", "error", err.Error())
			return fiber.NewError(fiber.StatusServiceUnavailable, "Chrome session interrupted")
		}
		logging.Error("PDF generation failed", "error", err.Error())
		return fiber.NewError(fiber.StatusInternalServerError, "PDF generation failed: "+err.Error())
	}

	if len(pdfBuf) > svc.Config.Limits.MaxPDFBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "PDF exceeds allowed size")
	}

	// Cache PDF
	if svc.Redis != nil && svc.Config.Cache.PDFCacheEnabled {
		setCachedPDF(c, svc.Redis, cacheKey, pdfBuf, svc.Config.Cache.PDFCacheTTL)
	}

	requestID := c.Get("X-Request-ID")
	logging.Info("PDF generated", "filename", params.Filename, "request_id", requestID)

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "attachment; filename="+params.Filename)
	return c.Send(pdfBuf)
}

func (svc *PDFService) renderPDF(params *PDFRequestParams) ([]byte, error) {
	pool, err := svc.getChromePool()
	if err != nil {
		return nil, err
	}
	if pool == nil {
		// Fallback: start a new Chrome instance per request.
		return renderPDFWithChrome(params.HTML, params.URL, params.Paper, params.Margin, *svc.Config)
	}

	timeout := time.Duration(svc.Config.PDF.TimeoutSecs) * time.Second

	runOnce := func() ([]byte, error) {
		acquireCtx, acquireCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer acquireCancel()

		tab, err := pool.Acquire(acquireCtx)
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithTimeout(tab.Ctx, timeout)
		pdfBuf, renderErr := renderPDFInExistingTab(ctx, params.HTML, params.URL, params.Paper, params.Margin)
		cancel()

		pool.Release(tab, renderErr)
		return pdfBuf, renderErr
	}

	pdfBuf, renderErr := runOnce()
	if renderErr != nil && chrome.IsSessionInterrupted(renderErr) {
		logging.Warn("Chrome session interrupted; restarting pool and retrying once", "error", renderErr)
		_ = pool.Restart()
		return runOnce()
	}

	return pdfBuf, renderErr
}

// validateAndExtractPDFParams validates and parses input parameters from the HTTP request.
func validateAndExtractPDFParams(c *fiber.Ctx, cfg config.Config) (*PDFRequestParams, error) {
	html := c.FormValue("html")

	if len(html) < 10 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid HTML: content too short or missing")
	}

	if len(html) > cfg.Limits.MaxHTMLBytes {
		return nil, fiber.NewError(fiber.StatusRequestEntityTooLarge, fmt.Sprintf("HTML input exceeds %d bytes", cfg.Limits.MaxHTMLBytes))
	}

	format := strings.ToUpper(c.FormValue("format"))
	if format != "" {
		if _, ok := cfg.PDF.PaperSizes[format]; !ok {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid format: not supported")
		}
	}

	orientation := strings.ToLower(c.FormValue("orientation"))
	if orientation != "" && orientation != "portrait" && orientation != "landscape" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid orientation: must be 'portrait' or 'landscape'")
	}

	margin := 0.4
	if marginStr := c.FormValue("margin"); marginStr != "" {
		m, err := strconv.ParseFloat(marginStr, 64)
		if err != nil || m < 0.1 || m > 2.0 {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid margin: must be a float between 0.1 and 2.0")
		}
		margin = m
	}

	filename := c.FormValue("filename")
	if filename == "" {
		filename = "output.pdf"
	} else {
		if !strings.HasSuffix(filename, ".pdf") {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Filename must end with .pdf")
		}
		if matched := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(filename); !matched {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Filename contains invalid characters")
		}
	}

	paper, ok := cfg.PDF.PaperSizes[format]
	if !ok {
		paper, ok = cfg.PDF.PaperSizes[cfg.PDF.DefaultPaper]
		if !ok {
			return nil, fiber.NewError(fiber.StatusInternalServerError, "Default paper size not configured")
		}
	}

	if orientation == "landscape" {
		paper.Width, paper.Height = paper.Height, paper.Width
	}

	return &PDFRequestParams{
		HTML:        html,
		Format:      format,
		Orientation: orientation,
		Margin:      margin,
		Filename:    filename,
		Paper:       paper,
	}, nil
}

// validateAndExtractURLParams validates query parameters and fetches HTML from the provided URL.
func validateAndExtractURLParams(c *fiber.Ctx, cfg config.Config) (*PDFRequestParams, error) {
	urlStr := c.Query("url")
	if urlStr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid URL: missing")
	}

	parsed, err := neturl.ParseRequestURI(urlStr)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid URL: must be HTTP or HTTPS")
	}

	format := strings.ToUpper(c.Query("format"))
	if format != "" {
		if _, ok := cfg.PDF.PaperSizes[format]; !ok {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid format: not supported")
		}
	}

	orientation := strings.ToLower(c.Query("orientation"))
	if orientation != "" && orientation != "portrait" && orientation != "landscape" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid orientation: must be 'portrait' or 'landscape'")
	}

	margin := 0.4
	if marginStr := c.Query("margin"); marginStr != "" {
		m, err := strconv.ParseFloat(marginStr, 64)
		if err != nil || m < 0.1 || m > 2.0 {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Invalid margin: must be a float between 0.1 and 2.0")
		}
		margin = m
	}

	filename := c.Query("filename")
	if filename == "" {
		filename = "output.pdf"
	} else {
		if !strings.HasSuffix(filename, ".pdf") {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Filename must end with .pdf")
		}
		if matched := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(filename); !matched {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Filename contains invalid characters")
		}
	}

	paper, ok := cfg.PDF.PaperSizes[format]
	if !ok {
		paper, ok = cfg.PDF.PaperSizes[cfg.PDF.DefaultPaper]
		if !ok {
			return nil, fiber.NewError(fiber.StatusInternalServerError, "Default paper size not configured")
		}
	}

	if orientation == "landscape" {
		paper.Width, paper.Height = paper.Height, paper.Width
	}

	return &PDFRequestParams{
		URL:         urlStr,
		Format:      format,
		Orientation: orientation,
		Margin:      margin,
		Filename:    filename,
		Paper:       paper,
	}, nil
}

// computePDFCacheKey creates a SHA256-based cache key based on input parameters.
func computePDFCacheKey(params *PDFRequestParams) string {
	h := sha256.New()
	if params.URL != "" {
		h.Write([]byte(params.URL))
	} else {
		h.Write([]byte(params.HTML))
	}
	h.Write([]byte(params.Format))
	h.Write([]byte(params.Orientation))
	h.Write([]byte(strconv.FormatFloat(params.Margin, 'f', 2, 64)))
	return "pdfcache:" + hex.EncodeToString(h.Sum(nil))
}

// getCachedPDF attempts to retrieve a cached PDF from Redis.
func getCachedPDF(c *fiber.Ctx, rdb *redis.Client, key, filename string) ([]byte, error) {
	ctxRedis, cancel := context.WithTimeout(c.Context(), 1*time.Second)
	defer cancel()

	cached, err := rdb.Get(ctxRedis, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		logging.Warn("Redis read failed", "error", err)
		return nil, err
	}

	logging.Info("PDF cache hit", "key", key)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "attachment; filename="+filename)
	return cached, nil
}

// setCachedPDF stores a PDF in Redis for 24 hours.
func setCachedPDF(c *fiber.Ctx, rdb *redis.Client, key string, data []byte, ttl time.Duration) {
	ctxRedis, cancel := context.WithTimeout(c.Context(), 1*time.Second)
	defer cancel()

	if ttl <= 0 {
		ttl = 1 * time.Minute
	}

	if err := rdb.Set(ctxRedis, key, data, ttl).Err(); err != nil {
		logging.Warn("Redis write failed", "error", err)
	}
}

// renderPDFWithChrome uses headless Chrome via chromedp to render the HTML to PDF.
func renderPDFWithChrome(html, url string, paper config.PaperSize, margin float64, cfg config.Config) ([]byte, error) {

	tmpDir, err := os.MkdirTemp("", "chromedata-*")
	if err != nil {
		return nil, fmt.Errorf("cannot create temp profile dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(tmpDir),
		// Force software rendering and avoid Vulkan/ANGLE issues in minimal container environments.
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-gpu-compositing", true),
		chromedp.Flag("disable-features", "Vulkan,UseSkiaRenderer"),
		chromedp.Flag("use-gl", "swiftshader"),
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	if cfg.PDF.ChromePath != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(cfg.PDF.ChromePath))
	}
	if cfg.PDF.ChromeNoSandbox {
		allocatorOptions = append(allocatorOptions, chromedp.Flag("no-sandbox", true))
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), allocatorOptions...)
	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	timeout := time.Duration(cfg.PDF.TimeoutSecs) * time.Second
	chromeCtx, cancel = context.WithTimeout(chromeCtx, timeout)
	defer cancel()

	pdfBuf, err := renderPDFInExistingTab(chromeCtx, html, url, paper, margin)

	if err != nil {
		return nil, err
	}
	return pdfBuf, nil
}

// renderPDFInExistingTab renders either raw HTML or a remote URL into PDF within a pre-existing chromedp tab.
func renderPDFInExistingTab(ctx context.Context, html, url string, paper config.PaperSize, margin float64) ([]byte, error) {
	var pdfBuf []byte
	var actions []chromedp.Action

	if url != "" {
		actions = append(actions,
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
	} else {
		actions = append(actions,
			chromedp.Navigate("about:blank"),
			chromedp.ActionFunc(func(ctx context.Context) error {
				frame, err := page.GetFrameTree().Do(ctx)
				if err != nil {
					return err
				}
				return page.SetDocumentContent(frame.Frame.ID, html).Do(ctx)
			}),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
	}

	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return waitForRenderReady(ctx, 15*time.Second)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(paper.Width).
				WithPaperHeight(paper.Height).
				WithMarginTop(margin).
				WithMarginBottom(margin).
				WithMarginLeft(margin).
				WithMarginRight(margin).
				Do(ctx)
			return err
		}),
	)

	if err := chromedp.Run(ctx, actions...); err != nil {
		return nil, err
	}
	return pdfBuf, nil
}

// waitForRenderReady waits until the page finished loading and critical assets are available.
// This avoids rendering PDFs before CDN assets (CSS/fonts/images) are loaded.
func waitForRenderReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	// 1) Document readyState
	for time.Now().Before(deadline) {
		var state string
		if err := chromedp.Evaluate(`document.readyState`, &state).Do(ctx); err != nil {
			return err
		}
		if state == "complete" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 2) Optional explicit hook: allow examples to signal "I'm ready"
	// If the flag is undefined, we don't block on it.
	for time.Now().Before(deadline) {
		var ok bool
		expr := `(typeof window.__HTML2PDF_READY__ === "undefined") || (window.__HTML2PDF_READY__ === true)`
		if err := chromedp.Evaluate(expr, &ok).Do(ctx); err != nil {
			return err
		}
		if ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 3) Fonts loaded (if Font Loading API exists)
	for time.Now().Before(deadline) {
		var loaded bool
		expr := `(document.fonts && document.fonts.status) ? (document.fonts.status === "loaded") : true`
		if err := chromedp.Evaluate(expr, &loaded).Do(ctx); err != nil {
			return err
		}
		if loaded {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 4) Images loaded (complete==true means loaded or failed; we mainly avoid "still downloading")
	for time.Now().Before(deadline) {
		var done bool
		expr := `Array.from(document.images || []).every(img => img.complete)`
		if err := chromedp.Evaluate(expr, &done).Do(ctx); err != nil {
			return err
		}
		if done {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// HandleChromeStats exposes basic observability for the Chrome pool (capacity / idle / in_use).
func (svc *PDFService) HandleChromeStats(c *fiber.Ctx) error {
	pool, err := svc.getChromePool()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Chrome pool init failed: "+err.Error())
	}

	// Pool disabled.
	if pool == nil {
		return c.JSON(fiber.Map{
			"enabled":        false,
			"capacity":       0,
			"idle":           0,
			"in_use":         0,
			"pool_size_conf": svc.Config.PDF.ChromePoolSize,
			"profile_dir":    "",
			"timeout_secs":   svc.Config.PDF.TimeoutSecs,
			"restarts":       0,
		})
	}

	s := pool.Stats(svc.Config.PDF.TimeoutSecs)
	return c.JSON(fiber.Map{
		"enabled":        s.Enabled,
		"capacity":       s.Capacity,
		"idle":           s.Idle,
		"in_use":         s.InUse,
		"pool_size_conf": s.PoolSizeConf,
		"profile_dir":    s.ProfileDir,
		"timeout_secs":   svc.Config.PDF.TimeoutSecs,
		"restarts":       s.Restarts,
		"last_restart":   s.LastRestart,
	})
}
