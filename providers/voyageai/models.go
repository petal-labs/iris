// Package voyageai provides a Voyage AI API provider implementation for Iris.
package voyageai

import "github.com/petal-labs/iris/core"

// Model constants for Voyage AI models.
const (
	// Embedding models - voyage-4 series
	ModelVoyage4Large core.ModelID = "voyage-4-large"
	ModelVoyage4      core.ModelID = "voyage-4"
	ModelVoyage4Lite  core.ModelID = "voyage-4-lite"

	// Embedding models - voyage-3.5 series
	ModelVoyage35     core.ModelID = "voyage-3.5"
	ModelVoyage35Lite core.ModelID = "voyage-3.5-lite"

	// Embedding models - voyage-3 series
	ModelVoyage3Large core.ModelID = "voyage-3-large"

	// Embedding models - specialized
	ModelVoyageCode3    core.ModelID = "voyage-code-3"
	ModelVoyageFinance2 core.ModelID = "voyage-finance-2"
	ModelVoyageLaw2     core.ModelID = "voyage-law-2"

	// Contextualized embedding models
	ModelVoyageContext3 core.ModelID = "voyage-context-3"

	// Reranker models
	ModelRerank25     core.ModelID = "rerank-2.5"
	ModelRerank25Lite core.ModelID = "rerank-2.5-lite"
	ModelRerank2      core.ModelID = "rerank-2"
	ModelRerank2Lite  core.ModelID = "rerank-2-lite"
)

// models is the static list of supported models.
var models = []core.ModelInfo{
	// Voyage-4 series (embeddings)
	{
		ID:          ModelVoyage4Large,
		DisplayName: "Voyage 4 Large",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          ModelVoyage4,
		DisplayName: "Voyage 4",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          ModelVoyage4Lite,
		DisplayName: "Voyage 4 Lite",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	// Voyage-3.5 series (embeddings)
	{
		ID:          ModelVoyage35,
		DisplayName: "Voyage 3.5",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          ModelVoyage35Lite,
		DisplayName: "Voyage 3.5 Lite",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	// Voyage-3 series (embeddings)
	{
		ID:          ModelVoyage3Large,
		DisplayName: "Voyage 3 Large",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	// Specialized embedding models
	{
		ID:          ModelVoyageCode3,
		DisplayName: "Voyage Code 3",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          ModelVoyageFinance2,
		DisplayName: "Voyage Finance 2",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          ModelVoyageLaw2,
		DisplayName: "Voyage Law 2",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	// Contextualized embedding models
	{
		ID:          ModelVoyageContext3,
		DisplayName: "Voyage Context 3",
		Capabilities: []core.Feature{
			core.FeatureContextualizedEmbeddings,
		},
	},
	// Reranker models
	{
		ID:          ModelRerank25,
		DisplayName: "Rerank 2.5",
		Capabilities: []core.Feature{
			core.FeatureReranking,
		},
	},
	{
		ID:          ModelRerank25Lite,
		DisplayName: "Rerank 2.5 Lite",
		Capabilities: []core.Feature{
			core.FeatureReranking,
		},
	},
	{
		ID:          ModelRerank2,
		DisplayName: "Rerank 2",
		Capabilities: []core.Feature{
			core.FeatureReranking,
		},
	},
	{
		ID:          ModelRerank2Lite,
		DisplayName: "Rerank 2 Lite",
		Capabilities: []core.Feature{
			core.FeatureReranking,
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
