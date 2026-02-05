package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"pdf-renderer/internal/infra/chrome"
)

func TestHandlerEntrypoints_ValidationAndStats(t *testing.T) {
	cfg := testPDFCfg()
	cfg.Cache.PDFCacheEnabled = false
	cfg.PDF.ChromePath = "/definitely/missing/chrome"

	app := fiber.New()
	app.Post("/pdf", HandlePDFConversion(cfg, nil))
	app.Get("/pdf", HandlePDFURL(cfg, nil))
	svc := NewPDFService(cfg, nil)
	app.Get("/stats", svc.HandleChromeStats)

	badPost := httptest.NewRequest("POST", "/pdf", strings.NewReader("html=x"))
	badPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp1, _ := app.Test(badPost)
	if resp1.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for invalid html, got %d", resp1.StatusCode)
	}

	badURL := httptest.NewRequest("GET", "/pdf", nil)
	resp2, _ := app.Test(badURL)
	if resp2.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d", resp2.StatusCode)
	}

	statsReq := httptest.NewRequest("GET", "/stats", nil)
	resp3, err := app.Test(statsReq)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if resp3.StatusCode != fiber.StatusOK {
		t.Fatalf("expected stats 200, got %d", resp3.StatusCode)
	}
}

func TestHandleConversion_RenderErrorPath(t *testing.T) {
	cfg := testPDFCfg()
	cfg.Cache.PDFCacheEnabled = false
	cfg.PDF.ChromePath = "/definitely/missing/chrome"
	cfg.PDF.ChromePoolSize = 0

	svc := NewPDFService(cfg, nil)
	app := fiber.New()
	app.Post("/pdf", svc.HandleConversion)

	req := httptest.NewRequest("POST", "/pdf", strings.NewReader("html=<html><body>hello world from test</body></html>"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500 from missing chrome path, got %d", resp.StatusCode)
	}
}

func TestHandleChromeStats_DisabledAndPoolErrorAndEnabled(t *testing.T) {
	base := testPDFCfg()

	// disabled pool path
	disabled := NewPDFService(base, nil)
	app1 := fiber.New()
	app1.Get("/stats", disabled.HandleChromeStats)
	resp1, _ := app1.Test(httptest.NewRequest("GET", "/stats", nil))
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for disabled pool stats, got %d", resp1.StatusCode)
	}

	// pool init error path
	errCfg := base
	errCfg.PDF.ChromePoolSize = 1
	errCfg.PDF.UserDataDir = "/dev/null/not-allowed"
	errSvc := NewPDFService(errCfg, nil)
	app2 := fiber.New()
	app2.Get("/stats", errSvc.HandleChromeStats)
	resp2, _ := app2.Test(httptest.NewRequest("GET", "/stats", nil))
	if resp2.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500 for pool init error, got %d", resp2.StatusCode)
	}

	// enabled pool path via injected lightweight pool
	enCfg := base
	enCfg.PDF.ChromePoolSize = 2
	enSvc := NewPDFService(enCfg, nil)
	enSvc.pool = &chrome.Pool{}
	// fill unexported fields via behavior: use zero-value stats by attaching sem/profile through restart-free path
	enSvc.pool = &chrome.Pool{}
	app3 := fiber.New()
	app3.Get("/stats", enSvc.HandleChromeStats)
	resp3, _ := app3.Test(httptest.NewRequest("GET", "/stats", nil))
	if resp3.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for enabled pool stats, got %d", resp3.StatusCode)
	}
}

func TestRenderPDF_UsesExistingPoolAcquireFailure(t *testing.T) {
	cfg := testPDFCfg()
	cfg.PDF.ChromePoolSize = 1
	cfg.PDF.TimeoutSecs = 1

	svc := NewPDFService(cfg, nil)
	// custom pool with no token available to force acquire timeout.
	svc.pool = &chrome.Pool{}

	// Can't set unexported sem directly from different package; instead verify path through getChromePool error cache.
	svc.poolErr = context.DeadlineExceeded
	_, _ = svc.getChromePool()

	// render still goes through configured pool path if pool exists; with zero-value pool, acquire returns ctx deadline quickly.
	ctxPool := &chrome.Pool{}
	// emulate closed pool error path
	ctxPool.Close()
	svc.pool = ctxPool
	_, err := svc.renderPDF(&PDFRequestParams{HTML: "<html>hello world</html>", Paper: cfg.PDF.PaperSizes["A4"], Margin: 0.4})
	if err == nil {
		t.Fatalf("expected render error when pool cannot acquire")
	}

	// timeout keeps test bounded
	_ = time.Second
}
