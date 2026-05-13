// Package anthropic provides an Anthropic API provider implementation for Iris.
package anthropic

import "github.com/petal-labs/iris/core"

// Model constants for Anthropic Claude models.
const (
	// Claude 4.7 series (latest)
	ModelClaudeOpus47 core.ModelID = "claude-opus-4-7"

	// Claude 4.6 series
	ModelClaudeSonnet46         core.ModelID = "claude-sonnet-4-6"
	ModelClaudeSonnet46Thinking core.ModelID = "claude-sonnet-4-6-thinking"
	ModelClaudeOpus46           core.ModelID = "claude-opus-4-6"
	ModelClaudeOpus46Thinking   core.ModelID = "claude-opus-4-6-thinking"

	// Claude 4.5 series
	ModelClaudeSonnet45         core.ModelID = "claude-sonnet-4-5"
	ModelClaudeSonnet45Thinking core.ModelID = "claude-sonnet-4-5-thinking"
	ModelClaudeHaiku45          core.ModelID = "claude-haiku-4-5"
	ModelClaudeOpus45           core.ModelID = "claude-opus-4-5"
	ModelClaudeOpus45Thinking   core.ModelID = "claude-opus-4-5-thinking"

	// Claude 3.5 series (legacy)
	ModelClaude35HaikuLatest core.ModelID = "claude-3-5-haiku-latest"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// Claude 4.7 series (latest)
	{
		ID:          ModelClaudeOpus47,
		DisplayName: "Claude Opus 4.7",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Claude 4.6 series
	{
		ID:          ModelClaudeSonnet46,
		DisplayName: "Claude Sonnet 4.6",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeSonnet46Thinking,
		DisplayName: "Claude Sonnet 4.6 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeOpus46,
		DisplayName: "Claude Opus 4.6",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeOpus46Thinking,
		DisplayName: "Claude Opus 4.6 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Claude 4.5 series
	{
		ID:          ModelClaudeSonnet45,
		DisplayName: "Claude Sonnet 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeSonnet45Thinking,
		DisplayName: "Claude Sonnet 4.5 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeHaiku45,
		DisplayName: "Claude Haiku 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeOpus45,
		DisplayName: "Claude Opus 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeOpus45Thinking,
		DisplayName: "Claude Opus 4.5 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Claude 3.5 series (legacy)
	{
		ID:          ModelClaude35HaikuLatest,
		DisplayName: "Claude 3.5 Haiku",
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
