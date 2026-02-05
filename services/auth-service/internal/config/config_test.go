package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoadFrom_Valid(t *testing.T) {
	p := writeConfig(t, `listen_addr: ":9000"
postgres_dsn: "postgres://x"
token_reload_interval: 1m
rate_interval: 1h
enable_user_limiter: true
user_limit: 20
enable_token_rate_limiter: true
`)
	cfg := LoadFrom(p)
	if cfg.PostgresDSN != "postgres://x" {
		t.Fatalf("unexpected postgres dsn: %q", cfg.PostgresDSN)
	}
	if cfg.UserLimit != 20 {
		t.Fatalf("unexpected user_limit: %d", cfg.UserLimit)
	}
}

func TestLoadFrom_PanicsOnInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		yml  string
	}{
		{name: "missing postgres dsn", yml: "token_reload_interval: 1m\nrate_interval: 1h\n"},
		{name: "invalid rate interval", yml: "postgres_dsn: 'x'\ntoken_reload_interval: 1m\nrate_interval: 0s\n"},
		{name: "negative user limit", yml: "postgres_dsn: 'x'\ntoken_reload_interval: 1m\nrate_interval: 1h\nuser_limit: -1\n"},
		{name: "invalid reload interval", yml: "postgres_dsn: 'x'\ntoken_reload_interval: 0s\nrate_interval: 1h\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := writeConfig(t, tc.yml)
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic")
				}
			}()
			_ = LoadFrom(p)
		})
	}
}

func TestLoad_UsesConfigPathEnv(t *testing.T) {
	p := writeConfig(t, `postgres_dsn: "postgres://env"
token_reload_interval: 2m
rate_interval: 30m
user_limit: 1
`)
	t.Setenv("CONFIG_PATH", p)
	cfg := Load()
	if cfg.PostgresDSN != "postgres://env" {
		t.Fatalf("expected CONFIG_PATH to be used")
	}
}
