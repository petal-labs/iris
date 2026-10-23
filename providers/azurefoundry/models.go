package azurefoundry

import "github.com/petal-labs/iris/core"

// models defines the common models available through Azure AI Foundry.
// Actual availability depends on your Azure deployment configuration.
// These are representative models; Azure supports many more through
// Azure OpenAI Service and the Model Inference API.
var models = []core.ModelInfo{
	// -------------------------------------------------------------------------
	// Anthropic Claude Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "claude-fable-5",
		DisplayName: "Claude Fable 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-fable-5-1",
		DisplayName: "Claude Fable 5.1",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-haiku-4-5",
		DisplayName: "Claude Haiku 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-haiku-5-5",
		DisplayName: "Claude Haiku 5.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-mythos-5",
		DisplayName: "Claude Mythos 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-opus-4-5",
		DisplayName: "Claude Opus 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-opus-4-6",
		DisplayName: "Claude Opus 4.6",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-opus-4-7",
		DisplayName: "Claude Opus 4.7",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-opus-4-8",
		DisplayName: "Claude Opus 4.8",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-opus-5",
		DisplayName: "Claude Opus 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-opus-5-5",
		DisplayName: "Claude Opus 5.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-sonnet-4-5",
		DisplayName: "Claude Sonnet 4.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-sonnet-4-6",
		DisplayName: "Claude Sonnet 4.6",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-sonnet-5",
		DisplayName: "Claude Sonnet 5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "claude-sonnet-5-5",
		DisplayName: "Claude Sonnet 5.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	// -------------------------------------------------------------------------
	// Mistral Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "codestral-2501",
		DisplayName: "Codestral 25.01",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "ministral-3b",
		DisplayName: "Ministral 3B",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "mistral-medium-2505",
		DisplayName: "Mistral Medium 3",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "mistral-small-2503",
		DisplayName: "Mistral Small 3.1",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// -------------------------------------------------------------------------
	// Cohere Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "cohere-command-a",
		DisplayName: "Command A",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "cohere-embed-v-4-0",
		DisplayName: "Embed v4",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "cohere-embed-v3-english",
		DisplayName: "Embed v3 English",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "cohere-embed-v3-multilingual",
		DisplayName: "Embed v3 Multilingual",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	// -------------------------------------------------------------------------
	// DeepSeek Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "deepseek-v3.2",
		DisplayName: "DeepSeek-V3.2",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "deepseek-v3.2-speciale",
		DisplayName: "DeepSeek-V3.2-Speciale",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "deepseek-v4-flash",
		DisplayName: "DeepSeek-V4-Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "deepseek-v4-pro",
		DisplayName: "DeepSeek-V4-Pro",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "deepseek-v4.1-flash",
		DisplayName: "DeepSeek-V4.1-Flash",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	// -------------------------------------------------------------------------
	// OpenAI Models (via Azure OpenAI Service)
	// -------------------------------------------------------------------------
	{
		ID:          "gpt-5",
		DisplayName: "GPT-5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "gpt-5-codex",
		DisplayName: "GPT-5-Codex",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "gpt-5-mini",
		DisplayName: "GPT-5 Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "gpt-5-nano",
		DisplayName: "GPT-5 Nano",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "gpt-5-pro",
		DisplayName: "GPT-5 Pro",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.1",
		DisplayName: "GPT-5.1",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.1-codex",
		DisplayName: "GPT-5.1 Codex",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.1-codex-max",
		DisplayName: "GPT-5.1 Codex Max",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.1-codex-mini",
		DisplayName: "GPT-5.1 Codex Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.2",
		DisplayName: "GPT-5.2",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.2-codex",
		DisplayName: "GPT-5.2 Codex",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.3-codex",
		DisplayName: "GPT-5.3 Codex",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.4",
		DisplayName: "GPT-5.4",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.4-mini",
		DisplayName: "GPT-5.4 Mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.4-nano",
		DisplayName: "GPT-5.4 Nano",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.4-pro",
		DisplayName: "GPT-5.4 Pro",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "gpt-5.5",
		DisplayName: "GPT-5.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.6-luna",
		DisplayName: "GPT-5.6 Luna",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.6-sol",
		DisplayName: "GPT-5.6 Sol",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-5.6-terra",
		DisplayName: "GPT-5.6 Terra",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-6-astra",
		DisplayName: "GPT-6 Astra",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-6-luna",
		DisplayName: "GPT-6 Luna",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-6-sol",
		DisplayName: "GPT-6 Sol",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-6.1-sol",
		DisplayName: "GPT-6.1 Sol",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "gpt-chat-latest",
		DisplayName: "GPT Chat Latest",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "o3",
		DisplayName: "o3",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "text-embedding-3-large",
		DisplayName: "text-embedding-3-large",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "text-embedding-3-small",
		DisplayName: "text-embedding-3-small",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	{
		ID:          "text-embedding-ada-002",
		DisplayName: "text-embedding-ada-002",
		Capabilities: []core.Feature{
			core.FeatureEmbeddings,
		},
	},
	// -------------------------------------------------------------------------
	// xAI Grok Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "grok-4-1-fast-non-reasoning",
		DisplayName: "Grok 4.1 Fast (Non-Reasoning)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "grok-4-1-fast-reasoning",
		DisplayName: "Grok 4.1 Fast (Reasoning)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "grok-4-20-non-reasoning",
		DisplayName: "Grok 4.20 (Non-Reasoning)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "grok-4-20-reasoning",
		DisplayName: "Grok 4.20 (Reasoning)",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "grok-4.6",
		DisplayName: "Grok 4.6",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "grok-4.7",
		DisplayName: "Grok 4.7",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	// -------------------------------------------------------------------------
	// Moonshot AI Kimi Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "kimi-k2.5",
		DisplayName: "Kimi K2.5",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "kimi-k2.6",
		DisplayName: "Kimi K2.6",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	{
		ID:          "kimi-k2.7-code",
		DisplayName: "Kimi K2.7 Code",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
			core.FeatureStructuredOutput,
		},
	},
	// -------------------------------------------------------------------------
	// Meta Llama Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "llama-3.3-70b-instruct",
		DisplayName: "Llama-3.3-70B-Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "llama-4-maverick-17b-128e-instruct-fp8",
		DisplayName: "Llama 4 Maverick 17B 128E Instruct FP8",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "llama-4-scout-17b-16e-instruct",
		DisplayName: "Llama 4 Scout 17B 16E Instruct",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// -------------------------------------------------------------------------
	// Model Router
	// -------------------------------------------------------------------------
	{
		ID:          "model-router",
		DisplayName: "Model Router",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	// -------------------------------------------------------------------------
	// Microsoft Phi Models (via Model Inference API)
	// -------------------------------------------------------------------------
	{
		ID:          "phi-4",
		DisplayName: "Phi-4",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "phi-4-mini",
		DisplayName: "Phi-4-mini",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
		},
	},
	{
		ID:          "phi-4-mini-reasoning",
		DisplayName: "Phi-4-mini-reasoning",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureToolCalling,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "phi-4-multimodal",
		DisplayName: "Phi-4-multimodal",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
		},
	},
	{
		ID:          "phi-4-reasoning",
		DisplayName: "Phi-4-reasoning",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
	{
		ID:          "phi-4-reasoning-plus",
		DisplayName: "Phi-4-reasoning-plus",
		Capabilities: []core.Feature{
			core.FeatureChat,
			core.FeatureChatStreaming,
			core.FeatureReasoning,
		},
	},
}

// GetModelInfo returns a copy of the model info for a given model ID, or nil
// if not found. The returned pointer references a fresh copy whose fields
// (including the Capabilities slice) do not alias the package's static
// catalog, so mutating it cannot affect subsequent GetModelInfo or Models calls.
func GetModelInfo(id core.ModelID) *core.ModelInfo {
	for i := range models {
		if models[i].ID != id {
			continue
		}
		cp := models[i]
		if caps := cp.Capabilities; caps != nil {
			cp.Capabilities = make([]core.Feature, len(caps))
			copy(cp.Capabilities, caps)
		}
		return &cp
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
