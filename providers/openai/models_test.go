// providers/openai/models_test.go
package openai

import (
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestImageModels(t *testing.T) {
	imageModels := []core.ModelID{
		ModelGPTImage15,
		ModelGPTImage1,
		ModelGPTImage1Mini,
		ModelDALLE3,
		ModelDALLE2,
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

func TestChatGPTImageLatestModel(t *testing.T) {
	info := GetModelInfo(ModelChatGPTImageLatest)
	if info == nil {
		t.Fatal("GetModelInfo(ModelChatGPTImageLatest) = nil")
	}
	if !info.HasCapability(core.FeatureImageGeneration) {
		t.Error("chatgpt-image-latest missing FeatureImageGeneration")
	}
}
