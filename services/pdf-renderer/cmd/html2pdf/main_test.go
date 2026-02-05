package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"pdf-renderer/internal/config"
)

func TestStartServer_GracefulShutdownOnSignal(t *testing.T) {
	app := fiber.New()
	var cfg config.Config
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = ":0"

	idleConnsClosed := make(chan struct{})
	go startServer(app, cfg, idleConnsClosed)

	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	select {
	case <-idleConnsClosed:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

func TestMain_UsesConfigAndShutsDown(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cfg.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  host: "127.0.0.1"
  port: ":0"
  prefork: false
limits:
  max_html_bytes: 1048576
  max_pdf_bytes: 1048576
logger:
  file: "`+filepath.Join(t.TempDir(), `renderer.log`)+`"
  level: "info"
  max_size_mb: 1
  max_backups: 1
  max_age_days: 1
  compress: false
cache:
  pdf_cache_enabled: false
  pdf_cache_ttl: 1m
  redis_host: "127.0.0.1:1"
  redis_rate_db: 0
  redis_pdf_db: 1
pdf:
  default_paper: "A4"
  paper_sizes:
    A4:
      width: 8.27
      height: 11.69
  timeout_secs: 1
  chrome_path: ""
  chrome_no_sandbox: true
  chrome_pool_size: 0
  user_data_dir: ""
`), 0o644)
	if err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	t.Setenv("CONFIG_PATH", cfgPath)
	t.Setenv("CHROME_BIN", "/bin/true")

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("signal main: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for main to exit")
	}
}
