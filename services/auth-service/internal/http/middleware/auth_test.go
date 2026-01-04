package middleware

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"html2pdf-auth-service/internal/tokens"
)

func TestOptionalAPIKeyAuth_PublicAccess(t *testing.T) {
	app := fiber.New()
	cache := tokens.NewCache() // not ready, but should be OK for public

	app.Use(OptionalAPIKeyAuth(cache))
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

func TestOptionalAPIKeyAuth_TokenStoreNotReady(t *testing.T) {
	app := fiber.New()
	cache := tokens.NewCache() // not ready

	app.Use(OptionalAPIKeyAuth(cache))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "abc")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestOptionalAPIKeyAuth_InvalidAndValidKey(t *testing.T) {
	app := fiber.New()
	cache := tokens.NewCache()
	cache.Replace(map[string]int{"good": 1})

	app.Use(OptionalAPIKeyAuth(cache))
	app.Get("/", func(c *fiber.Ctx) error {
		// keyauth stores validated key into Locals(ContextKey)
		if v, _ := c.Locals("api_key").(string); v == "" {
			return c.Status(fiber.StatusInternalServerError).SendString("missing api_key local")
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// invalid
	req1, _ := http.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-API-Key", "bad")
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp1.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp1.StatusCode)
	}

	// valid
	req2, _ := http.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-API-Key", "good")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}
