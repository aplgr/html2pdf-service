package middleware

import (
	"net/http"
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
