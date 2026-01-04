package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"html2pdf/internal/app"
	u "html2pdf/internal/utils"
)

var RedisClient *redis.Client

func main() {
	cfg := u.LoadConfig()

	// Allow common container env var to override chrome_path.
	if cfg.PDF.ChromePath == "" {
		if v := os.Getenv("CHROME_BIN"); v != "" {
			cfg.PDF.ChromePath = v
		}
	}

	u.InitLogger(
		cfg.Logger.File,
		cfg.Logger.MaxSizeMB,
		cfg.Logger.MaxBackups,
		cfg.Logger.MaxAgeDays,
		cfg.Logger.Compress,
		cfg.Logger.Level,
	)
	u.SetLogLevel(cfg.Logger.Level)

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Cache.RedisHost,
		DB:   cfg.Cache.PDFCacheDB,
	})
	RedisClient = rdb // optional, kept for potential global usage

	app := app.SetupApp(cfg, rdb)

	idleConnsClosed := make(chan struct{})
	startServer(app, cfg, idleConnsClosed)
	<-idleConnsClosed
}

// startServer starts the Fiber app and listens for shutdown signals.
func startServer(app *fiber.App, cfg u.Config, idleConnsClosed chan struct{}) {
	go func() {
		if err := app.Listen(cfg.Server.Host + cfg.Server.Port); err != nil {
			u.Error("Server error", "error", err)
		}
	}()

	// Listen for OS termination signals.
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
	<-sigint

	u.Warn("Shutdown signal received, closing server...")

	// Graceful shutdown with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		u.Error("Server forced to shutdown", "error", err)
	}

	close(idleConnsClosed)
	u.Info("Server stopped cleanly")
}
