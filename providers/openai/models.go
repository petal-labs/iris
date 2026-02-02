// Package openai provides an OpenAI API provider implementation for Iris.
package openai

import "github.com/petal-labs/iris/core"

// Model constants for OpenAI models.
const (
	// GPT-5.2 series
	ModelGPT52      core.ModelID = "gpt-5.2"
	ModelGPT52Pro   core.ModelID = "gpt-5.2-pro"
	ModelGPT52Codex core.ModelID = "gpt-5.2-codex"

	// GPT-5.1 series
	ModelGPT51          core.ModelID = "gpt-5.1"
	ModelGPT51Codex     core.ModelID = "gpt-5.1-codex"
	ModelGPT51CodexMini core.ModelID = "gpt-5.1-codex-mini"
	ModelGPT51CodexMax  core.ModelID = "gpt-5.1-codex-max"

	// GPT-5 series
	ModelGPT5      core.ModelID = "gpt-5"
	ModelGPT5Mini  core.ModelID = "gpt-5-mini"
	ModelGPT5Nano  core.ModelID = "gpt-5-nano"
	ModelGPT5Pro   core.ModelID = "gpt-5-pro"
	ModelGPT5Codex core.ModelID = "gpt-5-codex"

	// GPT-4.1 series
	ModelGPT41     core.ModelID = "gpt-4.1"
	ModelGPT41Mini core.ModelID = "gpt-4.1-mini"
	ModelGPT41Nano core.ModelID = "gpt-4.1-nano"

	// GPT-4o series
	ModelGPT4o     core.ModelID = "gpt-4o"
	ModelGPT4oMini core.ModelID = "gpt-4o-mini"

	// GPT-4 series
	ModelGPT4Turbo core.ModelID = "gpt-4-turbo"
	ModelGPT4      core.ModelID = "gpt-4"

	// GPT-3.5 series
	ModelGPT35Turbo         core.ModelID = "gpt-3.5-turbo"
	ModelGPT35Turbo16k      core.ModelID = "gpt-3.5-turbo-16k"
	ModelGPT35TurboInstruct core.ModelID = "gpt-3.5-turbo-instruct"

	// Reasoning models (o-series)
	ModelO4Mini             core.ModelID = "o4-mini"
	ModelO4MiniDeepResearch core.ModelID = "o4-mini-deep-research"
	ModelO3                 core.ModelID = "o3"
	ModelO3Mini             core.ModelID = "o3-mini"
	ModelO1                 core.ModelID = "o1"
	ModelO1Pro              core.ModelID = "o1-pro"

	// Image generation models
	ModelGPTImage15         core.ModelID = "gpt-image-1.5"
	ModelGPTImage1          core.ModelID = "gpt-image-1"
	ModelGPTImage1Mini      core.ModelID = "gpt-image-1-mini"
	ModelDALLE3             core.ModelID = "dall-e-3"
	ModelDALLE2             core.ModelID = "dall-e-2"
	ModelChatGPTImageLatest core.ModelID = "chatgpt-image-latest"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// GPT-5.2 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT52,
		DisplayName: "GPT-5.2",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT52Pro,
		DisplayName: "GPT-5.2 Pro",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT52Codex,
		DisplayName: "GPT-5.2 Codex",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5.1 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT51,
		DisplayName: "GPT-5.1",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT51Codex,
		DisplayName: "GPT-5.1 Codex",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT51CodexMini,
		DisplayName: "GPT-5.1 Codex Mini",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT51CodexMax,
		DisplayName: "GPT-5.1 Codex Max",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT5,
		DisplayName: "GPT-5",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT5Mini,
		DisplayName: "GPT-5 Mini",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT5Nano,
		DisplayName: "GPT-5 Nano",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT5Pro,
		DisplayName: "GPT-5 Pro",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT5Codex,
		DisplayName: "GPT-5 Codex",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-4.1 series (Responses API without reasoning)
	{
		ID:          ModelGPT41,
		DisplayName: "GPT-4.1",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT41Mini,
		DisplayName: "GPT-4.1 Mini",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT41Nano,
		DisplayName: "GPT-4.1 Nano",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-4o series (Chat Completions API)
	{
		ID:          ModelGPT4o,
		DisplayName: "GPT-4o",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGPT4oMini,
		DisplayName: "GPT-4o Mini",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// GPT-4 series (Chat Completions API)
	{
		ID:          ModelGPT4Turbo,
		DisplayName: "GPT-4 Turbo",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGPT4,
		DisplayName: "GPT-4",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// GPT-3.5 series (Chat Completions API)
	{
		ID:          ModelGPT35Turbo,
		DisplayName: "GPT-3.5 Turbo",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGPT35Turbo16k,
		DisplayName: "GPT-3.5 Turbo 16k",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGPT35TurboInstruct,
		DisplayName: "GPT-3.5 Turbo Instruct",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	// Reasoning models (o-series) - Responses API with reasoning
	{
		ID:          ModelO4Mini,
		DisplayName: "o4-mini",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelO4MiniDeepResearch,
		DisplayName: "o4-mini Deep Research",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelO3,
		DisplayName: "o3",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelO3Mini,
		DisplayName: "o3-mini",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelO1,
		DisplayName: "o1",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelO1Pro,
		DisplayName: "o1 Pro",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureResponseChain,
		},
	},
	// Image generation models
	{
		ID:          ModelGPTImage15,
		DisplayName: "GPT Image 1.5",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelGPTImage1,
		DisplayName: "GPT Image 1",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelGPTImage1Mini,
		DisplayName: "GPT Image 1 Mini",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelDALLE3,
		DisplayName: "DALL-E 3",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelDALLE2,
		DisplayName: "DALL-E 2",
		Capabilities: []core.Feature{
			core.FeatureImageGeneration,
		},
	},
	{
		ID:          ModelChatGPTImageLatest,
		DisplayName: "ChatGPT Image Latest",
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
