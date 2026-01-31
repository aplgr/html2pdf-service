package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	memoryStorage "github.com/gofiber/storage/memory/v2"

	"auth-service/internal/config"
	"auth-service/internal/tokens"
)

func TestNewApp_ExtAuthzModesAndAuth(t *testing.T) {
	cache := tokens.NewCache()
	cache.Replace(map[string]int{"good-get": 1, "good-post": 1})

	deps := Deps{
		Config: config.Config{
			ListenAddr:             ":0",
			RateInterval:           time.Hour,
			EnableUserLimiter:      false,
			UserLimit:              0,
			EnableTokenRateLimiter: true,
		},
		TokenCache: cache,
		Store:      memoryStorage.New(),
	}

	app := NewApp(deps)

	assertPublic := func(method string) {
		t.Helper()
		req, _ := http.NewRequest(method, "/ext-authz/test?foo=bar", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if got := resp.Header.Get("X-Auth-Mode"); got != "public" {
			t.Fatalf("expected X-Auth-Mode=public, got %q", got)
		}
	}

	assertInvalid := func(method string) {
		t.Helper()
		req, _ := http.NewRequest(method, "/ext-authz/test?foo=bar", nil)
		req.Header.Set("X-API-Key", "bad")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	}

	assertValidRateLimited := func(method, token string) {
		t.Helper()
		req, _ := http.NewRequest(method, "/ext-authz/test?foo=bar", nil)
		req.Header.Set("X-API-Key", token)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if got := resp.Header.Get("X-Auth-Mode"); got != "token" {
			t.Fatalf("expected X-Auth-Mode=token, got %q", got)
		}

		resp2, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp2.StatusCode != fiber.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", resp2.StatusCode)
		}
	}

	assertPublic(http.MethodGet)
	assertPublic(http.MethodPost)
	assertInvalid(http.MethodGet)
	assertInvalid(http.MethodPost)
	assertValidRateLimited(http.MethodGet, "good-get")
	assertValidRateLimited(http.MethodPost, "good-post")
}
