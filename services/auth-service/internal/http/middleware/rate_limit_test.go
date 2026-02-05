package middleware

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	memoryStorage "github.com/gofiber/storage/memory/v2"
)

type fakeTokenRater struct{ limit int }

func (f fakeTokenRater) RateLimit(token string) int { return f.limit }

func TestTokenRateLimit_Enforced(t *testing.T) {
	app := fiber.New()
	store := memoryStorage.New()
	cfg := RateLimitConfig{
		RateInterval:           time.Hour,
		EnableTokenRateLimiter: true,
	}
	cache := NewLimiterCache()

	// Pretend auth already happened
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("api_key", "abc")
		return c.Next()
	})
	app.Use(TokenRateLimit(cfg, fakeTokenRater{limit: 1}, store, cache))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/", nil)

	resp1, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	resp2, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp2.StatusCode)
	}
}

func TestTokenRateLimit_Disabled(t *testing.T) {
	app := fiber.New()
	store := memoryStorage.New()
	cfg := RateLimitConfig{
		RateInterval:           time.Hour,
		EnableTokenRateLimiter: false,
	}
	cache := NewLimiterCache()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("api_key", "abc")
		return c.Next()
	})
	app.Use(TokenRateLimit(cfg, fakeTokenRater{limit: 1}, store, cache))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUserRateLimit_PublicLimitedButTokenBypasses(t *testing.T) {
	app := fiber.New()
	store := memoryStorage.New()
	cfg := RateLimitConfig{
		RateInterval:      time.Hour,
		EnableUserLimiter: true,
		UserLimit:         1,
	}

	app.Use(UserRateLimit(cfg, store))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	publicReq, _ := http.NewRequest(http.MethodGet, "/", nil)
	publicReq.Header.Set("User-Agent", "public-client")
	resp1, err := app.Test(publicReq)
	if err != nil {
		t.Fatalf("public request failed: %v", err)
	}
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected first public request to pass, got %d", resp1.StatusCode)
	}

	resp2, err := app.Test(publicReq)
	if err != nil {
		t.Fatalf("public request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected second public request to be rate limited, got %d", resp2.StatusCode)
	}

	tokenReq, _ := http.NewRequest(http.MethodGet, "/", nil)
	tokenReq.Header.Set("User-Agent", "public-client")
	tokenReq.Header.Set("X-API-Key", "abc")
	appWithToken := fiber.New()
	appWithToken.Use(func(c *fiber.Ctx) error {
		c.Locals("api_key", c.Get("X-API-Key"))
		return c.Next()
	})
	appWithToken.Use(UserRateLimit(cfg, memoryStorage.New()))
	appWithToken.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp3, err := appWithToken.Test(tokenReq)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	if resp3.StatusCode != fiber.StatusOK {
		t.Fatalf("expected token-authenticated request to bypass user limiter, got %d", resp3.StatusCode)
	}
}

func TestTokenRateLimit_TooManyRequestsBody(t *testing.T) {
	app := fiber.New()
	store := memoryStorage.New()
	cfg := RateLimitConfig{
		RateInterval:           time.Hour,
		EnableTokenRateLimiter: true,
	}
	cache := NewLimiterCache()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("api_key", "abc")
		return c.Next()
	})
	app.Use(TokenRateLimit(cfg, fakeTokenRater{limit: 1}, store, cache))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	_, _ = app.Test(req)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Too many requests") {
		t.Fatalf("expected JSON body to mention rate limit, got %q", string(body))
	}
}
