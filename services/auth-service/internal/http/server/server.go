package server

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"

	"html2pdf-auth-service/internal/tokens"
	"html2pdf-auth-service/internal/config"
	"html2pdf-auth-service/internal/infra/ratelimit"
	"html2pdf-auth-service/internal/http/handlers"
	"html2pdf-auth-service/internal/http/middleware"
)

type Deps struct {
	Config     config.Config
	TokenCache *tokens.Cache
	Store      fiber.Storage
}

func NewApp(deps Deps) *fiber.App {
	cfg := deps.Config

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		// Envoy terminates the downstream connection, so we must trust the proxy header
		// to get stable user-based keys for rate limiting.
		ProxyHeader: "X-Forwarded-For",
	})

	app.Use(healthcheck.New())

	// API key auth (optional). Missing key = public access.
	app.Use(middleware.OptionalAPIKeyAuth(deps.TokenCache))

	// Rate limiting happens before the allow/deny response.
	rlCfg := middleware.RateLimitConfig{
		RateInterval:           cfg.RateInterval,
		EnableUserLimiter:      cfg.EnableUserLimiter,
		UserLimit:              cfg.UserLimit,
		EnableTokenRateLimiter: cfg.EnableTokenRateLimiter,
	}

	cache := middleware.NewLimiterCache()
	app.Use(middleware.TokenRateLimit(rlCfg, deps.TokenCache, deps.Store, cache))
	app.Use(middleware.UserRateLimit(rlCfg, deps.Store))

	// Envoy ext_authz with http_service uses a path_prefix and appends the original path.
	// We accept both /ext-authz and /ext-authz/*.
	app.All("/ext-authz", handlers.ExtAuthzOK)
	app.All("/ext-authz/*", handlers.ExtAuthzOK)

	return app
}

func Run(ctx context.Context, deps Deps) error {
	_ = ctx
	app := NewApp(deps)
	log.Printf("auth-service listening on %s", deps.Config.ListenAddr)
	return app.Listen(deps.Config.ListenAddr)
}

// helper exposed for wiring convenience
func NewRateLimitStore(cfg config.Config) fiber.Storage {
	return ratelimit.NewStore(ratelimit.RedisConfig{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisRateDB,
	})
}
