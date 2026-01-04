package postgres

import (
	"context"
	"database/sql"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	mu  sync.Mutex
	dsn string
	db  *sql.DB
}

func NewDB() *DB {
	return &DB{}
}

func (p *DB) Get(dsn string) (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.db != nil && p.dsn == dsn {
		return p.db, nil
	}
	if p.db != nil {
		_ = p.db.Close()
		p.db = nil
		p.dsn = ""
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	p.db = db
	p.dsn = dsn
	return db, nil
}

func EnsureSchema(db *sql.DB) error {
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
