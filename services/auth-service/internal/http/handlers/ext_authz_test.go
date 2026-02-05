package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestExtAuthzOK_PublicAndTokenModes(t *testing.T) {
	app := fiber.New()
	app.Get("/public", ExtAuthzOK)
	app.Get("/token", func(c *fiber.Ctx) error {
		c.Locals("api_key", "abc")
		return ExtAuthzOK(c)
	})

	req1, _ := http.NewRequest(http.MethodGet, "/public", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("public request failed: %v", err)
	}
	if got := resp1.Header.Get("X-Auth-Mode"); got != "public" {
		t.Fatalf("expected public mode, got %q", got)
	}

	req2, _ := http.NewRequest(http.MethodGet, "/token", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	if got := resp2.Header.Get("X-Auth-Mode"); got != "token" {
		t.Fatalf("expected token mode, got %q", got)
	}
}
