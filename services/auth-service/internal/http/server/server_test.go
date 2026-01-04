package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	memoryStorage "github.com/gofiber/storage/memory/v2"

	"html2pdf-auth-service/internal/config"
	"html2pdf-auth-service/internal/tokens"
)

func TestNewApp_ExtAuthzModesAndAuth(t *testing.T) {
	cache := tokens.NewCache()
	cache.Replace(map[string]int{"good": 1})

	deps := Deps{
		Config: config.Config{
			ListenAddr:            ":0",
			RateInterval:          time.Hour,
			EnableUserLimiter:     false,
			UserLimit:             0,
			EnableTokenRateLimiter: true,
		},
		TokenCache: cache,
		Store:      memoryStorage.New(),
	}

	app := NewApp(deps)

	// public
	req1, _ := http.NewRequest(http.MethodGet, "/ext-authz/test", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	if got := resp1.Header.Get("X-Auth-Mode"); got != "public" {
		t.Fatalf("expected X-Auth-Mode=public, got %q", got)
	}

	// invalid key
	req2, _ := http.NewRequest(http.MethodGet, "/ext-authz/test", nil)
	req2.Header.Set("X-API-Key", "bad")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp2.StatusCode)
	}

	// valid key (first request OK, second should hit token rate limit = 1)
	req3, _ := http.NewRequest(http.MethodGet, "/ext-authz/test", nil)
	req3.Header.Set("X-API-Key", "good")
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp3.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	if got := resp3.Header.Get("X-Auth-Mode"); got != "token" {
		t.Fatalf("expected X-Auth-Mode=token, got %q", got)
	}

	resp4, err := app.Test(req3)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp4.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp4.StatusCode)
	}
}
