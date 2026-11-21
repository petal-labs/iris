// providers/openai/models_test.go
package openai

import (
	"testing"

	"github.com/petal-labs/iris/v2/core"
)

func TestImageModels(t *testing.T) {
	imageModels := []core.ModelID{
		ModelGPTImage2,
	}

	for _, id := range imageModels {
		info := GetModelInfo(id)
		if info == nil {
			t.Errorf("GetModelInfo(%s) = nil, want non-nil", id)
			continue
		}
		if !info.HasCapability(core.FeatureImageGeneration) {
			t.Errorf("Model %s missing FeatureImageGeneration", id)
		}
	}
}

func TestNewGPT6Models(t *testing.T) {
	newModels := []core.ModelID{
		ModelGPT62Astra,
		ModelGPT62Sol,
		ModelGPT61Terra,
	}

	for _, id := range newModels {
		info := GetModelInfo(id)
		if info == nil {
			t.Errorf("GetModelInfo(%s) = nil, want non-nil", id)
			continue
		}
		if info.GetAPIEndpoint() != core.APIEndpointResponses {
			t.Errorf("Model %s APIEndpoint = %q, want %q", id, info.GetAPIEndpoint(), core.APIEndpointResponses)
		}
		if !info.HasCapability(core.FeatureReasoning) {
			t.Errorf("Model %s missing FeatureReasoning", id)
		}
	}
}
