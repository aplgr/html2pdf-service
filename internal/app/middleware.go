package app

import (
	"crypto/sha256"
	"encoding/hex"
	u "html2pdf/internal/utils"

	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	memoryStorage "github.com/gofiber/storage/memory/v2"
	redisStorage "github.com/gofiber/storage/redis/v2"
	"github.com/rs/xid"
)

var (
	tokenLimiterCache struct {
		sync.RWMutex
		handlers map[int]fiber.Handler
	}
	rateLimitStore fiber.Storage
)

// getTokenLimiter returns a cached limiter for the given token limit, creating one if needed.
func getTokenLimiter(limit int) fiber.Handler {
	tokenLimiterCache.RLock()
	h, ok := tokenLimiterCache.handlers[limit]
	tokenLimiterCache.RUnlock()
	if ok {
		return h
	}

	appCfg := u.GetConfig()
	cfg := limiter.Config{
		Max:               limit,
		Expiration:        appCfg.RateLimiter.Interval,
		LimiterMiddleware: limiter.SlidingWindow{},
		Storage:           rateLimitStore,
		KeyGenerator: func(c *fiber.Ctx) string {
			if token, ok := c.Locals("api_key").(string); ok {
				return token
			}
			return ""
		},
		LimitReached: func(c *fiber.Ctx) error {
			token, _ := c.Locals("api_key").(string)
			u.Warn("Rate limit exceeded", "token", token, "path", c.Path())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    fiber.StatusTooManyRequests,
					"message": "Too Many Requests",
				},
			})
		},
	}

	h = limiter.New(cfg)

	tokenLimiterCache.Lock()
	if tokenLimiterCache.handlers == nil {
		tokenLimiterCache.handlers = make(map[int]fiber.Handler)
	}
	tokenLimiterCache.handlers[limit] = h
	tokenLimiterCache.Unlock()

	return h
}

// rateLimitMiddleware applies per-token rate limits using Redis-backed limiter.
func rateLimitMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token, ok := c.Locals("api_key").(string)
		if !ok || token == "" {
			return c.Next()
		}
		limit := u.GetRateLimit(token)
		if limit == 0 {
			return c.Next()
		}
		return getTokenLimiter(limit)(c)
	}
}

// userRateLimitMiddleware limits requests based on client information when enabled.
func userRateLimitMiddleware(cfg u.Config) fiber.Handler {
	if cfg.RateLimiter.UserLimit <= 0 {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}
	hcfg := limiter.Config{
		Max:               cfg.RateLimiter.UserLimit,
		Expiration:        cfg.RateLimiter.Interval,
		LimiterMiddleware: limiter.SlidingWindow{},
		Storage:           rateLimitStore,
		KeyGenerator: func(c *fiber.Ctx) string {
			sum := sha256.Sum256([]byte(c.IP() + c.Get("User-Agent")))
			return hex.EncodeToString(sum[:])
		},
		LimitReached: func(c *fiber.Ctx) error {
			sum := sha256.Sum256([]byte(c.IP() + c.Get("User-Agent")))
			key := hex.EncodeToString(sum[:])
			u.Warn("Rate limit exceeded", "user", key, "path", c.Path())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    fiber.StatusTooManyRequests,
					"message": "Too Many Requests",
				},
			})
		},
	}
	userLimiter := limiter.New(hcfg)
	return func(c *fiber.Ctx) error {
		// If a request is authenticated via X-API-Key, we intentionally skip the
		// user-based limiter. Token-based limits are applied earlier.
		if token, ok := c.Locals("api_key").(string); ok && token != "" {
			return c.Next()
		}
		return userLimiter(c)
	}
}

// RegisterMiddleware attaches global middleware to the app
func RegisterMiddleware(app *fiber.App, cfg u.Config) {
	rateLimitStore = memoryStorage.New() // safe default

	func() {
		defer func() {
			if r := recover(); r != nil {
				u.Error("Redis limiter store init panicked, falling back to memory", "panic", r)
			}
		}()
		rateLimitStore = redisStorage.New(redisStorage.Config{
			Addrs:    []string{cfg.Cache.RedisHost},
			Database: cfg.Cache.RateLimitDB,
		})
		u.Info("Using Redis for rate limiting", "addr", cfg.Cache.RedisHost, "db", cfg.Cache.RateLimitDB)
	}()

	app.Use(cors.New())

	app.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return xid.New().String()
		},
	}))

	app.Use(healthcheck.New())

	app.Use(keyauth.New(keyauth.Config{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			// Avoid nil-pointer panics in ErrorHandler (fiber keyauth may pass nil err).
			// Also provide a clear signal when the token store is not loaded yet.
			if !u.TokensReady() {
				return false, u.ErrTokenStoreNotReady
			}
			if !u.ValidateToken(key) {
				return false, u.ErrInvalidAPIKey
			}
			return true, nil
		},
		Next: func(c *fiber.Ctx) bool {
			return c.Method() == fiber.MethodOptions || c.Get("X-API-Key") == ""
		},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Keyauth can call ErrorHandler with a nil error.
			status := fiber.StatusUnauthorized
			if err == nil {
				err = fiber.ErrUnauthorized
			}
			if err == u.ErrTokenStoreNotReady {
				status = fiber.StatusServiceUnavailable
			}
			return c.Status(status).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    status,
					"message": err.Error(),
				},
			})
		},
	}))

	app.Use(rateLimitMiddleware())

	if cfg.RateLimiter.EnableUserLimiter || cfg.RateLimiter.UserLimit > 0 {
		app.Use(userRateLimitMiddleware(cfg))
	}

	app.Use(func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = c.GetRespHeader("X-Request-ID")
		}
		u.Info("Incoming request", "method", c.Method(), "path", c.Path(), "request_id", requestID)
		return c.Next()
	})
}
