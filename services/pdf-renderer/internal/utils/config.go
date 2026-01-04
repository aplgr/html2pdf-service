package utils

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the full application configuration, loaded from a YAML file.
type Config struct {
	Server struct {
		Host    string `yaml:"host"`    // Host address to bind the service to
		Port    string `yaml:"port"`    // Port on which the service listens
		Prefork bool   `yaml:"prefork"` // Enable Fiber prefork mode (multi-process)
	} `yaml:"server"`

	Limits struct {
		MaxHTMLBytes int `yaml:"max_html_bytes"` // Maximum size of HTML input in bytes
		MaxPDFBytes  int `yaml:"max_pdf_bytes"`  // Maximum size of generated PDF in bytes
	} `yaml:"limits"`

	Logger struct {
		File       string `yaml:"file"`         // Path to the log file
		Level      string `yaml:"level"`        // Logging verbosity level
		MaxSizeMB  int    `yaml:"max_size_mb"`  // Max log file size before rotation (in MB)
		MaxBackups int    `yaml:"max_backups"`  // Number of old log files to retain
		MaxAgeDays int    `yaml:"max_age_days"` // Number of days to retain old log files
		Compress   bool   `yaml:"compress"`     // Whether to compress old log files
	} `yaml:"logger"`

	Cache struct {
		PDFCacheEnabled bool          `yaml:"pdf_cache_enabled"` // Whether caching generated PDFs in Redis is enabled
		PDFCacheTTL     time.Duration `yaml:"pdf_cache_ttl"`     // TTL for cached PDFs in Redis (e.g. 5m). If 0, a safe default is used
		RedisHost       string        `yaml:"redis_host"`        // Redis server host (optional)
		RateLimitDB     int           `yaml:"redis_rate_db"`     // Redis DB for rate limiting
		PDFCacheDB      int           `yaml:"redis_pdf_db"`      // Redis DB for PDF caching
	} `yaml:"cache"`

	PDF struct {
		DefaultPaper    string               `yaml:"default_paper"`     // Default paper format if none is provided
		PaperSizes      map[string]PaperSize `yaml:"paper_sizes"`       // Map of available paper formats and their sizes
		TimeoutSecs     int                  `yaml:"timeout_secs"`      // Timeout for PDF generation in seconds
		ChromePath      string               `yaml:"chrome_path"`       // Path to the Chrome binary
		ChromeNoSandbox bool                 `yaml:"chrome_no_sandbox"` // Whether to launch Chrome with --no-sandbox
		ChromePoolSize  int                  `yaml:"chrome_pool_size"`  // Number of preloaded Chrome tabs (0 = disabled)
		UserDataDir     string               `yaml:"user_data_dir"`     // Optional fixed user data dir (recommended when pooling)
	} `yaml:"pdf"`
}

// PaperSize defines width and height in inches for a specific paper format.
type PaperSize struct {
	Width  float64 `yaml:"width"`  // Width in inches
	Height float64 `yaml:"height"` // Height in inches
}

var (
	mu        sync.RWMutex // Mutex to guard access to AppConfig
	AppConfig Config       // Global application configuration
)

// LoadConfig loads the configuration. You can override the path via CONFIG_PATH.
func LoadConfig() Config {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config/html2pdf.yaml"
	}
	return LoadConfigFrom(path)
}

// LoadConfigFrom loads the configuration from the specified YAML file path.
// Panics if the file cannot be read or the format is invalid.
func LoadConfigFrom(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		panic("Error reading " + path + ": " + err.Error())
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic("Invalid YAML format in " + path + ": " + err.Error())
	}

	mu.Lock()
	AppConfig = cfg
	mu.Unlock()

	return cfg
}

// GetConfig returns the current application configuration in a thread-safe manner.
func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return AppConfig
}
