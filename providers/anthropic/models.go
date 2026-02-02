// Package anthropic provides an Anthropic API provider implementation for Iris.
package anthropic

import "github.com/petal-labs/iris/core"

// Model constants for Anthropic Claude models.
const (
	// Claude 4.5 series
	ModelClaudeSonnet45 core.ModelID = "claude-sonnet-4-5"
	ModelClaudeHaiku45  core.ModelID = "claude-haiku-4-5"
	ModelClaudeOpus45   core.ModelID = "claude-opus-4-5"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	{
		ID:          ModelClaudeSonnet45,
		DisplayName: "Claude Sonnet 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelClaudeHaiku45,
		DisplayName: "Claude Haiku 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelClaudeOpus45,
		DisplayName: "Claude Opus 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
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
