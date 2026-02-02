package openai

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestOpenAIImplementsProvider(t *testing.T) {
	p := New("test-key")
	var _ core.Provider = p
}

func TestID(t *testing.T) {
	p := New("test-key")

	if p.ID() != "openai" {
		t.Errorf("ID() = %q, want %q", p.ID(), "openai")
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

	if !modelIDs[ModelGPT4o] {
		t.Error("Models() missing gpt-4o")
	}

	if !modelIDs[ModelGPT4oMini] {
		t.Error("Models() missing gpt-4o-mini")
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

		// All chat models should support chat (image-only models are exempt)
		hasChat := false
		hasImageGen := false
		for _, cap := range m.Capabilities {
			if cap == core.FeatureChat {
				hasChat = true
			}
			if cap == core.FeatureImageGeneration {
				hasImageGen = true
			}
		}
		// Only require FeatureChat if it's not an image-only model
		if !hasChat && !hasImageGen {
			t.Errorf("Model %s missing FeatureChat capability", m.ID)
		}
	}
}

func TestSupportsChat(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureChat) {
		t.Error("Supports(FeatureChat) = false, want true")
	}
}

func TestSupportsChatStreaming(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureChatStreaming) {
		t.Error("Supports(FeatureChatStreaming) = false, want true")
	}
}

func TestSupportsToolCalling(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureToolCalling) {
		t.Error("Supports(FeatureToolCalling) = false, want true")
	}
}

func TestSupportsUnknownFeature(t *testing.T) {
	p := New("test-key")

	if p.Supports(core.Feature("unknown_feature")) {
		t.Error("Supports(unknown) = true, want false")
	}
}

func TestSupportsImageGeneration(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureImageGeneration) {
		t.Error("OpenAI should support FeatureImageGeneration")
	}
}

func TestSupportsEmbeddings(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureEmbeddings) {
		t.Error("Expected OpenAI to support embeddings")
	}
}

func TestImplementsImageGenerator(t *testing.T) {
	var _ core.ImageGenerator = (*OpenAI)(nil)
}

func TestBuildHeadersAuth(t *testing.T) {
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
}

func TestBuildHeadersWithOrgID(t *testing.T) {
	p := New("test-key", WithOrgID("org-abc123"))
	headers := p.buildHeaders()

	org := headers.Get("OpenAI-Organization")
	if org != "org-abc123" {
		t.Errorf("OpenAI-Organization = %q, want %q", org, "org-abc123")
	}
}

func TestBuildHeadersWithProjectID(t *testing.T) {
	p := New("test-key", WithProjectID("proj-xyz789"))
	headers := p.buildHeaders()

	project := headers.Get("OpenAI-Project")
	if project != "proj-xyz789" {
		t.Errorf("OpenAI-Project = %q, want %q", project, "proj-xyz789")
	}
}

func TestBuildHeadersWithoutOptionals(t *testing.T) {
	p := New("test-key")
	headers := p.buildHeaders()

	if headers.Get("OpenAI-Organization") != "" {
		t.Error("OpenAI-Organization should be empty when not configured")
	}

	if headers.Get("OpenAI-Project") != "" {
		t.Error("OpenAI-Project should be empty when not configured")
	}
}

func TestBuildHeadersWithCustomHeaders(t *testing.T) {
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
}

func TestBuildHeadersAllOptions(t *testing.T) {
	p := New("test-key",
		WithOrgID("my-org"),
		WithProjectID("my-project"),
		WithHeader("X-Request-ID", "req-123"),
	)
	headers := p.buildHeaders()

	// Check all headers are present
	checks := map[string]string{
		"Authorization":       "Bearer test-key",
		"Content-Type":        "application/json",
		"OpenAI-Organization": "my-org",
		"OpenAI-Project":      "my-project",
		"X-Request-ID":        "req-123",
	}

	for key, want := range checks {
		got := headers.Get(key)
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
