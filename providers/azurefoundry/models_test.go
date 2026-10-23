package azurefoundry

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestGetModelInfo(t *testing.T) {
	info := GetModelInfo("gpt-5.6-terra")
	if info == nil {
		t.Fatal("GetModelInfo(gpt-5.6-terra) returned nil")
	}
	if info.ID != "gpt-5.6-terra" {
		t.Errorf("ID = %q, want gpt-5.6-terra", info.ID)
	}
	if info.DisplayName != "GPT-5.6 Terra" {
		t.Errorf("DisplayName = %q, want GPT-5.6 Terra", info.DisplayName)
	}
}

func TestGetModelInfoUnknown(t *testing.T) {
	info := GetModelInfo("unknown-model")
	if info != nil {
		t.Errorf("GetModelInfo(unknown) = %v, want nil", info)
	}
}

func TestListModels(t *testing.T) {
	models := ListModels()

	if len(models) < 40 {
		t.Errorf("ListModels() returned %d models, want at least 40", len(models))
	}

	// Verify it returns a copy
	original := ListModels()
	models[0].DisplayName = "modified"
	if original[0].DisplayName == "modified" {
		t.Error("ListModels() did not return a copy")
	}
}

func TestListChatModels(t *testing.T) {
	models := ListChatModels()

	if len(models) == 0 {
		t.Error("ListChatModels() returned empty list")
	}

	for _, m := range models {
		if !m.HasCapability(core.FeatureChat) {
			t.Errorf("Model %s in ListChatModels() does not have FeatureChat", m.ID)
		}
	}
}

func TestListStreamingModels(t *testing.T) {
	models := ListStreamingModels()

	if len(models) == 0 {
		t.Error("ListStreamingModels() returned empty list")
	}

	for _, m := range models {
		if !m.HasCapability(core.FeatureChatStreaming) {
			t.Errorf("Model %s in ListStreamingModels() does not have FeatureChatStreaming", m.ID)
		}
	}
}

func TestListEmbeddingModels(t *testing.T) {
	models := ListEmbeddingModels()

	if len(models) < 5 {
		t.Errorf("ListEmbeddingModels() returned %d models, want at least 5", len(models))
	}

	for _, m := range models {
		if !m.HasCapability(core.FeatureEmbeddings) {
			t.Errorf("Model %s in ListEmbeddingModels() does not have FeatureEmbeddings", m.ID)
		}
	}

	// Check for known embedding models
	found := make(map[core.ModelID]bool)
	for _, m := range models {
		found[m.ID] = true
	}

	if !found["text-embedding-3-large"] {
		t.Error("ListEmbeddingModels() missing text-embedding-3-large")
	}
	if !found["text-embedding-3-small"] {
		t.Error("ListEmbeddingModels() missing text-embedding-3-small")
	}
	if !found["cohere-embed-v3-english"] {
		t.Error("ListEmbeddingModels() missing cohere-embed-v3-english")
	}
}

func TestListToolCallingModels(t *testing.T) {
	models := ListToolCallingModels()

	if len(models) < 10 {
		t.Errorf("ListToolCallingModels() returned %d models, want at least 10", len(models))
	}

	for _, m := range models {
		if !m.HasCapability(core.FeatureToolCalling) {
			t.Errorf("Model %s in ListToolCallingModels() does not have FeatureToolCalling", m.ID)
		}
	}
}

func TestListReasoningModels(t *testing.T) {
	models := ListReasoningModels()

	if len(models) < 4 {
		t.Errorf("ListReasoningModels() returned %d models, want at least 4", len(models))
	}

	for _, m := range models {
		if !m.HasCapability(core.FeatureReasoning) {
			t.Errorf("Model %s in ListReasoningModels() does not have FeatureReasoning", m.ID)
		}
	}

	// Check for known reasoning models
	found := make(map[core.ModelID]bool)
	for _, m := range models {
		found[m.ID] = true
	}

	if !found["o3"] {
		t.Error("ListReasoningModels() missing o3")
	}
	if !found["deepseek-v4-pro"] {
		t.Error("ListReasoningModels() missing deepseek-v4-pro")
	}
}

func TestModelsByCapability(t *testing.T) {
	models := ModelsByCapability(core.FeatureStructuredOutput)

	if len(models) < 3 {
		t.Errorf("ModelsByCapability(StructuredOutput) returned %d models, want at least 3", len(models))
	}

	for _, m := range models {
		if !m.HasCapability(core.FeatureStructuredOutput) {
			t.Errorf("Model %s returned by ModelsByCapability does not have requested capability", m.ID)
		}
	}
}

