package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"auth-service/internal/config"
	"auth-service/internal/http/server"
	"auth-service/internal/infra/logging"
	"auth-service/internal/infra/postgres"
	"auth-service/internal/tokens"
)

func main() {
	cfg := config.Load()

	if err := ensureLogDir(cfg.Logger.File); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory for %s: %v\n", cfg.Logger.File, err)
	}

	logging.InitLogger(
		cfg.Logger.File,
		cfg.Logger.MaxSizeMB,
		cfg.Logger.MaxBackups,
		cfg.Logger.MaxAgeDays,
		cfg.Logger.Compress,
		cfg.Logger.Level,
	)
	logging.SetLogLevel(cfg.Logger.Level)

	// Token cache + repository
	cache := tokens.NewCache()
	db := postgres.NewDB()
	repo := postgres.NewTokenRepository(db, cfg.PostgresDSN)

	// Initial token load (may fail if DB not ready yet).
	if err := tokens.NewReloader(repo, cache, cfg.TokenReloadInterval).LoadOnce(context.Background()); err != nil {
		logging.Error("Initial token load failed", "error", err)
	} else {
		logging.Info("Token store ready")
	}

	// Periodic reload.
	reloader := tokens.NewReloader(repo, cache, cfg.TokenReloadInterval)
	reloader.Start(context.Background())

	store := server.NewRateLimitStore(cfg)

	if err := server.Run(context.Background(), server.Deps{
		Config:     cfg,
		TokenCache: cache,
		Store:      store,
	}); err != nil {
		logging.Error("Server stopped with error", "error", err)
		os.Exit(1)
	}
}

func ensureLogDir(logPath string) error {
	if logPath == "" {
		return nil
	}
	logDir := filepath.Dir(logPath)
	if logDir == "." || logDir == "/" {
		return nil
	}
	return os.MkdirAll(logDir, 0o755)
}
