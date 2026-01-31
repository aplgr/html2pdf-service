package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr string `yaml:"listen_addr"`

	PostgresDSN string `yaml:"postgres_dsn"`

	Logger LoggerConfig `yaml:"logger"`

	RedisAddr     string `yaml:"redis_addr"`
	RedisPassword string `yaml:"redis_password"`
	RedisRateDB   int    `yaml:"redis_rate_db"`

	TokenReloadInterval time.Duration `yaml:"token_reload_interval"`

	RateInterval           time.Duration `yaml:"rate_interval"`
	EnableUserLimiter      bool          `yaml:"enable_user_limiter"`
	UserLimit              int           `yaml:"user_limit"`
	EnableTokenRateLimiter bool          `yaml:"enable_token_rate_limiter"`
}

type LoggerConfig struct {
	File       string `yaml:"file"`
	Level      string `yaml:"level"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

// Load loads the configuration. You can override the path via CONFIG_PATH.
func Load() Config {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config/auth-service.yaml"
	}
	return LoadFrom(path)
}

// LoadFrom loads the configuration from the specified YAML file path.
// Panics if the file cannot be read or the format is invalid.
func LoadFrom(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		panic("Error reading " + path + ": " + err.Error())
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic("Invalid YAML format in " + path + ": " + err.Error())
	}

	if cfg.PostgresDSN == "" {
		panic("postgres_dsn must be set")
	}
	if cfg.RateInterval <= 0 {
		panic("rate_interval must be > 0")
	}
	if cfg.UserLimit < 0 {
		panic("user_limit must be >= 0")
	}
	if cfg.TokenReloadInterval <= 0 {
		panic("token_reload_interval must be > 0")
	}

	return cfg
}
