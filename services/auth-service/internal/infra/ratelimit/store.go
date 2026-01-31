package ratelimit

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	memoryStorage "github.com/gofiber/storage/memory/v2"
	redisStorage "github.com/gofiber/storage/redis/v2"

	"auth-service/internal/infra/logging"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewStore creates a Fiber storage backend used by the Fiber limiter middleware.
// It falls back to in-memory storage if Redis init fails.
func NewStore(cfg RedisConfig) fiber.Storage {
	var store fiber.Storage = memoryStorage.New() // safe default

	if strings.TrimSpace(cfg.Addr) == "" {
		logging.Warn("Redis addr empty, using memory for rate limiting")
		return store
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				logging.Error("Redis limiter store init panicked, falling back to memory", "error", r)
			}
		}()
		store = redisStorage.New(redisStorage.Config{
			Addrs:    []string{cfg.Addr},
			Password: cfg.Password,
			Database: cfg.DB,
		})
		logging.Info("Using redis for rate limiting", "addr", cfg.Addr, "db", cfg.DB)
	}()

	return store
}
