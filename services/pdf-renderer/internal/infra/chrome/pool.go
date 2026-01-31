package chrome

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"

	"pdf-renderer/internal/config"
	"pdf-renderer/internal/infra/logging"
)

// Tab represents a single-use Chrome tab (chromedp context) created from a shared browser instance.
type Tab struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// Pool keeps one long-lived Chromium process warm and limits concurrent renders.
// Instead of reusing the same tab across requests (which is often brittle after PrintToPDF),
// each Acquire creates a fresh tab context and Release closes it. Concurrency is controlled
// by a semaphore with size = chrome_pool_size.
type Pool struct {
	cfg config.Config

	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc

	sem chan struct{} // concurrency tokens

	profileDir string

	mu     sync.Mutex
	closed bool

	restarts    uint64
	lastRestart atomic.Value // stores time.Time
}

// Stats is a lightweight snapshot for observability.
type Stats struct {
	Enabled      bool   `json:"enabled"`
	Capacity     int    `json:"capacity"`
	Idle         int    `json:"idle"`
	InUse        int    `json:"in_use"`
	PoolSizeConf int    `json:"pool_size_conf"`
	ProfileDir   string `json:"profile_dir"`
	Restarts     uint64 `json:"restarts"`
	LastRestart  string `json:"last_restart,omitempty"`
}

func NewPool(cfg config.Config) (*Pool, error) {
	if cfg.PDF.ChromePoolSize <= 0 {
		return nil, fmt.Errorf("chrome pool disabled (chrome_pool_size <= 0)")
	}

	profileDir, err := createProfileDir(cfg)
	if err != nil {
		return nil, err
	}

	// chromedp.DefaultExecAllocatorOptions may be an array in some versions.
	// Convert it to a slice before using variadic expansion.
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)

	// Only override exec path when configured.
	if cfg.PDF.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.PDF.ChromePath))
	}

	// Stable-ish flags for containers.
	opts = append(opts,
		chromedp.UserDataDir(profileDir),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,

		// Avoid Vulkan/ANGLE issues in minimal container environments.
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-gpu-compositing", true),
		chromedp.Flag("disable-features", "Vulkan,UseSkiaRenderer"),
		chromedp.Flag("use-gl", "swiftshader"),

		// I still recommend shm_size in compose, but keep this as a safe default.
		chromedp.Flag("disable-dev-shm-usage", true),

		// Reduce background noise.
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-domain-reliability", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
	)

	if cfg.PDF.ChromeNoSandbox {
		opts = append(opts, chromedp.Flag("no-sandbox", true))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	p := &Pool{
		cfg:           cfg,
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		sem:           make(chan struct{}, cfg.PDF.ChromePoolSize),
		profileDir:    profileDir,
	}
	for i := 0; i < cfg.PDF.ChromePoolSize; i++ {
		p.sem <- struct{}{}
	}

	// Warm up Chrome once.
	warmupTimeout := time.Duration(cfg.PDF.TimeoutSecs) * time.Second
	if warmupTimeout < 10*time.Second {
		warmupTimeout = 10 * time.Second
	}
	warmupCtx, cancel := context.WithTimeout(browserCtx, warmupTimeout)
	_ = chromedp.Run(warmupCtx, chromedp.Navigate("about:blank"))
	cancel()

	logging.Info("Chrome pool initialized", "tabs", cfg.PDF.ChromePoolSize, "profile_dir", profileDir)
	return p, nil
}

// Acquire blocks until capacity is available or ctx is cancelled.
// It returns a fresh tab context; callers must Release it.
func (p *Pool) Acquire(ctx context.Context) (*Tab, error) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, errors.New("chrome pool is closed")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.sem:
		// ok
	}

	// Create a fresh tab for this request.
	tabCtx, cancel := chromedp.NewContext(p.browserCtx)
	return &Tab{Ctx: tabCtx, Cancel: cancel}, nil
}

