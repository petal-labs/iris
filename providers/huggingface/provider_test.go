package huggingface

import (
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestHuggingFaceImplementsProvider(t *testing.T) {
	p := New("test-key")
	var _ core.Provider = p
}

func TestNew(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		p := New("test-key")

		if p.config.APIKey.Expose() != "test-key" {
			t.Errorf("APIKey = %q, want %q", p.config.APIKey.Expose(), "test-key")
		}
		if p.config.BaseURL != DefaultBaseURL {
			t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultBaseURL)
		}
		if p.config.HTTPClient != http.DefaultClient {
			t.Error("HTTPClient should be http.DefaultClient")
		}
	})

	t.Run("with options", func(t *testing.T) {
		client := &http.Client{Timeout: 30 * time.Second}

		p := New("test-key",
			WithBaseURL("https://custom.api.huggingface.co"),
			WithHTTPClient(client),
			WithHeader("X-Custom", "value"),
			WithTimeout(60*time.Second),
			WithProviderPolicy(PolicyFastest),
		)

		if p.config.BaseURL != "https://custom.api.huggingface.co" {
			t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, "https://custom.api.huggingface.co")
		}
		if p.config.HTTPClient != client {
			t.Error("HTTPClient not set correctly")
		}
		if p.config.Headers.Get("X-Custom") != "value" {
			t.Errorf("Headers[X-Custom] = %q, want %q", p.config.Headers.Get("X-Custom"), "value")
		}
		if p.config.Timeout != 60*time.Second {
			t.Errorf("Timeout = %v, want %v", p.config.Timeout, 60*time.Second)
		}
		if p.config.ProviderPolicy != PolicyFastest {
			t.Errorf("ProviderPolicy = %q, want %q", p.config.ProviderPolicy, PolicyFastest)
		}
	})
}

func TestID(t *testing.T) {
	p := New("test-key")

	if p.ID() != "huggingface" {
		t.Errorf("ID() = %q, want %q", p.ID(), "huggingface")
	}
}

func TestModels(t *testing.T) {
	p := New("test-key")
	models := p.Models()

	// HuggingFace returns empty list since models are dynamic
	if len(models) != 0 {
		t.Errorf("Models() returned %d models, want 0 (dynamic models)", len(models))
	}
}

func TestSupports(t *testing.T) {
	p := New("test-key")

	tests := []struct {
		feature core.Feature
		want    bool
	}{
		{core.FeatureChat, true},
		{core.FeatureChatStreaming, true},
		{core.FeatureToolCalling, true},
		{core.FeatureReasoning, false},
		{core.FeatureImageGeneration, false},
		{core.FeatureEmbeddings, false},
		{core.Feature("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.feature), func(t *testing.T) {
			if got := p.Supports(tt.feature); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestBuildHeaders(t *testing.T) {
	t.Run("required headers", func(t *testing.T) {
		p := New("hf-test-key-123")
		headers := p.buildHeaders()

		auth := headers.Get("Authorization")
		if auth != "Bearer hf-test-key-123" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer hf-test-key-123")
		}

		contentType := headers.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
		}
	})

	t.Run("with custom headers", func(t *testing.T) {
		p := New("test-key",
			WithHeader("X-Custom-One", "value1"),
			WithHeader("X-Custom-Two", "value2"),
		)
		headers := p.buildHeaders()

		if headers.Get("X-Custom-One") != "value1" {
			t.Errorf("X-Custom-One = %q, want %q", headers.Get("X-Custom-One"), "value1")
		}

		if headers.Get("X-Custom-Two") != "value2" {
			t.Errorf("X-Custom-Two = %q, want %q", headers.Get("X-Custom-Two"), "value2")
		}
	})
}

func TestNewFromEnvSuccess(t *testing.T) {
	// Test with HF_TOKEN
	t.Run("HF_TOKEN", func(t *testing.T) {
		originalHF := os.Getenv(HFTokenEnvVar)
		originalHuggingFace := os.Getenv(HuggingFaceTokenEnvVar)
		os.Setenv(HFTokenEnvVar, "hf-test-from-env-123")
		os.Unsetenv(HuggingFaceTokenEnvVar)
		defer func() {
			if originalHF != "" {
				os.Setenv(HFTokenEnvVar, originalHF)
			} else {
				os.Unsetenv(HFTokenEnvVar)
			}
			if originalHuggingFace != "" {
				os.Setenv(HuggingFaceTokenEnvVar, originalHuggingFace)
			}
		}()

		p, err := NewFromEnv()
		if err != nil {
			t.Fatalf("NewFromEnv() error = %v", err)
		}

		if p == nil {
			t.Fatal("NewFromEnv() returned nil provider")
		}

		headers := p.buildHeaders()
		auth := headers.Get("Authorization")
		if auth != "Bearer hf-test-from-env-123" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer hf-test-from-env-123")
		}
	})

	// Test with HUGGINGFACE_TOKEN fallback
	t.Run("HUGGINGFACE_TOKEN fallback", func(t *testing.T) {
		originalHF := os.Getenv(HFTokenEnvVar)
		originalHuggingFace := os.Getenv(HuggingFaceTokenEnvVar)
		os.Unsetenv(HFTokenEnvVar)
		os.Setenv(HuggingFaceTokenEnvVar, "hf-fallback-token")
		defer func() {
			if originalHF != "" {
				os.Setenv(HFTokenEnvVar, originalHF)
			}
			if originalHuggingFace != "" {
				os.Setenv(HuggingFaceTokenEnvVar, originalHuggingFace)
			} else {
				os.Unsetenv(HuggingFaceTokenEnvVar)
			}
		}()

		p, err := NewFromEnv()
		if err != nil {
			t.Fatalf("NewFromEnv() error = %v", err)
		}

		headers := p.buildHeaders()
		auth := headers.Get("Authorization")
		if auth != "Bearer hf-fallback-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer hf-fallback-token")
		}
	})
}

