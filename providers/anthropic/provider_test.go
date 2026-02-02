package anthropic

import (
	"net/http"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestNew(t *testing.T) {
	p := New("test-key")

	if p.config.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want 'test-key'", p.config.APIKey)
	}

	if p.config.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultBaseURL)
	}

	if p.config.Version != DefaultVersion {
		t.Errorf("Version = %q, want %q", p.config.Version, DefaultVersion)
	}

	if p.config.HTTPClient != http.DefaultClient {
		t.Error("HTTPClient should be http.DefaultClient")
	}
}

func TestNewWithOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 30 * time.Second}

	p := New("test-key",
		WithBaseURL("https://custom.api.com"),
		WithHTTPClient(customClient),
		WithVersion("2024-01-01"),
		WithHeader("X-Custom", "value"),
		WithTimeout(60*time.Second),
	)

	if p.config.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want 'https://custom.api.com'", p.config.BaseURL)
	}

	if p.config.HTTPClient != customClient {
		t.Error("HTTPClient should be custom client")
	}

	if p.config.Version != "2024-01-01" {
		t.Errorf("Version = %q, want '2024-01-01'", p.config.Version)
	}

	if p.config.Headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom header = %q, want 'value'", p.config.Headers.Get("X-Custom"))
	}

	if p.config.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", p.config.Timeout)
	}
}

func TestID(t *testing.T) {
	p := New("test-key")

	if p.ID() != "anthropic" {
		t.Errorf("ID() = %q, want 'anthropic'", p.ID())
	}
}

func TestModels(t *testing.T) {
	p := New("test-key")
	models := p.Models()

	if len(models) != 3 {
		t.Errorf("Models() count = %d, want 3", len(models))
	}

	// Verify model IDs
	modelIDs := make(map[core.ModelID]bool)
	for _, m := range models {
		modelIDs[m.ID] = true
	}

	expected := []core.ModelID{
		ModelClaudeSonnet45,
		ModelClaudeHaiku45,
		ModelClaudeOpus45,
	}

	for _, id := range expected {
		if !modelIDs[id] {
			t.Errorf("Missing model: %s", id)
		}
	}
}

func TestModelsReturnsCopy(t *testing.T) {
	p := New("test-key")

	models1 := p.Models()
	models2 := p.Models()

	// Modify first slice
	models1[0].DisplayName = "Modified"

	// Second slice should be unchanged
	if models2[0].DisplayName == "Modified" {
		t.Error("Models() should return a copy")
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
		{core.FeatureBuiltInTools, false},
		{core.FeatureResponseChain, false},
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
	p := New("test-key", WithHeader("X-Custom", "value"))
	headers := p.buildHeaders()

	if headers.Get("x-api-key") != "test-key" {
		t.Errorf("x-api-key = %q, want 'test-key'", headers.Get("x-api-key"))
	}

	if headers.Get("anthropic-version") != DefaultVersion {
		t.Errorf("anthropic-version = %q, want %q", headers.Get("anthropic-version"), DefaultVersion)
	}

	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", headers.Get("Content-Type"))
	}

	if headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want 'value'", headers.Get("X-Custom"))
	}
}

func TestGetModelInfo(t *testing.T) {
	tests := []struct {
		id      core.ModelID
		wantNil bool
		wantID  core.ModelID
	}{
		{ModelClaudeSonnet45, false, ModelClaudeSonnet45},
		{ModelClaudeHaiku45, false, ModelClaudeHaiku45},
		{ModelClaudeOpus45, false, ModelClaudeOpus45},
		{"unknown-model", true, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			info := GetModelInfo(tt.id)

			if tt.wantNil {
				if info != nil {
					t.Errorf("GetModelInfo(%q) should be nil", tt.id)
				}
				return
			}

			if info == nil {
				t.Fatalf("GetModelInfo(%q) = nil, want non-nil", tt.id)
			}

			if info.ID != tt.wantID {
				t.Errorf("ModelInfo.ID = %q, want %q", info.ID, tt.wantID)
			}
		})
	}
}

func TestModelCapabilities(t *testing.T) {
	for _, id := range []core.ModelID{ModelClaudeSonnet45, ModelClaudeHaiku45, ModelClaudeOpus45} {
		t.Run(string(id), func(t *testing.T) {
			info := GetModelInfo(id)
			if info == nil {
				t.Fatalf("GetModelInfo(%q) = nil", id)
			}

			// All models should have these capabilities
			expected := []core.Feature{
				core.FeatureChat,
				core.FeatureChatStreaming,
				core.FeatureToolCalling,
			}

			for _, cap := range expected {
				if !info.HasCapability(cap) {
					t.Errorf("Model %s missing capability %s", id, cap)
				}
			}
		})
	}
}

func TestProviderImplementsInterface(t *testing.T) {
	var _ core.Provider = (*Anthropic)(nil)
}

func TestNewWithDefaultFilesAPIBeta(t *testing.T) {
	p := New("test-key")
	if p.config.FilesAPIBeta != DefaultFilesAPIBeta {
		t.Errorf("expected FilesAPIBeta %q, got %q", DefaultFilesAPIBeta, p.config.FilesAPIBeta)
	}
}
