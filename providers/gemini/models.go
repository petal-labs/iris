// Package gemini provides a Google Gemini API provider implementation for Iris.
package gemini

import (
	"strings"

	"github.com/petal-labs/iris/core"
)

// Model constants for Google Gemini models.
const (
	// Gemini 4 series (latest)
	ModelGemini4Argon core.ModelID = "gemini-4-argon"

	// Gemini 3.8 series
	ModelGemini38Flash core.ModelID = "gemini-3.8-flash"

	// Gemini 3.7 series
	ModelGemini37Flash core.ModelID = "gemini-3.7-flash"

	// Gemini 3.6 series
	ModelGemini36Flash core.ModelID = "gemini-3.6-flash"

	// Gemini 3.5 series
	ModelGemini35Flash     core.ModelID = "gemini-3.5-flash"
	ModelGemini35FlashLite core.ModelID = "gemini-3.5-flash-lite"

	// Gemini 3.1 series (preview)
	ModelGemini31Pro               core.ModelID = "gemini-3.1-pro-preview"
	ModelGemini31FlashLite         core.ModelID = "gemini-3.1-flash-lite"
	ModelGemini31FlashLiteImage    core.ModelID = "gemini-3.1-flash-lite-image"
	ModelGemini31FlashImagePreview core.ModelID = "gemini-3.1-flash-image-preview"
	ModelGemini31FlashImage        core.ModelID = "gemini-3.1-flash-image"

	// Gemini 3 series (preview)
	ModelGemini3Flash core.ModelID = "gemini-3-flash-preview"
	// ModelGemini3ProImage retains the preview ID it shipped with so the
	// exported constant's value stays stable; use ModelGemini3ProImageStable
	// for the generally available release.
	ModelGemini3ProImage       core.ModelID = "gemini-3-pro-image-preview"
	ModelGemini3ProImageStable core.ModelID = "gemini-3-pro-image"

	// Latest aliases (point at the most recent stable snapshot)
	ModelGeminiFlashLatest     core.ModelID = "gemini-flash-latest"
	ModelGeminiFlashLiteLatest core.ModelID = "gemini-flash-lite-latest"
	ModelGeminiProLatest       core.ModelID = "gemini-pro-latest"

	// Gemini 2.5 series
	ModelGemini25Flash     core.ModelID = "gemini-2.5-flash"
	ModelGemini25FlashLite core.ModelID = "gemini-2.5-flash-lite"
	ModelGemini25Pro       core.ModelID = "gemini-2.5-pro"

	// Image generation models (Nano Banana)
	ModelGemini25FlashImage core.ModelID = "gemini-2.5-flash-image"

	// Embedding models
	ModelGeminiEmbedding2   core.ModelID = "gemini-embedding-2"
	ModelGeminiEmbedding001 core.ModelID = "gemini-embedding-001"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// Gemini 4 series (latest)
	{
		ID:          ModelGemini4Argon,
		DisplayName: "Gemini 4 Argon",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Gemini 3.8 series
	{
		ID:          ModelGemini38Flash,
		DisplayName: "Gemini 3.8 Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Gemini 3.7 series
	{
		ID:          ModelGemini37Flash,
		DisplayName: "Gemini 3.7 Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Gemini 3.6 series
	{
		ID:          ModelGemini36Flash,
		DisplayName: "Gemini 3.6 Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Gemini 3.5 series (preview)
	{
		ID:          ModelGemini35Flash,
		DisplayName: "Gemini 3.5 Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini35FlashLite,
		DisplayName: "Gemini 3.5 Flash Lite",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Gemini 3.1 series (preview)
	{
		ID:          ModelGemini31Pro,
		DisplayName: "Gemini 3.1 Pro Preview",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini31FlashLite,
		DisplayName: "Gemini 3.1 Flash Lite",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGemini31FlashLiteImage,
		DisplayName: "Gemini 3.1 Flash Lite Image (Nano Banana 2 Lite)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelGemini31FlashImagePreview,
		DisplayName: "Gemini 3.1 Flash Image Preview",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelGemini31FlashImage,
		DisplayName: "Gemini 3.1 Flash Image (Nano Banana 2)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureImageGeneration,
		},
	},
	// Gemini 3 series (preview)
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
		ID:          ModelGemini3ProImage,
		DisplayName: "Gemini 3 Pro Image Preview (Nano Banana Pro)",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelGemini3ProImageStable,
		DisplayName: "Gemini 3 Pro Image (Nano Banana Pro)",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	// Latest aliases
	{
		ID:          ModelGeminiFlashLatest,
		DisplayName: "Gemini Flash Latest",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGeminiFlashLiteLatest,
		DisplayName: "Gemini Flash-Lite Latest",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGeminiProLatest,
		DisplayName: "Gemini Pro Latest",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Gemini 2.5 series
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
		ID:          ModelGeminiEmbedding2,
		DisplayName: "Gemini Embedding 2",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          ModelGeminiEmbedding001,
		DisplayName: "Gemini Embedding 001",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
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

// GetModelInfo returns a copy of the ModelInfo for a given model ID, or nil
// if not found. The returned pointer references a fresh copy whose fields
// (including the Capabilities slice) do not alias the package's static
// catalog, so mutating it cannot affect subsequent GetModelInfo or Models calls.
func GetModelInfo(id core.ModelID) *core.ModelInfo {
	m, ok := modelRegistry[id]
	if !ok {
		return nil
	}
	cp := *m
	if caps := cp.Capabilities; caps != nil {
		cp.Capabilities = make([]core.Feature, len(caps))
		copy(cp.Capabilities, caps)
	}
	return &cp
}

// usesThinkingLevel returns true if the model uses the thinkingLevel
// reasoning control. This covers the Gemini 3 families (3, 3.1, 3.5, 3.6,
// 3.7, and 3.8) and Gemini 4, all of which use thinkingLevel rather than
// Gemini 2.5's thinkingBudget.
func usesThinkingLevel(model string) bool {
	return strings.HasPrefix(model, "gemini-3") || strings.HasPrefix(model, "gemini-4")
}