func TestNewFromEnvMissingKey(t *testing.T) {
	originalHF := os.Getenv(HFTokenEnvVar)
	originalHuggingFace := os.Getenv(HuggingFaceTokenEnvVar)
	os.Unsetenv(HFTokenEnvVar)
	os.Unsetenv(HuggingFaceTokenEnvVar)
	defer func() {
		if originalHF != "" {
			os.Setenv(HFTokenEnvVar, originalHF)
		}
		if originalHuggingFace != "" {
			os.Setenv(HuggingFaceTokenEnvVar, originalHuggingFace)
		}
	}()

	_, err := NewFromEnv()
	if err == nil {
		t.Fatal("NewFromEnv() should return error when env var is not set")
	}

	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Errorf("err = %v, want ErrAPIKeyNotFound", err)
	}
}

func TestNewFromEnvWithOptions(t *testing.T) {
	originalHF := os.Getenv(HFTokenEnvVar)
	os.Setenv(HFTokenEnvVar, "hf-test-from-env-456")
	defer func() {
		if originalHF != "" {
			os.Setenv(HFTokenEnvVar, originalHF)
		} else {
			os.Unsetenv(HFTokenEnvVar)
		}
	}()

	customURL := "https://custom.huggingface.co"
	p, err := NewFromEnv(WithBaseURL(customURL))
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}

	if p.config.BaseURL != customURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, customURL)
	}
}

func TestProviderPolicyConstants(t *testing.T) {
	if PolicyAuto != "auto" {
		t.Errorf("PolicyAuto = %q, want %q", PolicyAuto, "auto")
	}
	if PolicyFastest != "fastest" {
		t.Errorf("PolicyFastest = %q, want %q", PolicyFastest, "fastest")
	}
	if PolicyCheapest != "cheapest" {
		t.Errorf("PolicyCheapest = %q, want %q", PolicyCheapest, "cheapest")
	}
}

func TestDefaultBaseURL(t *testing.T) {
	if DefaultBaseURL != "https://router.huggingface.co" {
		t.Errorf("DefaultBaseURL = %q, want %q", DefaultBaseURL, "https://router.huggingface.co")
	}
}

func TestHubAPIBaseURL(t *testing.T) {
	if HubAPIBaseURL != "https://huggingface.co/api" {
		t.Errorf("HubAPIBaseURL = %q, want %q", HubAPIBaseURL, "https://huggingface.co/api")
	}
}
