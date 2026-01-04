package main

import (
	"context"
	"log"

	"html2pdf-auth-service/internal/tokens"
	"html2pdf-auth-service/internal/config"
	"html2pdf-auth-service/internal/infra/postgres"
	"html2pdf-auth-service/internal/http/server"
)

func main() {
	cfg := config.Load()

	// Token cache + repository
	cache := tokens.NewCache()
	db := postgres.NewDB()
	repo := postgres.NewTokenRepository(db, cfg.PostgresDSN)

	// Initial token load (may fail if DB not ready yet).
	if err := tokens.NewReloader(repo, cache, cfg.TokenReloadInterval).LoadOnce(context.Background()); err != nil {
		log.Printf("initial token load failed: %v", err)
	} else {
		log.Printf("token store ready")
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
		log.Fatal(err)
	}
}
