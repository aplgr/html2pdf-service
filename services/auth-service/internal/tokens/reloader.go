package tokens

import (
	"context"
	"time"

	"auth-service/internal/infra/logging"
)

type Repository interface {
	LoadTokens(ctx context.Context) (map[string]Entry, error)
}

type Reloader struct {
	repo     Repository
	cache    *Cache
	interval time.Duration
}

func NewReloader(repo Repository, cache *Cache, interval time.Duration) *Reloader {
	return &Reloader{repo: repo, cache: cache, interval: interval}
}

func (r *Reloader) LoadOnce(ctx context.Context) error {
	m, err := r.repo.LoadTokens(ctx)
	if err != nil {
		return err
	}
	r.cache.Replace(m)
	return nil
}

func (r *Reloader) Start(ctx context.Context) {
	t := time.NewTicker(r.interval)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := r.LoadOnce(ctx); err != nil {
					logging.Error("Token reload failed", "error", err)
				}
			}
		}
	}()
}
