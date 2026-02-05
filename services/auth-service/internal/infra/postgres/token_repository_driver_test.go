package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
)

type drvMode struct {
	schemaErr bool
	queryErr  bool
	badJSON   bool
}

var (
	testDriverCounter atomic.Int64
	testMode          drvMode
)

type fakeDriver struct{}

type fakeConn struct{}

type fakeRows struct {
	cols []string
	data [][]driver.Value
	i    int
}

func (d fakeDriver) Open(name string) (driver.Conn, error) { return fakeConn{}, nil }
func (c fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c fakeConn) Close() error              { return nil }
func (c fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

func (c fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if testMode.schemaErr {
		return nil, errors.New("schema failed")
	}
	return driver.RowsAffected(1), nil
}

func (c fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if testMode.queryErr {
		return nil, errors.New("query failed")
	}
	row1Scope := []byte(`{"api":true}`)
	if testMode.badJSON {
		row1Scope = []byte(`{bad`)
	}
	return &fakeRows{
		cols: []string{"token", "rate_limit", "scope"},
		data: [][]driver.Value{{"tok1", int64(5), row1Scope}, {"tok2", int64(2), []byte(`{"ops":true}`)}},
	}, nil
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.i])
	r.i++
	return nil
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("fakedrv_%d", testDriverCounter.Add(1))
	sql.Register(name, fakeDriver{})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	return db
}

func TestTokenRepository_LoadTokens_DriverSuccess(t *testing.T) {
	testMode = drvMode{}
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	r := &TokenRepository{DB: &DB{db: db, dsn: "x"}, DSN: "x"}
	out, err := r.LoadTokens(context.Background())
	if err != nil {
		t.Fatalf("load tokens: %v", err)
	}
	if out["tok1"].RateLimit != 5 || !out["tok1"].Scope["api"] {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestTokenRepository_LoadTokens_SchemaError(t *testing.T) {
	testMode = drvMode{schemaErr: true}
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	r := &TokenRepository{DB: &DB{db: db, dsn: "x"}, DSN: "x"}
	if _, err := r.LoadTokens(context.Background()); err == nil {
		t.Fatalf("expected schema error")
	}
}

func TestTokenRepository_LoadTokens_QueryAndJSONErrors(t *testing.T) {
	testMode = drvMode{queryErr: true}
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	r := &TokenRepository{DB: &DB{db: db, dsn: "x"}, DSN: "x"}
	if _, err := r.LoadTokens(context.Background()); err == nil {
		t.Fatalf("expected query error")
	}

	testMode = drvMode{badJSON: true}
	db2 := openTestDB(t)
	defer func() { _ = db2.Close() }()
	r2 := &TokenRepository{DB: &DB{db: db2, dsn: "x"}, DSN: "x"}
	if _, err := r2.LoadTokens(context.Background()); err == nil {
		t.Fatalf("expected json error")
	}
}
