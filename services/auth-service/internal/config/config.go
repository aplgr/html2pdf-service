package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr string

	PostgresDSN string

	RedisAddr     string
	RedisPassword string
	RedisRateDB   int

	TokenReloadInterval time.Duration

	RateInterval           time.Duration
	EnableUserLimiter      bool
	UserLimit              int
	EnableTokenRateLimiter bool
}

func Load() Config {
	cfg := Config{
		ListenAddr: env("AUTH_LISTEN", ":9000"),

		PostgresDSN: mustEnv("AUTH_PG_DSN"),

		RedisAddr:     env("AUTH_REDIS_ADDR", "redis:6379"),
		RedisPassword: env("AUTH_REDIS_PASSWORD", ""),
		RedisRateDB:   envInt("AUTH_REDIS_RATE_DB", 0),

		TokenReloadInterval: envDuration("AUTH_TOKEN_RELOAD", 10*time.Second),

		RateInterval:           envDuration("AUTH_RL_INTERVAL", time.Hour),
		EnableUserLimiter:      envBool("AUTH_ENABLE_USER_LIMITER", true),
		UserLimit:              envInt("AUTH_USER_LIMIT", 20),
		EnableTokenRateLimiter: envBool("AUTH_ENABLE_TOKEN_LIMITER", true),
	}

	if cfg.RateInterval <= 0 {
		log.Fatal("AUTH_RL_INTERVAL must be > 0")
	}
	if cfg.UserLimit < 0 {
		log.Fatal("AUTH_USER_LIMIT must be >= 0")
	}
	if cfg.TokenReloadInterval <= 0 {
		log.Fatal("AUTH_TOKEN_RELOAD must be > 0")
	}

	return cfg
}

func env(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func mustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("missing env: %s", key)
	}
	return v
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("invalid int env %s=%q: %v", key, v, err)
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Fatalf("invalid bool env %s=%q: %v", key, v, err)
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("invalid duration env %s=%q: %v", key, v, err)
	}
	return d
}
