package azurefoundry

import "github.com/petal-labs/iris/core"

// models defines the common models available through Azure AI Foundry.
// Actual availability depends on your Azure deployment configuration.
// These are representative models; Azure supports many more through
// Azure OpenAI Service and the Model Inference API.
var models = []core.ModelInfo{
	// -------------------------------------------------------------------------
	// OpenAI Models (via Azure OpenAI Service)
	// -------------------------------------------------------------------------

	// GPT-4o family
	{
		ID:          "gpt-4o",
		DisplayName: "GPT-4o",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-4o-mini",
		DisplayName: "GPT-4o Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureStructuredOutput,
		},
	},

	// GPT-4 family
	{
		ID:          "gpt-4-turbo",
		DisplayName: "GPT-4 Turbo",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-4",
		DisplayName: "GPT-4",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "gpt-4-32k",
		DisplayName: "GPT-4 32K",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},

	// GPT-3.5 family
	{
		ID:          "gpt-35-turbo",
		DisplayName: "GPT-3.5 Turbo",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "gpt-35-turbo-16k",
		DisplayName: "GPT-3.5 Turbo 16K",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},

	// Reasoning models (o1/o3 series)
	{
		ID:          "o1",
		DisplayName: "o1",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "o1-mini",
		DisplayName: "o1 Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "o1-preview",
		DisplayName: "o1 Preview",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "o3-mini",
		DisplayName: "o3 Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},

	// -------------------------------------------------------------------------
	// OpenAI Embedding Models
	// -------------------------------------------------------------------------
	{
		ID:          "text-embedding-3-large",
		DisplayName: "Text Embedding 3 Large",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "text-embedding-3-small",
		DisplayName: "Text Embedding 3 Small",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "text-embedding-ada-002",
		DisplayName: "Text Embedding Ada 002",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},

	// -------------------------------------------------------------------------
	// Meta Llama Models (via Model Inference API)
	// -------------------------------------------------------------------------

	// Llama 3.1 family
	{
		ID:          "Meta-Llama-3.1-405B-Instruct",
		DisplayName: "Llama 3.1 405B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "Meta-Llama-3.1-70B-Instruct",
		DisplayName: "Llama 3.1 70B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "Meta-Llama-3.1-8B-Instruct",
		DisplayName: "Llama 3.1 8B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},

	// Llama 3.2 family
	{
		ID:          "Llama-3.2-90B-Vision-Instruct",
		DisplayName: "Llama 3.2 90B Vision Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Llama-3.2-11B-Vision-Instruct",
		DisplayName: "Llama 3.2 11B Vision Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Llama-3.2-3B-Instruct",
		DisplayName: "Llama 3.2 3B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Llama-3.2-1B-Instruct",
		DisplayName: "Llama 3.2 1B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},

	// Llama 3.3 family
	{
		ID:          "Llama-3.3-70B-Instruct",
		DisplayName: "Llama 3.3 70B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},

	// -------------------------------------------------------------------------
	// Mistral Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "Mistral-Large-2411",
		DisplayName: "Mistral Large (Nov 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "Mistral-Large-2407",
		DisplayName: "Mistral Large (Jul 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "Mistral-Small-2409",
		DisplayName: "Mistral Small (Sep 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Mistral-Nemo-2407",
		DisplayName: "Mistral Nemo (Jul 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Ministral-3B-2410",
		DisplayName: "Ministral 3B (Oct 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},

	// -------------------------------------------------------------------------
	// Cohere Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "Cohere-command-r-plus",
		DisplayName: "Cohere Command R+",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "Cohere-command-r",
		DisplayName: "Cohere Command R",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Cohere-command-r-08-2024",
		DisplayName: "Cohere Command R (Aug 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Cohere-command-r-plus-08-2024",
		DisplayName: "Cohere Command R+ (Aug 2024)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},

	// Cohere embedding models
	{
		ID:          "Cohere-embed-v3-english",
		DisplayName: "Cohere Embed v3 English",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "Cohere-embed-v3-multilingual",
		DisplayName: "Cohere Embed v3 Multilingual",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},

	// -------------------------------------------------------------------------
	// DeepSeek Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "DeepSeek-V3",
		DisplayName: "DeepSeek V3",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "DeepSeek-R1",
		DisplayName: "DeepSeek R1",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},

	// -------------------------------------------------------------------------
	// Microsoft Phi Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "Phi-4",
		DisplayName: "Phi-4",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3.5-mini-instruct",
		DisplayName: "Phi-3.5 Mini Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3.5-MoE-instruct",
		DisplayName: "Phi-3.5 MoE Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3.5-vision-instruct",
		DisplayName: "Phi-3.5 Vision Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3-mini-4k-instruct",
		DisplayName: "Phi-3 Mini 4K Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3-mini-128k-instruct",
		DisplayName: "Phi-3 Mini 128K Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3-small-8k-instruct",
		DisplayName: "Phi-3 Small 8K Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3-small-128k-instruct",
		DisplayName: "Phi-3 Small 128K Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3-medium-4k-instruct",
		DisplayName: "Phi-3 Medium 4K Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "Phi-3-medium-128k-instruct",
		DisplayName: "Phi-3 Medium 128K Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},

	// -------------------------------------------------------------------------
	// AI21 Labs Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "AI21-Jamba-1.5-Large",
		DisplayName: "AI21 Jamba 1.5 Large",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "AI21-Jamba-1.5-Mini",
		DisplayName: "AI21 Jamba 1.5 Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "jamba-instruct",
		DisplayName: "AI21 Jamba Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},

	// -------------------------------------------------------------------------
	// JAIS Models (via Model Inference API) - Arabic/English bilingual
	// -------------------------------------------------------------------------
	{
		ID:          "jais-30b-chat",
		DisplayName: "JAIS 30B Chat",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},

	// -------------------------------------------------------------------------
	// NVIDIA Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "Nemotron-4-340B-Instruct",
		DisplayName: "Nemotron 4 340B Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
}

// GetModelInfo returns the model info for a given model ID, or nil if not found.
func GetModelInfo(id core.ModelID) *core.ModelInfo {
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	return nil
}

// ListModels returns all available model definitions.
func ListModels() []core.ModelInfo {
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// ListChatModels returns all models that support chat completions.
func ListChatModels() []core.ModelInfo {
	var result []core.ModelInfo
	for _, m := range models {
		if m.HasCapability(core.FeatureChat) {
			result = append(result, m)
		}
	}
	return result
}

// ListStreamingModels returns all models that support streaming.
func ListStreamingModels() []core.ModelInfo {
	var result []core.ModelInfo
	for _, m := range models {
		if m.HasCapability(core.FeatureChatStreaming) {
			result = append(result, m)
		}
	}
	return result
}

// ListEmbeddingModels returns all models that support embeddings.
func ListEmbeddingModels() []core.ModelInfo {
	var result []core.ModelInfo
	for _, m := range models {
		if m.HasCapability(core.FeatureEmbeddings) {
			result = append(result, m)
		}
	}
	return result
}

// ListToolCallingModels returns all models that support tool calling.
func ListToolCallingModels() []core.ModelInfo {
	var result []core.ModelInfo
	for _, m := range models {
		if m.HasCapability(core.FeatureToolCalling) {
			result = append(result, m)
		}
	}
	return result
}

// ListReasoningModels returns all models that support extended reasoning.
func ListReasoningModels() []core.ModelInfo {
	var result []core.ModelInfo
	for _, m := range models {
		if m.HasCapability(core.FeatureReasoning) {
			result = append(result, m)
		}
	}
	return result
}

// ModelsByCapability returns all models that have the specified capability.
func ModelsByCapability(capability core.Feature) []core.ModelInfo {
	var result []core.ModelInfo
	for _, m := range models {
		if m.HasCapability(capability) {
			result = append(result, m)
		}
	}
	return result
}

// SupportsCapability checks if a model supports a specific capability.
func SupportsCapability(id core.ModelID, capability core.Feature) bool {
	info := GetModelInfo(id)
	if info == nil {
		return false
	}
	return info.HasCapability(capability)
}

// IsEmbeddingModel returns true if the model is an embedding model.
func IsEmbeddingModel(id core.ModelID) bool {
	return SupportsCapability(id, core.FeatureEmbeddings)
}

// IsChatModel returns true if the model supports chat completions.
func IsChatModel(id core.ModelID) bool {
	return SupportsCapability(id, core.FeatureChat)
}

// IsReasoningModel returns true if the model supports extended reasoning.
func IsReasoningModel(id core.ModelID) bool {
	return SupportsCapability(id, core.FeatureReasoning)
}

// SupportsToolCalling returns true if the model supports tool calling.
func SupportsToolCalling(id core.ModelID) bool {
	return SupportsCapability(id, core.FeatureToolCalling)
}

// SupportsStructuredOutput returns true if the model supports structured output.
func SupportsStructuredOutput(id core.ModelID) bool {
	return SupportsCapability(id, core.FeatureStructuredOutput)
}
