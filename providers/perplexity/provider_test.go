package perplexity

import (
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestPerplexityImplementsProvider(t *testing.T) {
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
			WithBaseURL("https://custom.api.perplexity.ai"),
			WithHTTPClient(client),
			WithHeader("X-Custom", "value"),
			WithTimeout(60*time.Second),
		)

		if p.config.BaseURL != "https://custom.api.perplexity.ai" {
			t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, "https://custom.api.perplexity.ai")
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
	})
}

func TestID(t *testing.T) {
	p := New("test-key")

	if p.ID() != "perplexity" {
		t.Errorf("ID() = %q, want %q", p.ID(), "perplexity")
	}
}

func TestModels(t *testing.T) {
	p := New("test-key")
	models := p.Models()

	if len(models) < 2 {
		t.Errorf("Models() returned %d models, want at least 2", len(models))
	}

	// Check for required models
	modelIDs := make(map[core.ModelID]bool)
	for _, m := range models {
		modelIDs[m.ID] = true
	}

	if !modelIDs[ModelSonar] {
		t.Error("Models() missing sonar")
	}

	if !modelIDs[ModelSonarPro] {
		t.Error("Models() missing sonar-pro")
	}
}

func TestModelsReturnsCopy(t *testing.T) {
	p := New("test-key")
	models1 := p.Models()
	models2 := p.Models()

	if len(models1) > 0 {
		models1[0].DisplayName = "modified"
	}

	if models2[0].DisplayName == "modified" {
		t.Error("Models() did not return a copy")
	}
}

func TestModelsHaveCapabilities(t *testing.T) {
	p := New("test-key")
	models := p.Models()

	for _, m := range models {
		if len(m.Capabilities) == 0 {
			t.Errorf("Model %s has no capabilities", m.ID)
		}

		// All models should support chat
		hasChat := false
		for _, cap := range m.Capabilities {
			if cap == core.FeatureChat {
				hasChat = true
				break
			}
		}
		if !hasChat {
			t.Errorf("Model %s missing FeatureChat capability", m.ID)
		}
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
		{core.FeatureReasoning, true},
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
		p := New("sk-test-key-123")
		headers := p.buildHeaders()

		auth := headers.Get("Authorization")
		if auth != "Bearer sk-test-key-123" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer sk-test-key-123")
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
	// Set the environment variable
	originalValue := os.Getenv(DefaultAPIKeyEnvVar)
	os.Setenv(DefaultAPIKeyEnvVar, "pplx-test-from-env-123")
	defer func() {
		if originalValue != "" {
			os.Setenv(DefaultAPIKeyEnvVar, originalValue)
		} else {
			os.Unsetenv(DefaultAPIKeyEnvVar)
		}
	}()

	p, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}

	if p == nil {
		t.Fatal("NewFromEnv() returned nil provider")
	}

	// Verify the API key was set correctly
	headers := p.buildHeaders()
	auth := headers.Get("Authorization")
	if auth != "Bearer pplx-test-from-env-123" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer pplx-test-from-env-123")
	}
}

func TestNewFromEnvMissingKey(t *testing.T) {
	// Unset the environment variable
	originalValue := os.Getenv(DefaultAPIKeyEnvVar)
	os.Unsetenv(DefaultAPIKeyEnvVar)
	defer func() {
		if originalValue != "" {
			os.Setenv(DefaultAPIKeyEnvVar, originalValue)
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
	// Set the environment variable
	originalValue := os.Getenv(DefaultAPIKeyEnvVar)
	os.Setenv(DefaultAPIKeyEnvVar, "pplx-test-from-env-456")
	defer func() {
		if originalValue != "" {
			os.Setenv(DefaultAPIKeyEnvVar, originalValue)
		} else {
			os.Unsetenv(DefaultAPIKeyEnvVar)
		}
	}()

	customURL := "https://custom.perplexity.ai"
	p, err := NewFromEnv(WithBaseURL(customURL))
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}

	if p.config.BaseURL != customURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, customURL)
	}
}

func TestGetModelInfo(t *testing.T) {
	t.Run("existing model", func(t *testing.T) {
		info := GetModelInfo(ModelSonar)
		if info == nil {
			t.Fatal("GetModelInfo(ModelSonar) returned nil")
		}
		if info.ID != ModelSonar {
			t.Errorf("ID = %q, want %q", info.ID, ModelSonar)
		}
	})

	t.Run("non-existing model", func(t *testing.T) {
		info := GetModelInfo("non-existent-model")
		if info != nil {
			t.Errorf("GetModelInfo(non-existent) = %v, want nil", info)
		}
	})
}
