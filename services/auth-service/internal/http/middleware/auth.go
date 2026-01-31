package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"

	"auth-service/internal/domain"
	"auth-service/internal/infra/logging"
)

type TokenStore interface {
	Ready() bool
	Validate(token string) bool
	HasScope(token, scope string) bool
}

func OptionalAPIKeyAuth(tokens TokenStore) fiber.Handler {
	return keyauth.New(keyauth.Config{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			trimmed := strings.TrimSpace(key)
			if trimmed != key {
				key = trimmed
			}
			if !tokens.Ready() {
				logging.Warn("Auth reject", "reason", "token_store_not_ready", "method", c.Method(), "path", c.Path())
				return false, domain.ErrTokenStoreNotReady
			}
			if !tokens.Validate(key) {
				logging.Warn("Auth reject", "reason", "invalid_key", "key", redactToken(key), "method", c.Method(), "path", c.Path())
				return false, domain.ErrInvalidAPIKey
			}
			if isOpsPathFromRequest(c.Path()) && !tokens.HasScope(key, "ops") {
				logging.Warn("Auth reject", "reason", "missing_ops_scope", "key", redactToken(key), "method", c.Method(), "path", c.Path())
				return false, domain.ErrInvalidAPIKey
			}
			logging.Info("Auth allow", "key", redactToken(key), "method", c.Method(), "path", c.Path())
			return true, nil
		},
		// Missing key = public access. Also allow OPTIONS preflight.
		Next: func(c *fiber.Ctx) bool {
			if isOpsPathFromRequest(c.Path()) {
				return false
			}
			if c.Method() == fiber.MethodOptions || c.Get("X-API-Key") == "" {
				logging.Info("Auth allow", "reason", "public", "method", c.Method(), "path", c.Path())
				return true
			}
			return false
		},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			status := fiber.StatusUnauthorized
			if err == nil {
				err = fiber.ErrUnauthorized
			}
			if errors.Is(err, domain.ErrTokenStoreNotReady) {
				status = fiber.StatusServiceUnavailable
			}
			logging.Warn("Auth error", "status", status, "message", err.Error(), "method", c.Method(), "path", c.Path())
			return c.Status(status).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    status,
					"message": err.Error(),
				},
			})
		},
	})
}

func redactToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func isOpsPathFromRequest(path string) bool {
	if strings.HasPrefix(path, "/ext-authz") {
		path = strings.TrimPrefix(path, "/ext-authz")
		if path == "" {
			path = "/"
		}
	}
	return path == "/ops" || strings.HasPrefix(path, "/ops/")
}
