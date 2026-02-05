package middleware

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"pdf-renderer/internal/config"
)

func TestRegister_AddsHealthAndRequestID(t *testing.T) {
	app := fiber.New()
	Register(app, config.Config{})
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	healthReq, _ := http.NewRequest(http.MethodGet, "/ops/health", nil)
	healthResp, err := app.Test(healthReq)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if healthResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected health endpoint 200, got %d", healthResp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("ping request failed: %v", err)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Fatalf("expected X-Request-Id to be present")
	}
}