func TestSupportsCapability(t *testing.T) {
	tests := []struct {
		model      core.ModelID
		capability core.Feature
		want       bool
	}{
		{"gpt-5.6-terra", core.FeatureChat, true},
		{"gpt-5.6-terra", core.FeatureToolCalling, true},
		{"gpt-5.6-terra", core.FeatureStructuredOutput, true},
		{"gpt-5.6-terra", core.FeatureEmbeddings, false},
		{"text-embedding-3-large", core.FeatureEmbeddings, true},
		{"text-embedding-3-large", core.FeatureChat, false},
		{"o3", core.FeatureReasoning, true},
		{"phi-4", core.FeatureToolCalling, false},
		{"unknown-model", core.FeatureChat, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.model)+"_"+string(tt.capability), func(t *testing.T) {
			got := SupportsCapability(tt.model, tt.capability)
			if got != tt.want {
				t.Errorf("SupportsCapability(%q, %q) = %v, want %v", tt.model, tt.capability, got, tt.want)
			}
		})
	}
}

func TestIsEmbeddingModel(t *testing.T) {
	tests := []struct {
		model core.ModelID
		want  bool
	}{
		{"text-embedding-3-large", true},
		{"text-embedding-3-small", true},
		{"text-embedding-ada-002", true},
		{"cohere-embed-v3-english", true},
		{"gpt-5.6-terra", false},
		{"o3", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			got := IsEmbeddingModel(tt.model)
			if got != tt.want {
				t.Errorf("IsEmbeddingModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsChatModel(t *testing.T) {
	tests := []struct {
		model core.ModelID
		want  bool
	}{
		{"gpt-5.6-terra", true},
		{"gpt-5.5", true},
		{"llama-3.3-70b-instruct", true},
		{"mistral-medium-2505", true},
		{"text-embedding-3-large", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			got := IsChatModel(tt.model)
			if got != tt.want {
				t.Errorf("IsChatModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsReasoningModel(t *testing.T) {
	tests := []struct {
		model core.ModelID
		want  bool
	}{
		{"o3", true},
		{"deepseek-v4-pro", true},
		{"phi-4-reasoning", true},
		{"gpt-5.6-terra", true},
		{"text-embedding-3-large", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			got := IsReasoningModel(tt.model)
			if got != tt.want {
				t.Errorf("IsReasoningModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestSupportsToolCalling(t *testing.T) {
	tests := []struct {
		model core.ModelID
		want  bool
	}{
		{"gpt-5.6-terra", true},
		{"gpt-5.5", true},
		{"llama-3.3-70b-instruct", true},
		{"cohere-command-a", true},
		{"kimi-k2.6", true},
		{"deepseek-v4-pro", false},
		{"text-embedding-3-large", false},
		{"phi-4", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			got := SupportsToolCalling(tt.model)
			if got != tt.want {
				t.Errorf("SupportsToolCalling(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestSupportsStructuredOutput(t *testing.T) {
	tests := []struct {
		model core.ModelID
		want  bool
	}{
		{"gpt-5.6-terra", true},
		{"gpt-5.5", true},
		{"kimi-k2.6", true},
		{"o3", false}, // o3 doesn't have structured output
		{"mistral-medium-2505", false},
		{"text-embedding-3-large", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			got := SupportsStructuredOutput(tt.model)
			if got != tt.want {
				t.Errorf("SupportsStructuredOutput(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestAllModelsHaveCapabilities(t *testing.T) {
	models := ListModels()

	for _, m := range models {
		if len(m.Capabilities) == 0 {
			t.Errorf("Model %s has no capabilities", m.ID)
		}
	}
}

func TestAllModelsHaveDisplayName(t *testing.T) {
	models := ListModels()

	for _, m := range models {
		if m.DisplayName == "" {
			t.Errorf("Model %s has empty DisplayName", m.ID)
		}
	}
}

func TestNoDuplicateModelIDs(t *testing.T) {
	models := ListModels()
	seen := make(map[core.ModelID]bool)

	for _, m := range models {
		if seen[m.ID] {
			t.Errorf("Duplicate model ID: %s", m.ID)
		}
		seen[m.ID] = true
	}
}

func TestKnownModelsExist(t *testing.T) {
	knownModels := []core.ModelID{
		// OpenAI
		"gpt-5.6-terra",
		"gpt-5.5",
		"gpt-5",
		"o3",
		// Embeddings
		"text-embedding-3-large",
		"text-embedding-3-small",
		"text-embedding-ada-002",
		// Llama
		"llama-3.3-70b-instruct",
		"llama-4-scout-17b-16e-instruct",
		"llama-4-maverick-17b-128e-instruct-fp8",
		// Mistral
		"mistral-medium-2505",
		"mistral-small-2503",
		// Cohere
		"cohere-command-a",
		"cohere-embed-v3-english",
		// DeepSeek
		"deepseek-v3.2",
		"deepseek-v4-pro",
		// Phi
		"phi-4",
		"phi-4-mini",
	}

	for _, id := range knownModels {
		info := GetModelInfo(id)
		if info == nil {
			t.Errorf("Known model %s not found", id)
		}
	}
}
