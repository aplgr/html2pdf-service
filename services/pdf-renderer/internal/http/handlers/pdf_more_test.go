package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"pdf-renderer/internal/config"
)

func testPDFCfg() config.Config {
	var cfg config.Config
	cfg.PDF.DefaultPaper = "A4"
	cfg.PDF.PaperSizes = map[string]config.PaperSize{"A4": {Width: 8.27, Height: 11.69}, "LETTER": {Width: 8.5, Height: 11}}
	cfg.PDF.TimeoutSecs = 1
	cfg.Limits.MaxHTMLBytes = 1024 * 1024
	cfg.Limits.MaxPDFBytes = 1024 * 1024
	cfg.Cache.PDFCacheEnabled = true
	cfg.Cache.PDFCacheTTL = time.Minute
	return cfg
}

func TestValidateAndExtractPDFParams_ErrorCases(t *testing.T) {
	cfg := testPDFCfg()
	app := fiber.New()
	app.Post("/v", func(c *fiber.Ctx) error {
		_, err := validateAndExtractPDFParams(c, cfg)
		return err
	})

	tests := []struct {
		name string
		form string
		code int
	}{
		{"missing html", "format=A4", fiber.StatusBadRequest},
		{"html too large", "html=" + strings.Repeat("x", cfg.Limits.MaxHTMLBytes+1), fiber.StatusRequestEntityTooLarge},
		{"invalid format", "html=<html>hello world</html>&format=B0", fiber.StatusBadRequest},
		{"invalid orientation", "html=<html>hello world</html>&orientation=diag", fiber.StatusBadRequest},
		{"invalid margin range", "html=<html>hello world</html>&margin=4.2", fiber.StatusBadRequest},
		{"invalid filename ext", "html=<html>hello world</html>&filename=file.txt", fiber.StatusBadRequest},
		{"invalid filename chars", "html=<html>hello world</html>&filename=bad name.pdf", fiber.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v", strings.NewReader(tc.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != tc.code {
				t.Fatalf("expected %d got %d", tc.code, resp.StatusCode)
			}
		})
	}
}

func TestValidateAndExtractPDFParams_DefaultPaperMissing(t *testing.T) {
	cfg := testPDFCfg()
	cfg.PDF.PaperSizes = map[string]config.PaperSize{}
	app := fiber.New()
	app.Post("/v", func(c *fiber.Ctx) error {
		_, err := validateAndExtractPDFParams(c, cfg)
		return err
	})
	req := httptest.NewRequest("POST", "/v", strings.NewReader("html=<html>hello world</html>"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", resp.StatusCode)
	}
}

func TestValidateAndExtractURLParams_ErrorCases(t *testing.T) {
	cfg := testPDFCfg()
	app := fiber.New()
	app.Get("/v", func(c *fiber.Ctx) error {
		_, err := validateAndExtractURLParams(c, cfg)
		return err
	})

	tests := []struct {
		url  string
		code int
	}{
		{"/v", fiber.StatusBadRequest},
		{"/v?url=ftp://example.com", fiber.StatusBadRequest},
		{"/v?url=https://example.com&format=bad", fiber.StatusBadRequest},
		{"/v?url=https://example.com&orientation=diag", fiber.StatusBadRequest},
		{"/v?url=https://example.com&margin=9", fiber.StatusBadRequest},
		{"/v?url=https://example.com&filename=x.txt", fiber.StatusBadRequest},
	}
	for _, tc := range tests {
		req := httptest.NewRequest("GET", tc.url, nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != tc.code {
			t.Fatalf("url=%s expected %d got %d", tc.url, tc.code, resp.StatusCode)
		}
	}
}

func TestSetCachedPDF_DefaultTTLAndProcessCacheHit(t *testing.T) {
	mrs, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mrs.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mrs.Addr()})
	cfg := testPDFCfg()
	svc := NewPDFService(cfg, rdb)

	app := fiber.New()
	app.Get("/cache", func(c *fiber.Ctx) error {
		setCachedPDF(c, rdb, "k", []byte("pdf"), 0)
		ttl := mrs.TTL("k")
		if ttl < 50*time.Second || ttl > 70*time.Second {
			t.Fatalf("expected default ttl around 1m, got %v", ttl)
		}

		params := &PDFRequestParams{HTML: "<html>hello world</html>", Format: "A4", Orientation: "portrait", Margin: 0.4, Filename: "x.pdf"}
		key := computePDFCacheKey(params)
		if err := rdb.Set(c.Context(), key, []byte("cached-pdf"), time.Minute).Err(); err != nil {
			return err
		}
		return svc.processPDFGeneration(c, params)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/cache", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}
}

func TestRenderPDFWithChrome_ErrorWhenBinaryMissing(t *testing.T) {
	cfg := testPDFCfg()
	cfg.PDF.ChromePath = "/definitely/missing/chrome"
	_, err := renderPDFWithChrome("<html>hello world</html>", "", cfg.PDF.PaperSizes["A4"], 0.4, cfg)
	if err == nil {
		t.Fatalf("expected render error with missing chrome binary")
	}
}

func TestRenderPDFInExistingTab_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := renderPDFInExistingTab(ctx, "<html>hello world</html>", "", config.PaperSize{Width: 8.27, Height: 11.69}, 0.4)
	if err == nil {
		t.Fatalf("expected canceled-context error")
	}
}

func TestWaitForRenderReady_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRenderReady(ctx, 10*time.Millisecond); err == nil {
		t.Fatalf("expected canceled-context error")
	}
}
