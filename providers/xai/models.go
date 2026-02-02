// Package xai provides an xAI Grok API provider implementation for Iris.
package xai

import "github.com/petal-labs/iris/core"

// Model constants for xAI Grok models.
const (
	// Grok 3 series
	ModelGrok3     core.ModelID = "grok-3"
	ModelGrok3Mini core.ModelID = "grok-3-mini"

	// Grok 4 series
	ModelGrok4                 core.ModelID = "grok-4"
	ModelGrok4FastNonReasoning core.ModelID = "grok-4-fast-non-reasoning"
	ModelGrok4FastReasoning    core.ModelID = "grok-4-fast-reasoning"

	// Grok Code
	ModelGrokCodeFast core.ModelID = "grok-code-fast"

	// Grok 4.1 series
	ModelGrok41FastNonReasoning core.ModelID = "grok-4-1-fast-non-reasoning"
	ModelGrok41FastReasoning    core.ModelID = "grok-4-1-fast-reasoning"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// Grok 3 series
	{
		ID:          ModelGrok3,
		DisplayName: "Grok 3",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGrok3Mini,
		DisplayName: "Grok 3 Mini",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Grok 4 series
	{
		ID:          ModelGrok4,
		DisplayName: "Grok 4",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGrok4FastNonReasoning,
		DisplayName: "Grok 4 Fast (Non-Reasoning)",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGrok4FastReasoning,
		DisplayName: "Grok 4 Fast (Reasoning)",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Grok Code
	{
		ID:          ModelGrokCodeFast,
		DisplayName: "Grok Code Fast",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// Grok 4.1 series
	{
		ID:          ModelGrok41FastNonReasoning,
		DisplayName: "Grok 4.1 Fast (Non-Reasoning)",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGrok41FastReasoning,
		DisplayName: "Grok 4.1 Fast (Reasoning)",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
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
