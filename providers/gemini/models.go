// Package gemini provides a Google Gemini API provider implementation for Iris.
package gemini

import "github.com/petal-labs/iris/core"

// Model constants for Google Gemini models.
const (
	// Gemini 3 series (preview)
	ModelGemini3Pro   core.ModelID = "gemini-3-pro-preview"
	ModelGemini3Flash core.ModelID = "gemini-3-flash-preview"

	// Gemini 2.5 series
	ModelGemini25Flash     core.ModelID = "gemini-2.5-flash"
	ModelGemini25FlashLite core.ModelID = "gemini-2.5-flash-lite"
	ModelGemini25Pro       core.ModelID = "gemini-2.5-pro"

	// Image generation models (Nano Banana)
	ModelGemini25FlashImage core.ModelID = "gemini-2.5-flash-image"     // Nano Banana - fast/efficient
	ModelGemini3ProImage    core.ModelID = "gemini-3-pro-image-preview" // Nano Banana Pro - professional with reasoning
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	{
		ID:          ModelGemini3Pro,
		DisplayName: "Gemini 3 Pro Preview",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini3Flash,
		DisplayName: "Gemini 3 Flash Preview",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini25Flash,
		DisplayName: "Gemini 2.5 Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini25FlashLite,
		DisplayName: "Gemini 2.5 Flash Lite",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini25Pro,
		DisplayName: "Gemini 2.5 Pro",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Image generation models (Nano Banana)
	{
		ID:          ModelGemini25FlashImage,
		DisplayName: "Gemini 2.5 Flash Image (Nano Banana)",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelGemini3ProImage,
		DisplayName: "Gemini 3 Pro Image Preview (Nano Banana Pro)",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
}

// modelRegistry is a map for quick model lookup by ID.
var modelRegistry = buildModelRegistry()

// buildModelRegistry creates a map from model ID to ModelInfo.
func buildModelRegistry() map[core.ModelID]*core.ModelInfo {
	registry := make(map[core.ModelID]*core.ModelInfo, len(models))
	for i := range models {
		registry[models[i].ID] = &models[i]
	}
	return registry
}

// GetModelInfo returns the ModelInfo for a given model ID, or nil if not found.
func GetModelInfo(id core.ModelID) *core.ModelInfo {
	return modelRegistry[id]
}

// isGemini3Model returns true if the model is a Gemini 3 series model.
func isGemini3Model(model string) bool {
	return model == string(ModelGemini3Pro) || model == string(ModelGemini3Flash)
}
