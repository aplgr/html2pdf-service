package middleware

import (
	"pdf-renderer/internal/config"
	"pdf-renderer/internal/infra/logging"

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
func Register(app *fiber.App, cfg config.Config) {
	_ = cfg // kept for forward-compat; middleware might use config later.

	app.Use(cors.New())

	app.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return xid.New().String()
		},
	}))

	app.Use(healthcheck.New(healthcheck.Config{
		LivenessEndpoint:  "/ops/health",
		ReadinessEndpoint: "/ops/health",
	}))

	app.Use(func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = c.GetRespHeader("X-Request-ID")
		}
		logging.Info("Incoming request", "method", c.Method(), "path", c.Path(), "request_id", requestID)
		return c.Next()
	})
}
