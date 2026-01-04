package postgres

import (
	"context"
	"database/sql"
	"time"
)

type TokenRepository struct {
	DB  *DB
	DSN string
}

func NewTokenRepository(db *DB, dsn string) *TokenRepository {
	return &TokenRepository{DB: db, DSN: dsn}
}

func (r *TokenRepository) LoadTokens(ctx context.Context) (map[string]int, error) {
	db, err := r.DB.Get(r.DSN)
	if err != nil {
		return nil, err
	}
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(cctx, `SELECT token, rate_limit FROM tokens;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var token string
		var limit int
		if err := rows.Scan(&token, &limit); err != nil {
			return nil, err
		}
		out[token] = limit
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// compile-time check
var _ interface {
	LoadTokens(ctx context.Context) (map[string]int, error)
} = (*TokenRepository)(nil)

var _ *sql.DB
