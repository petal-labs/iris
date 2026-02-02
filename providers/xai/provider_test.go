package xai

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestXaiImplementsProvider(t *testing.T) {
	p := New("test-key")
	var _ core.Provider = p
}

func TestID(t *testing.T) {
	p := New("test-key")

	if p.ID() != "xai" {
		t.Errorf("ID() = %q, want %q", p.ID(), "xai")
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

	if !modelIDs[ModelGrok4] {
		t.Error("Models() missing grok-4")
	}

	if !modelIDs[ModelGrok41FastNonReasoning] {
		t.Error("Models() missing grok-4-1-fast-non-reasoning")
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

		// All xAI models should support chat
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

func TestSupportsReasoning(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureReasoning) {
		t.Error("Supports(FeatureReasoning) = false, want true")
	}
}

func TestSupportsUnknownFeature(t *testing.T) {
	p := New("test-key")

	if p.Supports(core.Feature("unknown_feature")) {
		t.Error("Supports(unknown) = true, want false")
	}
}

func TestBuildHeadersAuth(t *testing.T) {
	p := New("xai-test-key-123")
	headers := p.buildHeaders()

	auth := headers.Get("Authorization")
	if auth != "Bearer xai-test-key-123" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer xai-test-key-123")
	}

	contentType := headers.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
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

func TestGetModelInfo(t *testing.T) {
	info := GetModelInfo(ModelGrok4)
	if info == nil {
		t.Fatal("GetModelInfo(ModelGrok4) returned nil")
	}
	if info.ID != ModelGrok4 {
		t.Errorf("ID = %q, want %q", info.ID, ModelGrok4)
	}
}

func TestGetModelInfoUnknown(t *testing.T) {
	info := GetModelInfo("unknown-model")
	if info != nil {
		t.Errorf("GetModelInfo(unknown) = %v, want nil", info)
	}
}
