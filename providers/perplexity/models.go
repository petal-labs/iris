// Package perplexity provides a Perplexity Search API provider implementation for Iris.
package perplexity

import "github.com/petal-labs/iris/core"

// Model constants for Perplexity models.
const (
	// Search Models
	ModelSonar    core.ModelID = "sonar"     // Lightweight, cost-effective search with grounding
	ModelSonarPro core.ModelID = "sonar-pro" // Advanced search with Pro Search support

	// Reasoning Models
	ModelSonarReasoningPro core.ModelID = "sonar-reasoning-pro" // Chain of Thought reasoning with search

	// Research Models
	ModelSonarDeepResearch core.ModelID = "sonar-deep-research" // Comprehensive research and report generation
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// Search Models
	{
		ID:          ModelSonar,
		DisplayName: "Sonar",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelSonarPro,
		DisplayName: "Sonar Pro",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// Reasoning Models
	{
		ID:          ModelSonarReasoningPro,
		DisplayName: "Sonar Reasoning Pro",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Research Models
	{
		ID:          ModelSonarDeepResearch,
		DisplayName: "Sonar Deep Research",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
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
