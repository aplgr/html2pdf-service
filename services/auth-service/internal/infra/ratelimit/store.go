package ratelimit

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	memoryStorage "github.com/gofiber/storage/memory/v2"
	redisStorage "github.com/gofiber/storage/redis/v2"
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
		log.Printf("redis addr empty, using memory for rate limiting")
		return store
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("redis limiter store init panicked, falling back to memory: %v", r)
			}
		}()
		store = redisStorage.New(redisStorage.Config{
			Addrs:    []string{cfg.Addr},
			Password: cfg.Password,
			Database: cfg.DB,
		})
		log.Printf("using redis for rate limiting addr=%s db=%d", cfg.Addr, cfg.DB)
	}()

	return store
}
