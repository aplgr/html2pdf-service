package utils

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var tokens struct {
	sync.RWMutex
	cache map[string]int
}

var tokenDB struct {
	sync.Mutex
	dsn string
	db  *sql.DB
}

var (
	// ErrInvalidAPIKey signals that the provided API key is not known.
	ErrInvalidAPIKey = errors.New("invalid api key")
	// ErrTokenStoreNotReady signals that the token store has not been loaded yet.
	// This can happen during startup when the DB isn't ready.
	ErrTokenStoreNotReady = errors.New("token store not ready")
)

func postgresPort(cfg PostgresConfig) int {
	if cfg.Port != 0 {
		return cfg.Port
	}
	return 5432
}

func postgresDSN(cfg PostgresConfig) (string, error) {
	if strings.HasPrefix(cfg.Host, "postgres://") || strings.HasPrefix(cfg.Host, "postgresql://") {
		return cfg.Host, nil
	}
	if cfg.Host == "" {
		return "", fmt.Errorf("postgres host is empty")
	}
	if cfg.Database == "" {
		return "", fmt.Errorf("postgres database is empty")
	}
	if cfg.User == "" {
		return "", fmt.Errorf("postgres user is empty")
	}

	hostPort := cfg.Host
	port := postgresPort(cfg)
	// Handle IPv6 or explicit host:port strings.
	if strings.HasPrefix(hostPort, "[") {
		if !strings.Contains(hostPort, "]:") {
			hostPort = fmt.Sprintf("%s:%d", hostPort, port)
	}
	} else if strings.Count(hostPort, ":") >= 2 {
		hostPort = fmt.Sprintf("[%s]:%d", hostPort, port)
	} else if !strings.Contains(hostPort, ":") {
		hostPort = fmt.Sprintf("%s:%d", hostPort, port)
	}
	
	u := &url.URL{Scheme: "postgres", Host: hostPort, Path: "/" + cfg.Database}
	if cfg.Password != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	} else {
		u.User = url.User(cfg.User)
	}
	q := u.Query()
	if cfg.SSLMode != "" {
		q.Set("sslmode", cfg.SSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func getTokenDB(cfg PostgresConfig) (*sql.DB, error) {
	dsn, err := postgresDSN(cfg)
	if err != nil {
		return nil, err
	}

	tokenDB.Lock()
	defer tokenDB.Unlock()

	if tokenDB.db != nil && tokenDB.dsn == dsn {
		return tokenDB.db, nil
	}
	if tokenDB.db != nil {
		_ = tokenDB.db.Close()
		tokenDB.db = nil
		tokenDB.dsn = ""
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// This is a small, low-throughput control plane table.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	tokenDB.db = db
	tokenDB.dsn = dsn
	return tokenDB.db, nil
}

func ensureTokensSchemaPostgres(cfg PostgresConfig) error {
	db, err := getTokenDB(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ddl1 := `CREATE TABLE IF NOT EXISTS tokens (
		token TEXT PRIMARY KEY,
		rate_limit INTEGER NOT NULL DEFAULT 60,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		comment TEXT
	);`
	ddl2 := `CREATE INDEX IF NOT EXISTS idx_tokens_created_at ON tokens (created_at);`
	if _, err := db.ExecContext(ctx, ddl1); err != nil {
	return err
	}
	if _, err := db.ExecContext(ctx, ddl2); err != nil {
		return err
	}
	return nil
}

// LoadTokensFromPostgres reads all API tokens and their rate limits from
// Postgres and stores them in an in-memory cache.
func LoadTokensFromPostgres(cfg PostgresConfig) error {
	if err := ensureTokensSchemaPostgres(cfg); err != nil {
		return err
	}

	db, err := getTokenDB(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `SELECT token, rate_limit FROM tokens;`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cache := make(map[string]int)
	for rows.Next() {
		var token string
		var limit int
		if err := rows.Scan(&token, &limit); err != nil {
			return err
			}
			cache[token] = limit
		}
	if err := rows.Err(); err != nil {
		return err
	}

	tokens.Lock()
	tokens.cache = cache
	tokens.Unlock()
	return nil
}

// LoadTokensFromMap is a small helper intended for tests and local debugging.
// It replaces the current in-memory token cache with the provided map.
func LoadTokensFromMap(m map[string]int) {
	cache := make(map[string]int)
	for k, v := range m {
		cache[k] = v
	}
	tokens.Lock()
	tokens.cache = cache
	tokens.Unlock()
}

// TokensReady returns true if the token cache has been initialized at least once.
func TokensReady() bool {
	tokens.RLock()
	defer tokens.RUnlock()
	return tokens.cache != nil
}

// ValidateToken checks whether the given token exists in the cached list.
func ValidateToken(token string) bool {
	tokens.RLock()
	defer tokens.RUnlock()
	_, ok := tokens.cache[token]
	return ok
}

// GetRateLimit returns the configured rate limit for the given token. If the
// token is unknown, 0 is returned which effectively disables rate limiting for
// that token.
func GetRateLimit(token string) int {
	tokens.RLock()
	defer tokens.RUnlock()
	if limit, ok := tokens.cache[token]; ok {
		return limit
	}
	return 0
}

// RefreshTokensPeriodicallyFromPostgres reloads the token list from Postgres at
// the specified interval. It stops once the provided stop channel is closed.
func RefreshTokensPeriodicallyFromPostgres(cfg PostgresConfig, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := LoadTokensFromPostgres(cfg); err != nil {
				Error("Failed to reload API tokens", "error", err)
			}
		case <-stop:
			return
		}
	}
}
