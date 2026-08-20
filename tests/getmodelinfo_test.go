package tests

import (
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
	_ "github.com/petal-labs/iris/providers/anthropic"
	_ "github.com/petal-labs/iris/providers/azurefoundry"
	_ "github.com/petal-labs/iris/providers/gemini"
	_ "github.com/petal-labs/iris/providers/ollama"
	_ "github.com/petal-labs/iris/providers/openai"
	_ "github.com/petal-labs/iris/providers/perplexity"
	_ "github.com/petal-labs/iris/providers/voyageai"
	_ "github.com/petal-labs/iris/providers/xai"
	_ "github.com/petal-labs/iris/providers/zai"

	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/azurefoundry"
	"github.com/petal-labs/iris/providers/gemini"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/perplexity"
	"github.com/petal-labs/iris/providers/voyageai"
	"github.com/petal-labs/iris/providers/xai"
	"github.com/petal-labs/iris/providers/zai"
)

// TestGetModelInfoMutationIsolation verifies that GetModelInfo in every
// provider returns an independent copy: mutating the returned pointer must
// not corrupt the package's static catalog, so subsequent GetModelInfo and
// Models calls still see the original values. Regression guard for issue #64.
func TestGetModelInfoMutationIsolation(t *testing.T) {
	tests := []struct {
		name         string
		providerID   string // registry ID, for constructing a provider to call Models()
		getModelInfo func(core.ModelID) *core.ModelInfo
		id           core.ModelID
	}{
		{"openai", "openai", openai.GetModelInfo, openai.ModelGPT56},
		{"anthropic", "anthropic", anthropic.GetModelInfo, anthropic.ModelClaudeOpus5},
		{"gemini", "gemini", gemini.GetModelInfo, gemini.ModelGemini36Flash},
		{"xai", "xai", xai.GetModelInfo, xai.ModelGrok45},
		{"zai", "zai", zai.GetModelInfo, zai.ModelGLM52},
		{"perplexity", "perplexity", perplexity.GetModelInfo, perplexity.ModelSonar},
		{"voyageai", "voyageai", voyageai.GetModelInfo, voyageai.ModelVoyage4Large},
		{"azurefoundry", "azurefoundry", azurefoundry.GetModelInfo, "gpt-4o"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Snapshot the original values from a fresh GetModelInfo call.
			orig := tt.getModelInfo(tt.id)
			if orig == nil {
				t.Fatalf("GetModelInfo(%q) = nil, want non-nil", tt.id)
			}
			origDisplayName := orig.DisplayName
			origID := orig.ID
			origCapabilities := make([]core.Feature, len(orig.Capabilities))
			copy(origCapabilities, orig.Capabilities)

			// Mutate the returned pointer aggressively: scalar fields and the
			// Capabilities slice (both element assignment and append).
			mut := tt.getModelInfo(tt.id)
			if mut == nil {
				t.Fatalf("GetModelInfo(%q) = nil on second call", tt.id)
			}
			mut.DisplayName = "HACKED"
			mut.ID = "HACKED-ID"
			if len(mut.Capabilities) > 0 {
				mut.Capabilities[0] = core.FeatureImageGeneration // overwrite an element
			}
			mut.Capabilities = append(mut.Capabilities, core.FeatureReranking)

			// Subsequent GetModelInfo must return the original, uncorrupted values.
			after := tt.getModelInfo(tt.id)
			if after == nil {
				t.Fatalf("GetModelInfo(%q) = nil on third call", tt.id)
			}
			if after.DisplayName != origDisplayName {
				t.Errorf("DisplayName after mutation = %q, want %q (static catalog corrupted)", after.DisplayName, origDisplayName)
			}
			if after.ID != origID {
				t.Errorf("ID after mutation = %q, want %q (static catalog corrupted)", after.ID, origID)
			}
			if !equalFeatures(after.Capabilities, origCapabilities) {
				t.Errorf("Capabilities after mutation = %v, want %v (static catalog corrupted)", after.Capabilities, origCapabilities)
			}

			// Models() must also return the original, uncorrupted values.
			provider, err := providers.Create(tt.providerID, "test-key")
			if err != nil {
				t.Fatalf("providers.Create(%q) error = %v", tt.providerID, err)
			}
			for _, m := range provider.Models() {
				if m.ID != tt.id {
					continue
				}
				if m.DisplayName != origDisplayName {
					t.Errorf("Models() DisplayName = %q, want %q (static catalog corrupted)", m.DisplayName, origDisplayName)
				}
				if !equalFeatures(m.Capabilities, origCapabilities) {
					t.Errorf("Models() Capabilities = %v, want %v (static catalog corrupted)", m.Capabilities, origCapabilities)
				}
				return
			}
			t.Errorf("Models() did not return model %q", tt.id)
		})
	}
}

// TestGetModelInfoReturnedPointersAreDistinct verifies that repeated calls
// to GetModelInfo return distinct pointers, so callers cannot accidentally
// share mutable state with each other. Regression guard for issue #64.
func TestGetModelInfoReturnedPointersAreDistinct(t *testing.T) {
	tests := []struct {
		name         string
		getModelInfo func(core.ModelID) *core.ModelInfo
		id           core.ModelID
	}{
		{"openai", openai.GetModelInfo, openai.ModelGPT56},
		{"anthropic", anthropic.GetModelInfo, anthropic.ModelClaudeOpus5},
		{"gemini", gemini.GetModelInfo, gemini.ModelGemini36Flash},
		{"xai", xai.GetModelInfo, xai.ModelGrok45},
		{"zai", zai.GetModelInfo, zai.ModelGLM52},
		{"perplexity", perplexity.GetModelInfo, perplexity.ModelSonar},
		{"voyageai", voyageai.GetModelInfo, voyageai.ModelVoyage4Large},
		{"azurefoundry", azurefoundry.GetModelInfo, "gpt-4o"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.getModelInfo(tt.id)
			b := tt.getModelInfo(tt.id)
			if a == nil || b == nil {
				t.Fatalf("GetModelInfo(%q) = nil", tt.id)
			}
			if a == b {
				t.Errorf("GetModelInfo returned the same pointer on repeated calls; callers may share mutable state")
			}
		})
	}
}

func equalFeatures(a, b []core.Feature) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
