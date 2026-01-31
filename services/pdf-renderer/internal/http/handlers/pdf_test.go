package handlers

import (
	"net/http"
	"net/http/httptest"
	"pdf-renderer/internal/config"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// ------------------------------
// TEST: computePDFCacheKey
// ------------------------------

func Test_computePDFCacheKey_stable(t *testing.T) {
	p1 := &PDFRequestParams{
		HTML:        "<b>Hello</b>",
		Format:      "A4",
		Orientation: "portrait",
		Margin:      0.5,
	}
	p2 := &PDFRequestParams{
		HTML:        "<b>Hello</b>",
		Format:      "A4",
		Orientation: "portrait",
		Margin:      0.5,
	}
	key1 := computePDFCacheKey(p1)
	key2 := computePDFCacheKey(p2)

	if key1 != key2 {
		t.Errorf("expected keys to match, got %s and %s", key1, key2)
	}
}

func Test_computePDFCacheKey_differs(t *testing.T) {
	p1 := &PDFRequestParams{
		HTML:        "<b>Hello</b>",
		Format:      "A4",
		Orientation: "portrait",
		Margin:      0.5,
	}
	p2 := &PDFRequestParams{
		HTML:        "<b>Hello world</b>", // changed HTML
		Format:      "A4",
		Orientation: "portrait",
		Margin:      0.5,
	}
	key1 := computePDFCacheKey(p1)
	key2 := computePDFCacheKey(p2)

	if key1 == key2 {
		t.Errorf("expected keys to differ, but both were %s", key1)
	}
}

// ------------------------------
// TEST: setCachedPDF & getCachedPDF
// ------------------------------
func Test_setAndGetCachedPDF(t *testing.T) {
	// Start in-memory Redis server
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer srv.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: srv.Addr(),
	})

	app := fiber.New()

	app.Get("/test", func(c *fiber.Ctx) error {
		key := "testcachekey"
		data := []byte("PDFDATA123")

		setCachedPDF(c, rdb, key, data, 1*time.Minute)

		// Retrieve immediately
		result, err := getCachedPDF(c, rdb, key, "test.pdf")
		if err != nil {
			t.Errorf("unexpected error on getCachedPDF: %v", err)
			return err
		}
		if string(result) != string(data) {
			t.Errorf("cached data mismatch: got %s, expected %s", string(result), string(data))
		}
		return nil
	})

	// Fire the request to trigger test route
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Fiber test request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("unexpected status code: got %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func Test_validateAndExtractPDFParams_valid(t *testing.T) {
	cfg := config.Config{
		Limits: struct {
			MaxHTMLBytes int `yaml:"max_html_bytes"`
			MaxPDFBytes  int `yaml:"max_pdf_bytes"`
		}{
			MaxHTMLBytes: 1024,
			MaxPDFBytes:  1024 * 1024,
		},
		PDF: struct {
			DefaultPaper    string                      `yaml:"default_paper"`
			PaperSizes      map[string]config.PaperSize `yaml:"paper_sizes"`
			TimeoutSecs     int                         `yaml:"timeout_secs"`
			ChromePath      string                      `yaml:"chrome_path"`
			ChromeNoSandbox bool                        `yaml:"chrome_no_sandbox"`
			ChromePoolSize  int                         `yaml:"chrome_pool_size"`
			UserDataDir     string                      `yaml:"user_data_dir"`
		}{
			DefaultPaper: "A4",
			PaperSizes: map[string]config.PaperSize{
				"A4": {Width: 8.27, Height: 11.69},
			},
		},
	}

	app := fiber.New()
	app.Post("/validate", func(c *fiber.Ctx) error {
		params, err := validateAndExtractPDFParams(c, cfg)
		if err != nil {
			return err
		}
		if params.Filename != "output.pdf" {
			t.Errorf("expected default filename 'output.pdf', got %s", params.Filename)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	form := `html=<b>Hello World!</b>&format=A4&orientation=portrait&margin=0.5`
	req := httptest.NewRequest("POST", "/validate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("unexpected response: %v (status: %d)", err, resp.StatusCode)
	}
}

func Test_validateAndExtractPDFParams_invalidMargin(t *testing.T) {
	cfg := config.Config{
		Limits: struct {
			MaxHTMLBytes int `yaml:"max_html_bytes"`
			MaxPDFBytes  int `yaml:"max_pdf_bytes"`
		}{
			MaxHTMLBytes: 1024,
			MaxPDFBytes:  1024 * 1024,
		},
		PDF: struct {
			DefaultPaper    string                      `yaml:"default_paper"`
			PaperSizes      map[string]config.PaperSize `yaml:"paper_sizes"`
			TimeoutSecs     int                         `yaml:"timeout_secs"`
			ChromePath      string                      `yaml:"chrome_path"`
			ChromeNoSandbox bool                        `yaml:"chrome_no_sandbox"`
			ChromePoolSize  int                         `yaml:"chrome_pool_size"`
			UserDataDir     string                      `yaml:"user_data_dir"`
		}{
			DefaultPaper: "A4",
			PaperSizes: map[string]config.PaperSize{
				"A4": {Width: 8.27, Height: 11.69},
			},
		},
	}

	app := fiber.New()
	app.Post("/validate", func(c *fiber.Ctx) error {
		_, err := validateAndExtractPDFParams(c, cfg)
		if err == nil {
			t.Fatal("expected error for invalid margin, got nil")
		}
		return err
	})

	form := `html=<b>Hi</b>&format=A4&orientation=portrait&margin=abc`
	req := httptest.NewRequest("POST", "/validate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func Test_NewPDFService_Initialization(t *testing.T) {
	cfg := config.Config{}
	cfg.PDF.DefaultPaper = "A4"
	cfg.PDF.PaperSizes = map[string]config.PaperSize{
		"A4": {Width: 8.27, Height: 11.69},
	}
	cfg.PDF.TimeoutSecs = 10
	cfg.PDF.ChromePath = "/usr/bin/chromium"
	cfg.PDF.ChromeNoSandbox = true

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // consider mocking in real tests
	})

	svc := NewPDFService(cfg, rdb)
	if svc == nil {
		t.Fatal("expected PDFService instance, got nil")
	}
	if svc.Config.PDF.DefaultPaper != "A4" {
		t.Errorf("unexpected default paper: %s", svc.Config.PDF.DefaultPaper)
	}
}

func TestPDFService_getChromePool_DisabledReturnsNil(t *testing.T) {
	var cfg config.Config
	cfg.PDF.ChromePoolSize = 0

	svc := NewPDFService(cfg, nil)
	pool, err := svc.getChromePool()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if pool != nil {
		t.Fatalf("expected nil pool when disabled")
	}

	// Calling twice should still return nil and not attempt to initialize Chrome.
	pool, err = svc.getChromePool()
	if err != nil {
		t.Fatalf("expected nil error on second call, got %v", err)
	}
	if pool != nil {
		t.Fatalf("expected nil pool on second call")
	}
}

func Test_validateAndExtractURLParams_valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<b>Hello</b>"))
	}))
	defer srv.Close()

	cfg := config.Config{
		Limits: struct {
			MaxHTMLBytes int `yaml:"max_html_bytes"`
			MaxPDFBytes  int `yaml:"max_pdf_bytes"`
		}{
			MaxHTMLBytes: 1024,
			MaxPDFBytes:  1024 * 1024,
		},
		PDF: struct {
			DefaultPaper    string                      `yaml:"default_paper"`
			PaperSizes      map[string]config.PaperSize `yaml:"paper_sizes"`
			TimeoutSecs     int                         `yaml:"timeout_secs"`
			ChromePath      string                      `yaml:"chrome_path"`
			ChromeNoSandbox bool                        `yaml:"chrome_no_sandbox"`
			ChromePoolSize  int                         `yaml:"chrome_pool_size"`
			UserDataDir     string                      `yaml:"user_data_dir"`
		}{
			DefaultPaper: "A4",
			PaperSizes: map[string]config.PaperSize{
				"A4": {Width: 8.27, Height: 11.69},
			},
			TimeoutSecs: 5,
		},
	}

	app := fiber.New()
	app.Get("/validate", func(c *fiber.Ctx) error {
		params, err := validateAndExtractURLParams(c, cfg)
		if err != nil {
			return err
		}
		if params.Filename != "output.pdf" {
			t.Errorf("expected default filename 'output.pdf', got %s", params.Filename)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/validate?url="+srv.URL+"&format=A4&orientation=portrait&margin=0.5", nil)
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("unexpected response: %v (status: %d)", err, resp.StatusCode)
	}
}

func Test_validateAndExtractURLParams_invalidURL(t *testing.T) {
	cfg := config.Config{}
	cfg.PDF.DefaultPaper = "A4"
	cfg.PDF.PaperSizes = map[string]config.PaperSize{
		"A4": {Width: 8.27, Height: 11.69},
	}

	app := fiber.New()
	app.Get("/validate", func(c *fiber.Ctx) error {
		_, err := validateAndExtractURLParams(c, cfg)
		if err == nil {
			t.Fatal("expected error for invalid URL, got nil")
		}
		return err
	})

	req := httptest.NewRequest("GET", "/validate?url=ftp://example.com", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}
