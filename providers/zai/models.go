package zai

import "github.com/petal-labs/iris/core"

// Model constants for Z.ai GLM models.
const (
	// GLM-4.7 series (latest flagship)
	ModelGLM47       core.ModelID = "glm-4.7"
	ModelGLM47Flash  core.ModelID = "glm-4.7-flash"
	ModelGLM47FlashX core.ModelID = "glm-4.7-flashx"

	// GLM-4.6 series
	ModelGLM46        core.ModelID = "glm-4.6"
	ModelGLM46V       core.ModelID = "glm-4.6v"
	ModelGLM46VFlash  core.ModelID = "glm-4.6v-flash"
	ModelGLM46VFlashX core.ModelID = "glm-4.6v-flashx"

	// GLM-4.5 series
	ModelGLM45      core.ModelID = "glm-4.5"
	ModelGLM45V     core.ModelID = "glm-4.5v"
	ModelGLM45X     core.ModelID = "glm-4.5-x"
	ModelGLM45Air   core.ModelID = "glm-4.5-air"
	ModelGLM45AirX  core.ModelID = "glm-4.5-airx"
	ModelGLM45Flash core.ModelID = "glm-4.5-flash"

	// GLM-4 32B
	ModelGLM4_32B core.ModelID = "glm-4-32b-0414-128k"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// GLM-4.7 series (latest flagship)
	{
		ID:          ModelGLM47,
		DisplayName: "GLM-4.7",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGLM47Flash,
		DisplayName: "GLM-4.7 Flash",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGLM47FlashX,
		DisplayName: "GLM-4.7 FlashX",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// GLM-4.6 series
	{
		ID:          ModelGLM46,
		DisplayName: "GLM-4.6",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGLM46V,
		DisplayName: "GLM-4.6V",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			// Note: Vision model - supports image inputs
		},
	},
	{
		ID:          ModelGLM46VFlash,
		DisplayName: "GLM-4.6V Flash",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			// Note: Vision model - supports image inputs
		},
	},
	{
		ID:          ModelGLM46VFlashX,
		DisplayName: "GLM-4.6V FlashX",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			// Note: Vision model - supports image inputs
		},
	},
	// GLM-4.5 series
	{
		ID:          ModelGLM45,
		DisplayName: "GLM-4.5",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGLM45V,
		DisplayName: "GLM-4.5V",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			// Note: Vision model - supports image inputs
		},
	},
	{
		ID:          ModelGLM45X,
		DisplayName: "GLM-4.5-X",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGLM45Air,
		DisplayName: "GLM-4.5 Air",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGLM45AirX,
		DisplayName: "GLM-4.5 AirX",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          ModelGLM45Flash,
		DisplayName: "GLM-4.5 Flash",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// GLM-4 32B
	{
		ID:          ModelGLM4_32B,
		DisplayName: "GLM-4 32B",
		APIEndpoint: core.APIEndpointCompletions,
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