// Release closes the tab and returns the capacity token.
// renderErr is ignored (kept for backwards compatibility).
func (p *Pool) Release(t *Tab, _ error) {
	if t != nil && t.Cancel != nil {
		t.Cancel()
	}

	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return
	}

	// Return token (never blocks).
	select {
	case p.sem <- struct{}{}:
	default:
	}
}

func (p *Pool) Stats(timeoutSecs int) Stats {
	p.mu.Lock()
	closed := p.closed
	profile := p.profileDir
	poolSize := p.cfg.PDF.ChromePoolSize
	p.mu.Unlock()

	capacity := cap(p.sem)
	idle := len(p.sem)
	inUse := capacity - idle

	var lastRestart string
	if v := p.lastRestart.Load(); v != nil {
		if t, ok := v.(time.Time); ok && !t.IsZero() {
			lastRestart = t.UTC().Format(time.RFC3339)
		}
	}

	return Stats{
		Enabled:      !closed && poolSize > 0,
		Capacity:     capacity,
		Idle:         idle,
		InUse:        inUse,
		PoolSizeConf: poolSize,
		ProfileDir:   profile,
		Restarts:     atomic.LoadUint64(&p.restarts),
		LastRestart:  lastRestart,
	}
}

// Restart tears down and recreates the underlying Chromium process/profile.
// This is useful when Chrome/DevTools becomes unstable (e.g. "context canceled" / target closed).
func (p *Pool) Restart() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return errors.New("chrome pool is closed")
	}
	oldProfile := p.profileDir
	// Cancel old contexts.
	if p.browserCancel != nil {
		p.browserCancel()
	}
	if p.allocCancel != nil {
		p.allocCancel()
	}

	// New profile directory.
	profileDir, err := createProfileDir(p.cfg)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	p.profileDir = profileDir

	// Recreate allocator + browser contexts.
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if p.cfg.PDF.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(p.cfg.PDF.ChromePath))
	}
	opts = append(opts,
		chromedp.UserDataDir(profileDir),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-gpu-compositing", true),
		chromedp.Flag("disable-features", "Vulkan,UseSkiaRenderer"),
		chromedp.Flag("use-gl", "swiftshader"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-domain-reliability", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
	)
	if p.cfg.PDF.ChromeNoSandbox {
		opts = append(opts, chromedp.Flag("no-sandbox", true))
	}

	p.allocCtx, p.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	p.browserCtx, p.browserCancel = chromedp.NewContext(p.allocCtx)

	atomic.AddUint64(&p.restarts, 1)
	p.lastRestart.Store(time.Now())

	p.mu.Unlock()

	// Best-effort cleanup of old profile directory.
	if oldProfile != "" {
		_ = os.RemoveAll(oldProfile)
	}

	logging.Warn("Chrome pool restarted", "profile_dir", profileDir)
	return nil
}

func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	profile := p.profileDir
	p.mu.Unlock()

	if p.browserCancel != nil {
		p.browserCancel()
	}
	if p.allocCancel != nil {
		p.allocCancel()
	}
	if profile != "" {
		_ = os.RemoveAll(profile)
	}
}

func createProfileDir(cfg config.Config) (string, error) {
	base := cfg.PDF.UserDataDir
	if base == "" {
		base = filepath.Join(os.TempDir(), "html2pdf-chrome-profile")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("cannot create user_data_dir: %w", err)
	}
	dir, err := os.MkdirTemp(base, "html2pdf-chrome-profile-")
	if err != nil {
		return "", fmt.Errorf("cannot create temp user data dir: %w", err)
	}
	return dir, nil
}

// IsSessionInterrupted is a conservative detector for chrome/chromedp session breakage.
func IsSessionInterrupted(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		// Deadlines can be real timeouts, but with pooled Chrome it's often a wedged target.
		return true
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "target closed"),
		strings.Contains(msg, "session closed"),
		strings.Contains(msg, "websocket"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "io: read/write on closed pipe"),
		strings.Contains(msg, "eof"):
		return true
	}
	return false
}
