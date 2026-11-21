// Package openai provides an OpenAI API provider implementation for Iris.
package openai

import "github.com/petal-labs/iris/core"

// Model constants for OpenAI models.
const (
	// GPT-6.2 series (latest)
	ModelGPT62Astra core.ModelID = "gpt-6.2-astra"
	ModelGPT62Sol   core.ModelID = "gpt-6.2-sol"

	// GPT-6.1 series
	ModelGPT61Sol   core.ModelID = "gpt-6.1-sol"
	ModelGPT61Terra core.ModelID = "gpt-6.1-terra"

	// GPT-6 series
	ModelGPT6Astra core.ModelID = "gpt-6-astra"
	ModelGPT6Luna  core.ModelID = "gpt-6-luna"
	ModelGPT6Sol   core.ModelID = "gpt-6-sol"

	// GPT Daybreak series (cybersecurity-specialized aliases)
	ModelGPTDaybreakBlueLatest core.ModelID = "gpt-daybreak-blue-latest"
	ModelGPTDaybreakRedLatest  core.ModelID = "gpt-daybreak-red-latest"

	// GPT-5.6 series
	ModelGPT56      core.ModelID = "gpt-5.6"
	ModelGPT56Luna  core.ModelID = "gpt-5.6-luna"
	ModelGPT56Sol   core.ModelID = "gpt-5.6-sol"
	ModelGPT56Terra core.ModelID = "gpt-5.6-terra"

	// GPT-5.5 series
	ModelGPT55    core.ModelID = "gpt-5.5"
	ModelGPT55Pro core.ModelID = "gpt-5.5-pro"

	// GPT-5.3 series
	ModelGPT53Codex      core.ModelID = "gpt-5.3-codex"
	ModelGPT53CodexSpark core.ModelID = "gpt-5.3-codex-spark"

	// GPT-5.4 series
	ModelGPT54     core.ModelID = "gpt-5.4"
	ModelGPT54Pro  core.ModelID = "gpt-5.4-pro"
	ModelGPT54Mini core.ModelID = "gpt-5.4-mini"
	ModelGPT54Nano core.ModelID = "gpt-5.4-nano"

	// GPT-5.2 series
	ModelGPT52    core.ModelID = "gpt-5.2"
	ModelGPT52Pro core.ModelID = "gpt-5.2-pro"

	// GPT-5.1 series
	ModelGPT51 core.ModelID = "gpt-5.1"

	// GPT-5 series
	ModelGPT5     core.ModelID = "gpt-5"
	ModelGPT5Mini core.ModelID = "gpt-5-mini"
	ModelGPT5Nano core.ModelID = "gpt-5-nano"
	ModelGPT5Pro  core.ModelID = "gpt-5-pro"

	// GPT-4.1 series
	ModelGPT41     core.ModelID = "gpt-4.1"
	ModelGPT41Mini core.ModelID = "gpt-4.1-mini"

	// GPT-4o series
	ModelGPT4o     core.ModelID = "gpt-4o"
	ModelGPT4oMini core.ModelID = "gpt-4o-mini"

	// Reasoning models (o-series)
	ModelO3    core.ModelID = "o3"
	ModelO3Pro core.ModelID = "o3-pro"

	// Image generation models
	ModelGPTImage2 core.ModelID = "gpt-image-2"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// GPT-6.2 series (latest - Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT62Astra,
		DisplayName: "GPT-6.2 Astra",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT62Sol,
		DisplayName: "GPT-6.2 Sol",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-6.1 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT61Sol,
		DisplayName: "GPT-6.1 Sol",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT61Terra,
		DisplayName: "GPT-6.1 Terra",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-6 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT6Astra,
		DisplayName: "GPT-6 Astra",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT6Luna,
		DisplayName: "GPT-6 Luna",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT6Sol,
		DisplayName: "GPT-6 Sol",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT Daybreak series (cybersecurity-specialized aliases)
	{
		ID:          ModelGPTDaybreakBlueLatest,
		DisplayName: "Daybreak Blue",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPTDaybreakRedLatest,
		DisplayName: "Daybreak Red",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5.6 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT56,
		DisplayName: "GPT-5.6",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT56Luna,
		DisplayName: "GPT-5.6 Luna",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT56Sol,
		DisplayName: "GPT-5.6 Sol",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT56Terra,
		DisplayName: "GPT-5.6 Terra",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5.5 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT55,
		DisplayName: "GPT-5.5",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT55Pro,
		DisplayName: "GPT-5.5 Pro",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5.3 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT53Codex,
		DisplayName: "GPT-5.3 Codex",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT53CodexSpark,
		DisplayName: "GPT-5.3 Codex Spark",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5.4 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT54,
		DisplayName: "GPT-5.4",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT54Pro,
		DisplayName: "GPT-5.4 Pro",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT54Mini,
		DisplayName: "GPT-5.4 Mini",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelGPT54Nano,
		DisplayName: "GPT-5.4 Nano",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// GPT-5.2 series (Responses API with reasoning and built-in tools)
	{
		ID:          ModelGPT52,
		DisplayName: "GPT-5.2",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
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
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// Reasoning models (o-series) - Responses API with reasoning
	{
		ID:          ModelO3,
		DisplayName: "o3",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	{
		ID:          ModelO3Pro,
		DisplayName: "o3-pro",
		APIEndpoint: core.APIEndpointResponses,
		Capabilities: []core.Feature{
			core.FeatureStructuredOutput,
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureBuiltInTools,
			core.FeatureResponseChain,
		},
	},
	// Image generation models
	{
		ID:          ModelGPTImage2,
		DisplayName: "GPT Image 2",
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
