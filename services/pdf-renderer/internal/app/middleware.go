package app

import (
	u "html2pdf/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/xid"
)

// RegisterMiddleware attaches global middleware to the app.
//
// Auth and rate limiting are intentionally NOT handled here anymore.
// They are enforced at the gateway (Envoy) via an external auth service.
func RegisterMiddleware(app *fiber.App, cfg u.Config) {
	_ = cfg // kept for forward-compat; middleware might use config later.

	app.Use(cors.New())

	app.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return xid.New().String()
		},
	}))

	app.Use(healthcheck.New())

	app.Use(func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = c.GetRespHeader("X-Request-ID")
		}
		u.Info("Incoming request", "method", c.Method(), "path", c.Path(), "request_id", requestID)
		return c.Next()
	})
}
