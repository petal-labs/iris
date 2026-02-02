package openai

import (
	"net/http"
	"testing"
	"time"
)

func TestWithBaseURL(t *testing.T) {
	cfg := Config{}
	WithBaseURL("https://custom.api.com/v1")(&cfg)

	if cfg.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://custom.api.com/v1")
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 30 * time.Second}
	cfg := Config{}
	WithHTTPClient(customClient)(&cfg)

	if cfg.HTTPClient != customClient {
		t.Error("HTTPClient not set correctly")
	}
}

func TestWithOrgID(t *testing.T) {
	cfg := Config{}
	WithOrgID("org-12345")(&cfg)

	if cfg.OrgID != "org-12345" {
		t.Errorf("OrgID = %q, want %q", cfg.OrgID, "org-12345")
	}
}

func TestWithProjectID(t *testing.T) {
	cfg := Config{}
	WithProjectID("proj-67890")(&cfg)

	if cfg.ProjectID != "proj-67890" {
		t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "proj-67890")
	}
}

func TestWithHeader(t *testing.T) {
	cfg := Config{}
	WithHeader("X-Custom-Header", "custom-value")(&cfg)

	if cfg.Headers == nil {
		t.Fatal("Headers is nil")
	}

	if cfg.Headers.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Header X-Custom-Header = %q, want %q", cfg.Headers.Get("X-Custom-Header"), "custom-value")
	}
}

func TestWithHeaderMultiple(t *testing.T) {
	cfg := Config{}
	WithHeader("X-First", "first")(&cfg)
	WithHeader("X-Second", "second")(&cfg)

	if cfg.Headers.Get("X-First") != "first" {
		t.Errorf("Header X-First = %q, want %q", cfg.Headers.Get("X-First"), "first")
	}

	if cfg.Headers.Get("X-Second") != "second" {
		t.Errorf("Header X-Second = %q, want %q", cfg.Headers.Get("X-Second"), "second")
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := Config{}
	WithTimeout(45 * time.Second)(&cfg)

	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 45*time.Second)
	}
}

func TestDefaultValues(t *testing.T) {
	p := New("test-key")

	if p.config.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", p.config.APIKey, "test-key")
	}

	if p.config.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultBaseURL)
	}

	if p.config.HTTPClient != http.DefaultClient {
		t.Error("HTTPClient should default to http.DefaultClient")
	}
}

func TestOptionsApplied(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}

	p := New("test-key",
		WithBaseURL("https://custom.api.com"),
		WithHTTPClient(customClient),
		WithOrgID("my-org"),
		WithProjectID("my-project"),
		WithHeader("X-Custom", "value"),
		WithTimeout(30*time.Second),
	)

	if p.config.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, "https://custom.api.com")
	}

	if p.config.HTTPClient != customClient {
		t.Error("HTTPClient not applied")
	}

	if p.config.OrgID != "my-org" {
		t.Errorf("OrgID = %q, want %q", p.config.OrgID, "my-org")
	}

	if p.config.ProjectID != "my-project" {
		t.Errorf("ProjectID = %q, want %q", p.config.ProjectID, "my-project")
	}

	if p.config.Headers.Get("X-Custom") != "value" {
		t.Errorf("Header X-Custom = %q, want %q", p.config.Headers.Get("X-Custom"), "value")
	}

	if p.config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", p.config.Timeout, 30*time.Second)
	}
}
