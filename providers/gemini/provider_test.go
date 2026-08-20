package gemini

import (
	"net/http"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestNew(t *testing.T) {
	p := New("test-key")

	if p.config.APIKey.Expose() != "test-key" {
		t.Errorf("APIKey = %q, want 'test-key'", p.config.APIKey.Expose())
	}

	if p.config.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultBaseURL)
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
		WithHeader("X-Custom", "value"),
		WithTimeout(60*time.Second),
	)

	if p.config.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want 'https://custom.api.com'", p.config.BaseURL)
	}

	if p.config.HTTPClient != customClient {
		t.Error("HTTPClient should be custom client")
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

	if p.ID() != "gemini" {
		t.Errorf("ID() = %q, want 'gemini'", p.ID())
	}
}

func TestModels(t *testing.T) {
	p := New("test-key")
	models := p.Models()

	if len(models) != 15 {
		t.Errorf("Models() count = %d, want 15", len(models))
	}

	// Verify model IDs
	modelIDs := make(map[core.ModelID]bool)
	for _, m := range models {
		modelIDs[m.ID] = true
	}

	expected := []core.ModelID{
		ModelGemini36Flash,
		ModelGemini35Flash,
		ModelGemini35FlashLite,
		ModelGemini31Pro,
		ModelGemini31FlashLite,
		ModelGemini31FlashImagePreview,
		ModelGemini3Pro,
		ModelGemini3Flash,
		ModelGemini3ProImage,
		ModelGemini25Flash,
		ModelGemini25FlashLite,
		ModelGemini25Pro,
		ModelGemini20FlashLite,
		ModelGemini25FlashImage,
		ModelGeminiEmbedding001,
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
		{core.FeatureReasoning, true},
		{core.FeatureImageGeneration, true},
		{core.FeatureEmbeddings, true},
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

	if headers.Get("x-goog-api-key") != "test-key" {
		t.Errorf("x-goog-api-key = %q, want 'test-key'", headers.Get("x-goog-api-key"))
	}

	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", headers.Get("Content-Type"))
	}

	if headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want 'value'", headers.Get("X-Custom"))
	}
}

func TestBuildHeadersNoExtraHeaders(t *testing.T) {
	p := New("test-key")
	headers := p.buildHeaders()

	if headers.Get("x-goog-api-key") != "test-key" {
		t.Errorf("x-goog-api-key = %q, want 'test-key'", headers.Get("x-goog-api-key"))
	}

	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", headers.Get("Content-Type"))
	}
}

func TestBuildHeadersMultipleCustomHeaders(t *testing.T) {
	p := New("test-key",
		WithHeader("X-First", "first"),
		WithHeader("X-Second", "second"),
	)
	headers := p.buildHeaders()

	if headers.Get("X-First") != "first" {
		t.Errorf("X-First = %q, want 'first'", headers.Get("X-First"))
	}

	if headers.Get("X-Second") != "second" {
		t.Errorf("X-Second = %q, want 'second'", headers.Get("X-Second"))
	}
}

func TestGetModelInfo(t *testing.T) {
	tests := []struct {
		id      core.ModelID
		wantNil bool
		wantID  core.ModelID
	}{
		{ModelGemini3Pro, false, ModelGemini3Pro},
		{ModelGemini3Flash, false, ModelGemini3Flash},
		{ModelGemini25Flash, false, ModelGemini25Flash},
		{ModelGemini25FlashLite, false, ModelGemini25FlashLite},
		{ModelGemini25Pro, false, ModelGemini25Pro},
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
	allModelIDs := []core.ModelID{
		ModelGemini3Pro,
		ModelGemini3Flash,
		ModelGemini25Flash,
		ModelGemini25FlashLite,
		ModelGemini25Pro,
	}

	for _, id := range allModelIDs {
		t.Run(string(id), func(t *testing.T) {
			info := GetModelInfo(id)
			if info == nil {
				t.Fatalf("GetModelInfo(%q) = nil", id)
			}

			// All Gemini models should have these capabilities
			expected := []core.Feature{
				core.FeatureChat,
				core.FeatureChatStreaming,
				core.FeatureToolCalling,
				core.FeatureReasoning,
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
	var _ core.Provider = (*Gemini)(nil)
	var _ core.EmbeddingProvider = (*Gemini)(nil)
}

func TestEmbeddingModelCapabilities(t *testing.T) {
	info := GetModelInfo(ModelGeminiEmbedding001)
	if info == nil {
		t.Fatal("GetModelInfo(gemini-embedding-001) = nil")
	}
	if !info.HasCapability(core.FeatureEmbeddings) {
		t.Error("gemini-embedding-001 missing FeatureEmbeddings")
	}
	if info.HasCapability(core.FeatureChat) {
		t.Error("gemini-embedding-001 should not declare FeatureChat")
	}
}

func TestNewWithEmptyAPIKey(t *testing.T) {
	p := New("")

	if !p.config.APIKey.IsEmpty() {
		t.Errorf("APIKey should be empty")
	}

	// Provider should still be created, validation happens at request time
	if p.ID() != "gemini" {
		t.Errorf("ID() = %q, want 'gemini'", p.ID())
	}
}

func TestConfigImmutability(t *testing.T) {
	p := New("test-key", WithBaseURL("https://example.com"))

	// Attempt to get models and modify (shouldn't affect internal state)
	_ = p.Models()

	// Verify config wasn't affected
	if p.config.BaseURL != "https://example.com" {
		t.Errorf("Config was unexpectedly modified")
	}
}

func TestIsGemini3Model(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{string(ModelGemini3Pro), true},
		{string(ModelGemini3Flash), true},
		{string(ModelGemini25Flash), false},
		{string(ModelGemini25FlashLite), false},
		{string(ModelGemini25Pro), false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := isGemini3Model(tt.model)
			if got != tt.want {
				t.Errorf("isGemini3Model(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestSupportsContentPart(t *testing.T) {
	provider := New("test-key")
	tests := []struct {
		name string
		role core.Role
		part core.ContentPart
		want bool
	}{
		{name: "user text", role: core.RoleUser, part: core.InputText{Text: "describe"}, want: true},
		{name: "assistant image", role: core.RoleAssistant, part: core.InputImage{ImageURL: "https://example.com/cat.jpg"}, want: true},
		{name: "user file", role: core.RoleUser, part: &core.InputFile{FileID: "files/123"}, want: true},
		{name: "system image", role: core.RoleSystem, part: &core.InputImage{ImageURL: "https://example.com/cat.jpg"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := provider.SupportsContentPart(ModelGemini36Flash, tt.role, tt.part); got != tt.want {
				t.Errorf("SupportsContentPart() = %v, want %v", got, tt.want)
			}
		})
	}
}
