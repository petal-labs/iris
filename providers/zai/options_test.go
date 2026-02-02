package zai

import (
	"net/http"
	"testing"
	"time"
)

func TestNewWithDefaults(t *testing.T) {
	p := New("test-key")

	if p.config.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", p.config.APIKey, "test-key")
	}

	if p.config.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultBaseURL)
	}

	if p.config.HTTPClient != http.DefaultClient {
		t.Error("HTTPClient should be http.DefaultClient")
	}
}

func TestWithBaseURL(t *testing.T) {
	customURL := "https://custom.api.z.ai/v1"
	p := New("test-key", WithBaseURL(customURL))

	if p.config.BaseURL != customURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, customURL)
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 30 * time.Second}
	p := New("test-key", WithHTTPClient(customClient))

	if p.config.HTTPClient != customClient {
		t.Error("HTTPClient should be custom client")
	}
}

func TestWithHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Custom", "value")
	p := New("test-key", WithHeaders(headers))

	if p.config.Headers.Get("X-Custom") != "value" {
		t.Errorf("Headers[X-Custom] = %q, want %q", p.config.Headers.Get("X-Custom"), "value")
	}
}

func TestWithTimeout(t *testing.T) {
	timeout := 60 * time.Second
	p := New("test-key", WithTimeout(timeout))

	if p.config.Timeout != timeout {
		t.Errorf("Timeout = %v, want %v", p.config.Timeout, timeout)
	}
}

func TestMultipleOptions(t *testing.T) {
	customURL := "https://custom.api.z.ai/v1"
	customClient := &http.Client{Timeout: 30 * time.Second}
	timeout := 60 * time.Second

	p := New("test-key",
		WithBaseURL(customURL),
		WithHTTPClient(customClient),
		WithTimeout(timeout),
	)

	if p.config.BaseURL != customURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, customURL)
	}
	if p.config.HTTPClient != customClient {
		t.Error("HTTPClient should be custom client")
	}
	if p.config.Timeout != timeout {
		t.Errorf("Timeout = %v, want %v", p.config.Timeout, timeout)
	}
}
