package gemini

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
	customClient := &http.Client{Timeout: 30 * time.Second}
	cfg := &Config{}
	WithHTTPClient(customClient)(cfg)

	if cfg.HTTPClient != customClient {
		t.Error("HTTPClient should be custom client")
	}
}

func TestWithHeader(t *testing.T) {
	cfg := &Config{}
	WithHeader("X-Custom", "value")(cfg)

	if cfg.Headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom header = %q, want 'value'", cfg.Headers.Get("X-Custom"))
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := &Config{}
	WithTimeout(60 * time.Second)(cfg)

	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
	}
}
