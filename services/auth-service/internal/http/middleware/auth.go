package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"

	"html2pdf-auth-service/internal/domain"
)

type TokenStore interface {
	Ready() bool
	Validate(token string) bool
}

func OptionalAPIKeyAuth(tokens TokenStore) fiber.Handler {
	return keyauth.New(keyauth.Config{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			if !tokens.Ready() {
				return false, domain.ErrTokenStoreNotReady
			}
			if !tokens.Validate(key) {
				return false, domain.ErrInvalidAPIKey
			}
			return true, nil
		},
		// Missing key = public access. Also allow OPTIONS preflight.
		Next: func(c *fiber.Ctx) bool {
			return c.Method() == fiber.MethodOptions || c.Get("X-API-Key") == ""
		},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			status := fiber.StatusUnauthorized
			if err == nil {
				err = fiber.ErrUnauthorized
			}
			if errors.Is(err, domain.ErrTokenStoreNotReady) {
				status = fiber.StatusServiceUnavailable
			}
			return c.Status(status).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    status,
					"message": err.Error(),
				},
			})
		},
	})
}
