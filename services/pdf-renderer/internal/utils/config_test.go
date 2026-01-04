package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Ensure the directory structure matches the real repo layout:
	// ./config/html2pdf.yaml
	filePath := filepath.Join(tmpDir, "config", "html2pdf.yaml")
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	assert.NoError(t, err)

	err = os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err)

	return filePath
}

func TestLoadConfig_ValidFile(t *testing.T) {
	configYAML := `
server:
  host: "127.0.0.1"
  port: ":8080"
  prefork: true

limits:
  max_html_bytes: 2048
  max_pdf_bytes: 10485760

logger:
  file: "app.log"
  level: "debug"
  max_size_mb: 10
  max_backups: 5
  max_age_days: 7
  compress: true

cache:
  pdf_cache_enabled: true
  pdf_cache_ttl: 5m
  redis_host: "localhost:6379"
  redis_rate_db: 0
  redis_pdf_db: 1

auth:
  postgres:
    host: "localhost"
    port: 5432
    database: "html2pdf"
    user: "html2pdf"
    password: "secret"
    sslmode: "disable"

rate_limiter:
  interval: 1h
  enable_user_limiter: true
  user_limit: 15

pdf:
  default_paper: "A4"
  paper_sizes:
    A4:
      width: 8.27
      height: 11.69
  timeout_secs: 15
  chrome_path: "/usr/bin/chrome"
  chrome_no_sandbox: true
  chrome_pool_size: 2
  user_data_dir: "/tmp/chrome-profile"
`

	path := writeTempConfig(t, configYAML)

	cfg := LoadConfigFrom(path)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, ":8080", cfg.Server.Port)
	assert.True(t, cfg.Server.Prefork)

	assert.Equal(t, 2048, cfg.Limits.MaxHTMLBytes)
	assert.Equal(t, 10485760, cfg.Limits.MaxPDFBytes)

	assert.Equal(t, "app.log", cfg.Logger.File)
	assert.Equal(t, "debug", cfg.Logger.Level)
	assert.Equal(t, 10, cfg.Logger.MaxSizeMB)
	assert.Equal(t, 5, cfg.Logger.MaxBackups)
	assert.Equal(t, 7, cfg.Logger.MaxAgeDays)
	assert.True(t, cfg.Logger.Compress)

	assert.True(t, cfg.Cache.PDFCacheEnabled)
	assert.Equal(t, 5*time.Minute, cfg.Cache.PDFCacheTTL)
	assert.Equal(t, "localhost:6379", cfg.Cache.RedisHost)
	assert.Equal(t, 0, cfg.Cache.RateLimitDB)
	assert.Equal(t, 1, cfg.Cache.PDFCacheDB)

	assert.Equal(t, "A4", cfg.PDF.DefaultPaper)
	assert.Contains(t, cfg.PDF.PaperSizes, "A4")
	assert.InEpsilon(t, 8.27, cfg.PDF.PaperSizes["A4"].Width, 0.01)
	assert.InEpsilon(t, 11.69, cfg.PDF.PaperSizes["A4"].Height, 0.01)

	assert.Equal(t, 15, cfg.PDF.TimeoutSecs)
	assert.Equal(t, "/usr/bin/chrome", cfg.PDF.ChromePath)
	assert.True(t, cfg.PDF.ChromeNoSandbox)
	assert.Equal(t, 2, cfg.PDF.ChromePoolSize)
	assert.Equal(t, "/tmp/chrome-profile", cfg.PDF.UserDataDir)
}

func TestGetConfig_ReturnsLoadedConfig(t *testing.T) {
	AppConfig.Server.Host = "testhost"
	AppConfig.Limits.MaxHTMLBytes = 12345

	cfg := GetConfig()

	assert.Equal(t, "testhost", cfg.Server.Host)
	assert.Equal(t, 12345, cfg.Limits.MaxHTMLBytes)
}

func TestLoadConfigFrom_FileNotFound(t *testing.T) {
	assert.Panics(t, func() {
		LoadConfigFrom("non_existent.yaml")
	})
}
func TestLoadConfigFrom_InvalidYAML(t *testing.T) {
	invalidYAML := `
server:
  host: "127.0.0.1
` // missing quotes

	tmp := writeTempConfig(t, invalidYAML)
	assert.Panics(t, func() {
		LoadConfigFrom(tmp)
	})
}

func TestLoadConfigFrom_MissingSection(t *testing.T) {
	yaml := `
server:
  host: "localhost"
  port: "3000"
` // Rest (logger, pdf, etc.) is missing

	tmp := writeTempConfig(t, yaml)
	cfg := LoadConfigFrom(tmp)

	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, "3000", cfg.Server.Port)
	// Optional: assert default zero values
	assert.Equal(t, "", cfg.Logger.File)
	assert.Equal(t, 0, cfg.Limits.MaxHTMLBytes)
}

func TestLoadConfig_PrefersConfigPathEnvVarWhenPresent(t *testing.T) {
	tmpDir := t.TempDir()

	cfgDefault := `cache:
  redis_host: "default:6379"
`
	cfgEnv := `cache:
  redis_host: "env:6379"
`

	// Create default config at ./config/html2pdf.yaml
	defaultPath := filepath.Join(tmpDir, "config", "html2pdf.yaml")
	assert.NoError(t, os.MkdirAll(filepath.Dir(defaultPath), 0755))
	assert.NoError(t, os.WriteFile(defaultPath, []byte(cfgDefault), 0644))

	// Create alternative config used via CONFIG_PATH
	envPath := filepath.Join(tmpDir, "alt.yaml")
	assert.NoError(t, os.WriteFile(envPath, []byte(cfgEnv), 0644))

	// Run from tmpDir so the default relative path exists.
	wd, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// Backup/restore CONFIG_PATH
	oldEnv, hadEnv := os.LookupEnv("CONFIG_PATH")
	t.Cleanup(func() {
		if hadEnv {
			_ = os.Setenv("CONFIG_PATH", oldEnv)
		} else {
			os.Unsetenv("CONFIG_PATH")
		}
	})

	// 1) No env var -> default path
	os.Unsetenv("CONFIG_PATH")
	cfg := LoadConfig()
	assert.Equal(t, "default:6379", cfg.Cache.RedisHost)

	// 2) Env var set -> env path wins
	assert.NoError(t, os.Setenv("CONFIG_PATH", envPath))
	cfg = LoadConfig()
	assert.Equal(t, "env:6379", cfg.Cache.RedisHost)
}
