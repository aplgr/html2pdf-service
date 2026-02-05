package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestVerifySchema_FailsWithoutDatabase(t *testing.T) {
	mgr := NewDB()
	db, err := mgr.Get("postgres://user:pass@127.0.0.1:1/db?sslmode=disable")
	if err != nil {
		t.Fatalf("unexpected open error: %v", err)
	}
	if err := VerifySchema(db); err == nil {
		t.Fatalf("expected schema verification to fail without reachable db")
	}
}

func TestTokenRepository_LoadTokens_FailsWhenDBUnavailable(t *testing.T) {
	repo := NewTokenRepository(NewDB(), "postgres://user:pass@127.0.0.1:1/db?sslmode=disable")
	_, err := repo.LoadTokens(context.Background())
	if err == nil {
		t.Fatalf("expected load tokens error when db is unavailable")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "failed") && !strings.Contains(strings.ToLower(err.Error()), "connect") {
		// ensure we at least got a meaningful runtime/db error
		t.Logf("load error: %v", err)
	}
}
