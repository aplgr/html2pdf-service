package server

import (
	"net/http"
	"testing"

	"pdf-renderer/internal/config"
)

func minimalConfig() config.Config {
	var cfg config.Config
	cfg.PDF.DefaultPaper = "A4"
	cfg.PDF.PaperSizes = map[string]config.PaperSize{"A4": {Width: 8.27, Height: 11.69}}
	cfg.PDF.TimeoutSecs = 1
	cfg.Limits.MaxHTMLBytes = 1024 * 1024
	cfg.Limits.MaxPDFBytes = 5 * 1024 * 1024
	cfg.Cache.PDFCacheEnabled = false
	return cfg
}

func TestNew_RoutesAndJSON404(t *testing.T) {
	app := New(Deps{Config: minimalConfig(), Redis: nil})

	reqStats, _ := http.NewRequest(http.MethodGet, "/v0/chrome/stats", nil)
	respStats, err := app.Test(reqStats)
	if err != nil {
		t.Fatalf("stats request failed: %v", err)
	}
	if respStats.StatusCode != http.StatusOK {
		t.Fatalf("expected /v0/chrome/stats 200, got %d", respStats.StatusCode)
	}

	req404, _ := http.NewRequest(http.MethodGet, "/does-not-exist", nil)
	resp404, err := app.Test(req404)
	if err != nil {
		t.Fatalf("404 request failed: %v", err)
	}
	if resp404.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp404.StatusCode)
	}
	if got := resp404.Header.Get("Content-Type"); got == "" {
		t.Fatalf("expected JSON error response content type")
	}
}
