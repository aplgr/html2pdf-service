package utils

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetTokensCache() {
	tokens.Lock()
	tokens.cache = nil
	tokens.Unlock()
}

func TestLoadTokensAndValidation(t *testing.T) {
	defer resetTokensCache()

	LoadTokensFromMap(map[string]int{"a": 5, "b": 10})

	assert.True(t, ValidateToken("a"))
	assert.Equal(t, 5, GetRateLimit("a"))
	assert.True(t, ValidateToken("b"))
	assert.Equal(t, 10, GetRateLimit("b"))
	assert.False(t, ValidateToken("c"))
	assert.Equal(t, 0, GetRateLimit("c"))
}

func TestLoadTokensUpdatesCache(t *testing.T) {
	defer resetTokensCache()

	LoadTokensFromMap(map[string]int{"a": 5, "b": 10})
	assert.Equal(t, 10, GetRateLimit("b"))

	LoadTokensFromMap(map[string]int{"a": 7, "c": 12})

	assert.True(t, ValidateToken("a"))
	assert.Equal(t, 7, GetRateLimit("a"))
	assert.False(t, ValidateToken("b"))
	assert.True(t, ValidateToken("c"))
	assert.Equal(t, 12, GetRateLimit("c"))
}

func TestPostgresDSN_BuildsURL(t *testing.T) {
	dsn, err := postgresDSN(PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "html2pdf",
		User:     "user",
		Password: "p@ss word",
		SSLMode:  "disable",
	})
	assert.NoError(t, err)

	u, err := url.Parse(dsn)
	assert.NoError(t, err)
	assert.Equal(t, "postgres", u.Scheme)
	assert.Equal(t, "localhost:5432", u.Host)
	assert.Equal(t, "/html2pdf", u.Path)
	assert.Equal(t, "user", u.User.Username())
	pw, ok := u.User.Password()
	assert.True(t, ok)
	assert.Equal(t, "p@ss word", pw)
	assert.Equal(t, "disable", u.Query().Get("sslmode"))
}

func TestPostgresDSN_Passthrough(t *testing.T) {
	raw := "postgres://u:p@localhost:5432/db?sslmode=disable"
	dsn, err := postgresDSN(PostgresConfig{Host: raw})
	assert.NoError(t, err)
	assert.Equal(t, raw, dsn)
}
