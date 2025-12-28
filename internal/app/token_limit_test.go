package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
	u "html2pdf/internal/utils"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenRateLimitMiddleware(t *testing.T) {
	token := "test-token"
	limit := 2

	u.LoadTokensFromMap(map[string]int{token: limit})

	u.AppConfig.RateLimiter.Interval = time.Hour

	rateLimitStore = newMemStore()
	tokenLimiterCache.Lock()
	tokenLimiterCache.handlers = nil
	tokenLimiterCache.Unlock()

	app := fiber.New()
	app.Use(keyauth.New(keyauth.Config{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			return u.ValidateToken(key), nil
		},
	}))
	app.Use(rateLimitMiddleware())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	makeReq := func() *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-API-Key", token)
		return req
	}

	for i := 0; i < limit; i++ {
		resp, err := app.Test(makeReq(), -1)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200 but got %d", resp.StatusCode)
		}
	}

	resp, err := app.Test(makeReq(), -1)
	if err != nil {
		t.Fatalf("exceed request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected 429 but got %d", resp.StatusCode)
	}
}
