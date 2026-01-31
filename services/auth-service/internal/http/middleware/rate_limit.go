package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"auth-service/internal/infra/logging"
)

type TokenRater interface {
	RateLimit(token string) int
}

type RateLimitConfig struct {
	RateInterval           time.Duration
	EnableUserLimiter      bool
	UserLimit              int
	EnableTokenRateLimiter bool
}

type LimiterCache struct {
	mu       sync.RWMutex
	handlers map[int]fiber.Handler
}

func NewLimiterCache() *LimiterCache {
	return &LimiterCache{handlers: make(map[int]fiber.Handler)}
}

func (lc *LimiterCache) GetOrCreate(max int, interval time.Duration, store fiber.Storage) fiber.Handler {
	lc.mu.RLock()
	h, ok := lc.handlers[max]
	lc.mu.RUnlock()
	if ok {
		return h
	}

	cfg := limiter.Config{
		Max:               max,
		Expiration:        interval,
		LimiterMiddleware: limiter.SlidingWindow{},
		Storage:           store,
		KeyGenerator: func(c *fiber.Ctx) string {
			if token, ok := c.Locals("api_key").(string); ok {
				return token
			}
			return ""
		},
		LimitReached: func(c *fiber.Ctx) error {
			token, _ := c.Locals("api_key").(string)
			logging.Warn("Rate limit exceeded", "token", token, "path", c.Path())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    fiber.StatusTooManyRequests,
					"message": "Too many requests",
				},
			})
		},
	}

	h = limiter.New(cfg)

	lc.mu.Lock()
	lc.handlers[max] = h
	lc.mu.Unlock()

	return h
}

func TokenRateLimit(cfg RateLimitConfig, tokens TokenRater, store fiber.Storage, cache *LimiterCache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !cfg.EnableTokenRateLimiter {
			return c.Next()
		}
		token, ok := c.Locals("api_key").(string)
		if !ok || token == "" {
			return c.Next()
		}
		limit := tokens.RateLimit(token)
		if limit == 0 {
			return c.Next()
		}
		return cache.GetOrCreate(limit, cfg.RateInterval, store)(c)
	}
}

func UserRateLimit(cfg RateLimitConfig, store fiber.Storage) fiber.Handler {
	if !cfg.EnableUserLimiter || cfg.UserLimit <= 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	hcfg := limiter.Config{
		Max:               cfg.UserLimit,
		Expiration:        cfg.RateInterval,
		LimiterMiddleware: limiter.SlidingWindow{},
		Storage:           store,
		KeyGenerator: func(c *fiber.Ctx) string {
			sum := sha256.Sum256([]byte(c.IP() + c.Get("User-Agent")))
			return hex.EncodeToString(sum[:])
		},
		LimitReached: func(c *fiber.Ctx) error {
			sum := sha256.Sum256([]byte(c.IP() + c.Get("User-Agent")))
			key := hex.EncodeToString(sum[:])
			logging.Warn("Rate limit exceeded", "user", key, "path", c.Path())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    fiber.StatusTooManyRequests,
					"message": "Too many requests",
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
