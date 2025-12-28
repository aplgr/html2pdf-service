package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	u "html2pdf/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
)

func TestTokenBasedLimitOverridesUserBasedLimit(t *testing.T) {
	userLimit := 2
	interval := time.Hour

	token := "test-token"
	// Set a high token limit so only the user limiter would block if it were applied.
	u.LoadTokensFromMap(map[string]int{token: 100})
	u.AppConfig.RateLimiter.Interval = interval

	// Shared store for both limiters to reproduce the real middleware chain.
	rateLimitStore = newMemStore()
	tokenLimiterCache.Lock()
	tokenLimiterCache.handlers = nil
	tokenLimiterCache.Unlock()

	cfg := u.Config{}
	cfg.RateLimiter.EnableUserLimiter = true
	cfg.RateLimiter.UserLimit = userLimit
	cfg.RateLimiter.Interval = interval

	app := fiber.New()
	app.Use(keyauth.New(keyauth.Config{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			return u.ValidateToken(key), nil
		},
		// Allow anonymous requests to hit the user limiter.
		Next: func(c *fiber.Ctx) bool {
			return c.Method() == fiber.MethodOptions || c.Get("X-API-Key") == ""
		},
	}))
	app.Use(rateLimitMiddleware())
	app.Use(userRateLimitMiddleware(cfg))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	makeReq := func(withToken bool) *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.RemoteAddr = "1.2.3.4:5678"
		if withToken {
			req.Header.Set("X-API-Key", token)
		}
		return req
	}

	// Exhaust anonymous user limit.
	for i := 0; i < userLimit; i++ {
		resp, err := app.Test(makeReq(false), -1)
		if err != nil {
			t.Fatalf("anonymous request %d failed: %v", i+1, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200 but got %d", resp.StatusCode)
		}
	}
	resp, err := app.Test(makeReq(false), -1)
	if err != nil {
		t.Fatalf("anonymous exceed request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected 429 but got %d", resp.StatusCode)
	}

	// Now authenticate via token: this must NOT be blocked by the user limiter.
	resp, err = app.Test(makeReq(true), -1)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for token request but got %d", resp.StatusCode)
	}
}
