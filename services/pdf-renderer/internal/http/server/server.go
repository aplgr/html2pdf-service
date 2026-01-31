package server

import (
	"pdf-renderer/internal/config"
	"pdf-renderer/internal/http/handlers"
	"pdf-renderer/internal/http/middleware"
	"pdf-renderer/internal/infra/logging"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Config config.Config
	Redis  *redis.Client
}

// New creates and configures a new Fiber app instance.
func New(deps Deps) *fiber.App {
	cfg := deps.Config

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

			logging.Warn("Request failed", "path", c.Path(), "status", code, "message", msg)

			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    code,
					"message": msg,
				},
			})
		},
	})

	middleware.Register(app, cfg)
	registerRoutes(app, cfg, deps.Redis)

	// Ensure all responses, including 404s, return JSON.
	app.Use(func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "Not Found")
	})

	return app
}

func registerRoutes(app *fiber.App, cfg config.Config, redis *redis.Client) {
	v0 := app.Group("/v0")

	// Create one shared service instance so /v0/pdf (GET+POST) share the same Chrome pool.
	svc := handlers.NewPDFService(cfg, redis)

	v0.Post("/pdf", svc.HandleConversion)
	v0.Get("/pdf", svc.HandleURLConversion)
	v0.Get("/chrome/stats", svc.HandleChromeStats)

	v0.Get("/monitor", monitor.New())
}
