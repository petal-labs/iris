// Package config handles CLI configuration loading and management.
package config

import (
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config represents the CLI configuration.
type Config struct {
	DefaultProvider string                    `yaml:"default_provider"`
	DefaultModel    string                    `yaml:"default_model"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
}

// ProviderConfig holds configuration for a specific provider.
type ProviderConfig struct {
	APIKeyRef string `yaml:"api_key_ref"`
	BaseURL   string `yaml:"base_url,omitempty"`
}

// DefaultConfigPath returns the default configuration file path for the current platform.
// - macOS/Linux: ~/.iris/config.yaml
// - Windows: %USERPROFILE%\.iris\config.yaml
func DefaultConfigPath() string {
	var homeDir string

	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
	} else {
		homeDir = os.Getenv("HOME")
	}

	if homeDir == "" {
		// Fallback to current directory
		return "config.yaml"
	}

	return filepath.Join(homeDir, ".iris", "config.yaml")
}

// LoadConfig loads configuration from the specified path.
// If the file doesn't exist, returns an empty config without error.
// Returns an error only if the file exists but cannot be read or parsed.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Providers: make(map[string]ProviderConfig),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Missing config file is not an error
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure Providers map is initialized
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}

	return cfg, nil
}

// GetProvider returns the provider config for the given ID.
// Returns nil if the provider is not configured.
func (c *Config) GetProvider(id string) *ProviderConfig {
	if c.Providers == nil {
		return nil
	}
	if pc, ok := c.Providers[id]; ok {
		return &pc
	}
	return nil
}
