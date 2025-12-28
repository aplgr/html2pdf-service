package app

import (
	"html2pdf/internal/handlers"
	u "html2pdf/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/redis/go-redis/v9"
)

// SetupApp creates and configures a new Fiber app instance
func SetupApp(cfg u.Config, redis *redis.Client) *fiber.App {
	app := fiber.New(fiber.Config{
		Prefork:               cfg.Server.Prefork,
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			msg := "Internal Server Error"

			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				msg = e.Message
			}

			u.Warn("Request failed", "path", c.Path(), "status", code, "message", msg)

			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    code,
					"message": msg,
				},
			})
		},
	})

	RegisterMiddleware(app, cfg)
	RegisterRoutes(app, cfg, redis)

	// Ensure all responses, including 404s, return JSON
	app.Use(func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "Not Found")
	})

	return app
}

// RegisterRoutes mounts all route handlers to the app
func RegisterRoutes(app *fiber.App, cfg u.Config, redis *redis.Client) {
	v1 := app.Group("/v1")

	// Create one shared service instance so /v1/pdf (GET+POST) share the same Chrome pool.
	svc := handlers.NewPDFService(cfg, redis)

	v1.Post("/pdf", svc.HandleConversion)
	v1.Get("/pdf", svc.HandleURLConversion)
	v1.Get("/chrome/stats", svc.HandleChromeStats)

	v1.Get("/monitor", monitor.New())
}

