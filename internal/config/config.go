package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProxyURL       string        `yaml:"proxy_url"`
	Timeout        time.Duration `yaml:"timeout"`
	CacheTTL       time.Duration `yaml:"cache_ttl"`
	MaxConcurrent  int           `yaml:"max_concurrent"`
	DefaultVerbose bool          `yaml:"default_verbose"`
	DefaultQuiet   bool          `yaml:"default_quiet"`
}

var defaults = Config{
	ProxyURL:      "https://proxy.golang.org",
	Timeout:       30 * time.Second,
	CacheTTL:      5 * time.Minute,
	MaxConcurrent: 10,
}

func Load() (*Config, error) {
	cfg := defaults

	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".config", "gx", "config.yaml"),
		filepath.Join(os.Getenv("HOME"), ".gx.yaml"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		break
	}

	applyEnvOverrides(&cfg)

	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GX_PROXY"); v != "" {
		cfg.ProxyURL = v
	}
	if v := os.Getenv("GX_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}
	if v := os.Getenv("GX_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CacheTTL = d
		}
	}
	if v := os.Getenv("GX_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxConcurrent = n
		}
	}
}

func Default() *Config {
	return &defaults
}
