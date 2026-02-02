package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()

	if path == "" {
		t.Error("DefaultConfigPath() returned empty string")
	}

	// Should end with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultConfigPath() = %q, should end with config.yaml", path)
	}

	// Should contain .iris directory
	dir := filepath.Dir(path)
	if filepath.Base(dir) != ".iris" {
		t.Errorf("DefaultConfigPath() = %q, should be in .iris directory", path)
	}
}

func TestDefaultConfigPathPlatform(t *testing.T) {
	path := DefaultConfigPath()

	if runtime.GOOS == "windows" {
		// Should use USERPROFILE on Windows
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" && !strings.HasPrefix(path, userProfile) {
			t.Logf("Note: path %q doesn't start with USERPROFILE %q", path, userProfile)
		}
	} else {
		// Should use HOME on Unix
		home := os.Getenv("HOME")
		if home != "" && !strings.HasPrefix(path, home) {
			t.Logf("Note: path %q doesn't start with HOME %q", path, home)
		}
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")

	if err != nil {
		t.Errorf("LoadConfig() error = %v, want nil for missing file", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Should return empty config
	if cfg.DefaultProvider != "" {
		t.Errorf("DefaultProvider = %q, want empty", cfg.DefaultProvider)
	}
	if cfg.DefaultModel != "" {
		t.Errorf("DefaultModel = %q, want empty", cfg.DefaultModel)
	}
	if cfg.Providers == nil {
		t.Error("Providers map is nil")
	}
}

func TestLoadConfigValid(t *testing.T) {
	// Create temp config file
	content := `
default_provider: openai
default_model: gpt-4o

providers:
  openai:
    api_key_ref: openai_key
    base_url: https://api.openai.com/v1
  anthropic:
    api_key_ref: anthropic_key
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want openai", cfg.DefaultProvider)
	}
	if cfg.DefaultModel != "gpt-4o" {
		t.Errorf("DefaultModel = %q, want gpt-4o", cfg.DefaultModel)
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("len(Providers) = %d, want 2", len(cfg.Providers))
	}

	openai := cfg.Providers["openai"]
	if openai.APIKeyRef != "openai_key" {
		t.Errorf("openai.APIKeyRef = %q, want openai_key", openai.APIKeyRef)
	}
	if openai.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("openai.BaseURL = %q, want https://api.openai.com/v1", openai.BaseURL)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	// YAML that will cause unmarshal error (wrong type)
	content := `
default_provider: [invalid, array, instead, of, string]
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("LoadConfig() should return error for invalid YAML")
	}
}

func TestLoadConfigEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should return empty config with initialized Providers
	if cfg.Providers == nil {
		t.Error("Providers map is nil for empty file")
	}
}

func TestLoadConfigMinimal(t *testing.T) {
	content := `default_provider: openai`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.DefaultProvider != "openai" {
		t.Errorf("DefaultProvider = %q, want openai", cfg.DefaultProvider)
	}
	if cfg.Providers == nil {
		t.Error("Providers map is nil")
	}
}

func TestConfigGetProvider(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"openai": {
				APIKeyRef: "openai_key",
				BaseURL:   "https://api.openai.com/v1",
			},
		},
	}

	pc := cfg.GetProvider("openai")
	if pc == nil {
		t.Fatal("GetProvider(openai) returned nil")
	}
	if pc.APIKeyRef != "openai_key" {
		t.Errorf("APIKeyRef = %q, want openai_key", pc.APIKeyRef)
	}

	pc = cfg.GetProvider("nonexistent")
	if pc != nil {
		t.Error("GetProvider(nonexistent) should return nil")
	}
}

func TestConfigGetProviderNilMap(t *testing.T) {
	cfg := &Config{Providers: nil}

	pc := cfg.GetProvider("openai")
	if pc != nil {
		t.Error("GetProvider on nil Providers should return nil")
	}
}
