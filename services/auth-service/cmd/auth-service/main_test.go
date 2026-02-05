package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLogDir(t *testing.T) {
	if err := ensureLogDir(""); err != nil {
		t.Fatalf("empty path should be noop: %v", err)
	}
	if err := ensureLogDir("app.log"); err != nil {
		t.Fatalf("relative file in current dir should be noop: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "nested", "logs")
	path := filepath.Join(dir, "auth.log")
	if err := ensureLogDir(path); err != nil {
		t.Fatalf("ensureLogDir failed: %v", err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected directory to be created, err=%v", err)
	}
}
