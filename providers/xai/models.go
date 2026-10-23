// Package xai provides an xAI Grok API provider implementation for Iris.
package xai

import "github.com/petal-labs/iris/core"

// Model constants for xAI Grok models.
const (
	// Grok 4.7 series (latest)
	ModelGrok47 core.ModelID = "grok-4.7"

	// Grok 4.6 series
	ModelGrok46 core.ModelID = "grok-4.6"

	// Grok 4.5 series
	ModelGrok45 core.ModelID = "grok-4.5"

	// Grok 4.3 series
	ModelGrok43 core.ModelID = "grok-4.3"

	// Grok 4.20 series (general availability)
	ModelGrok420MultiAgent   core.ModelID = "grok-4.20-multi-agent-0309"
	ModelGrok420Reasoning    core.ModelID = "grok-4.20-0309-reasoning"
	ModelGrok420NonReasoning core.ModelID = "grok-4.20-0309-non-reasoning"

	// Grok Build
	ModelGrokBuild01 core.ModelID = "grok-build-0.1"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// Grok 4.7 series (latest)
	{
		ID:          ModelGrok47,
		DisplayName: "Grok 4.7",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Grok 4.6 series
	{
		ID:          ModelGrok46,
		DisplayName: "Grok 4.6",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Grok 4.5 series
	{
		ID:          ModelGrok45,
		DisplayName: "Grok 4.5",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Grok 4.3 series
	{
		ID:          ModelGrok43,
		DisplayName: "Grok 4.3",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	// Grok 4.20 series (general availability)
	{
		ID:          ModelGrok420MultiAgent,
		DisplayName: "Grok 4.20 Multi-Agent",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGrok420Reasoning,
		DisplayName: "Grok 4.20 (Reasoning)",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          ModelGrok420NonReasoning,
		DisplayName: "Grok 4.20 (Non-Reasoning)",
		APIEndpoint: core.APIEndpointCompletions,
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// Grok Build
	{
		ID:          ModelGrokBuild01,
		DisplayName: "Grok Build 0.1",
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
