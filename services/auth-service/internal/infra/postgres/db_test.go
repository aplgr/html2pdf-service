package postgres

import "testing"

func TestDBGet_ReusesAndReplacesByDSN(t *testing.T) {
	p := NewDB()

	db1, err := p.Get("postgres://user:pass@localhost:5432/db1?sslmode=disable")
	if err != nil {
		t.Fatalf("first get failed: %v", err)
	}
	db2, err := p.Get("postgres://user:pass@localhost:5432/db1?sslmode=disable")
	if err != nil {
		t.Fatalf("second get failed: %v", err)
	}
	if db1 != db2 {
		t.Fatalf("expected same *sql.DB for identical dsn")
	}

	db3, err := p.Get("postgres://user:pass@localhost:5432/db2?sslmode=disable")
	if err != nil {
		t.Fatalf("third get failed: %v", err)
	}
	if db3 == nil {
		t.Fatalf("expected non-nil db on dsn change")
	}
	if p.dsn == "" {
		t.Fatalf("expected manager dsn to be tracked")
	}
}
