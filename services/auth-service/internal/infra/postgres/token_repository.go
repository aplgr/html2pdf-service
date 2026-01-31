package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"auth-service/internal/tokens"
)

type TokenRepository struct {
	DB  *DB
	DSN string
}

func NewTokenRepository(db *DB, dsn string) *TokenRepository {
	return &TokenRepository{DB: db, DSN: dsn}
}

func (r *TokenRepository) LoadTokens(ctx context.Context) (map[string]tokens.Entry, error) {
	db, err := r.DB.Get(r.DSN)
	if err != nil {
		return nil, err
	}
	if err := VerifySchema(db); err != nil {
		return nil, err
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(cctx, `SELECT token, rate_limit, scope FROM fn_fetch_auth_tokens();`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]tokens.Entry)
	for rows.Next() {
		var token string
		var limit int
		var scopeRaw []byte
		if err := rows.Scan(&token, &limit, &scopeRaw); err != nil {
			return nil, err
		}
		scope := tokens.Scope{}
		if len(scopeRaw) > 0 {
			if err := json.Unmarshal(scopeRaw, &scope); err != nil {
				return nil, err
			}
		}
		out[token] = tokens.Entry{
			RateLimit: limit,
			Scope:     scope,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// compile-time check
var _ interface {
	LoadTokens(ctx context.Context) (map[string]tokens.Entry, error)
} = (*TokenRepository)(nil)

var _ *sql.DB
