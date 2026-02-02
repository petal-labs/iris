package anthropic

import (
	"net/http"
	"testing"
	"time"
)

func TestWithBaseURL(t *testing.T) {
	cfg := &Config{}
	WithBaseURL("https://custom.api.com")(cfg)

	if cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want 'https://custom.api.com'", cfg.BaseURL)
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 10 * time.Second}
	cfg := &Config{}
	WithHTTPClient(customClient)(cfg)

	if cfg.HTTPClient != customClient {
		t.Error("HTTPClient not set correctly")
	}
}

func TestWithVersion(t *testing.T) {
	cfg := &Config{}
	WithVersion("2024-01-01")(cfg)

	if cfg.Version != "2024-01-01" {
		t.Errorf("Version = %q, want '2024-01-01'", cfg.Version)
	}
}

func TestWithHeader(t *testing.T) {
	cfg := &Config{}
	WithHeader("X-Custom", "value1")(cfg)
	WithHeader("X-Another", "value2")(cfg)

	if cfg.Headers.Get("X-Custom") != "value1" {
		t.Errorf("X-Custom = %q, want 'value1'", cfg.Headers.Get("X-Custom"))
	}

	if cfg.Headers.Get("X-Another") != "value2" {
		t.Errorf("X-Another = %q, want 'value2'", cfg.Headers.Get("X-Another"))
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := &Config{}
	WithTimeout(30 * time.Second)(cfg)

	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultBaseURL != "https://api.anthropic.com" {
		t.Errorf("DefaultBaseURL = %q, want 'https://api.anthropic.com'", DefaultBaseURL)
	}

	if DefaultVersion != "2023-06-01" {
		t.Errorf("DefaultVersion = %q, want '2023-06-01'", DefaultVersion)
	}
}

func TestWithFilesAPIBeta(t *testing.T) {
	cfg := &Config{}
	WithFilesAPIBeta("files-api-2025-04-14")(cfg)

	if cfg.FilesAPIBeta != "files-api-2025-04-14" {
		t.Errorf("FilesAPIBeta = %q, want 'files-api-2025-04-14'", cfg.FilesAPIBeta)
	}
}

func TestDefaultFilesAPIBeta(t *testing.T) {
	if DefaultFilesAPIBeta != "files-api-2025-04-14" {
		t.Errorf("DefaultFilesAPIBeta = %q, want 'files-api-2025-04-14'", DefaultFilesAPIBeta)
	}
}
