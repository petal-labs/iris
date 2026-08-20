// Package anthropic provides an Anthropic API provider implementation for Iris.
package anthropic

import "github.com/petal-labs/iris/core"

// Model constants for Anthropic Claude models.
const (
	// Claude 5 series (latest)
	ModelClaudeOpus5           core.ModelID = "claude-opus-5"
	ModelClaudeOpus5Thinking   core.ModelID = "claude-opus-5-thinking"
	ModelClaudeSonnet5         core.ModelID = "claude-sonnet-5"
	ModelClaudeSonnet5Thinking core.ModelID = "claude-sonnet-5-thinking"
	ModelClaudeFable5          core.ModelID = "claude-fable-5"

	// Claude 4.8 series
	ModelClaudeOpus48         core.ModelID = "claude-opus-4-8"
	ModelClaudeOpus48Thinking core.ModelID = "claude-opus-4-8-thinking"

	// Claude 4.7 series
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
	// Claude 5 series (latest)
	{
		ID:          ModelClaudeOpus5,
		DisplayName: "Claude Opus 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeOpus5Thinking,
		DisplayName: "Claude Opus 5 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeSonnet5,
		DisplayName: "Claude Sonnet 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeSonnet5Thinking,
		DisplayName: "Claude Sonnet 5 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeFable5,
		DisplayName: "Claude Fable 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Claude 4.8 series
	{
		ID:          ModelClaudeOpus48,
		DisplayName: "Claude Opus 4.8",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelClaudeOpus48Thinking,
		DisplayName: "Claude Opus 4.8 (Thinking)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Claude 4.7 series
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
