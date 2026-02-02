// Package core provides the Iris SDK client and types.
package core

import "encoding/json"

// Feature represents a capability that a provider may support.
type Feature string

const (
	FeatureChat                     Feature = "chat"
	FeatureChatStreaming            Feature = "chat_streaming"
	FeatureToolCalling              Feature = "tool_calling"
	FeatureReasoning                Feature = "reasoning"
	FeatureBuiltInTools             Feature = "builtin_tools"
	FeatureResponseChain            Feature = "response_chain"
	FeatureEmbeddings               Feature = "embeddings"
	FeatureContextualizedEmbeddings Feature = "contextualized_embeddings"
	FeatureReranking                Feature = "reranking"
)

// APIEndpoint represents which API endpoint a model uses.
type APIEndpoint string

const (
	// APIEndpointCompletions is the Chat Completions API (default for older models).
	APIEndpointCompletions APIEndpoint = "completions"
	// APIEndpointResponses is the Responses API (for newer models like GPT-5.x).
	APIEndpointResponses APIEndpoint = "responses"
)

// ReasoningEffort represents the level of reasoning effort for models that support it.
type ReasoningEffort string

const (
	ReasoningEffortNone   ReasoningEffort = "none"
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
	ReasoningEffortXHigh  ReasoningEffort = "xhigh"
)

// BuiltInTool represents a built-in tool available in the Responses API.
type BuiltInTool struct {
	Type string `json:"type"` // "web_search", "file_search", "code_interpreter"
}

// ReasoningOutput contains reasoning information from the model.
type ReasoningOutput struct {
	ID      string   `json:"id"`
	Summary []string `json:"summary,omitempty"`
}

// ModelInfo describes a model available from a provider.
type ModelInfo struct {
	ID           ModelID     `json:"id"`
	DisplayName  string      `json:"display_name"`
	Capabilities []Feature   `json:"capabilities"`
	APIEndpoint  APIEndpoint `json:"api_endpoint,omitempty"` // defaults to completions
}

// HasCapability reports whether the model supports the given feature.
func (m ModelInfo) HasCapability(f Feature) bool {
	for _, cap := range m.Capabilities {
		if cap == f {
			return true
		}
	}
	return false
}

// GetAPIEndpoint returns the API endpoint for the model, defaulting to completions.
func (m ModelInfo) GetAPIEndpoint() APIEndpoint {
	if m.APIEndpoint == "" {
		return APIEndpointCompletions
	}
	return m.APIEndpoint
}

// ModelID is a string identifier for a model.
// Using string avoids coupling to provider-specific enums.
type ModelID string

// Role represents a message participant role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message represents a single message in a conversation.
// For simple text messages, use Content. For multimodal messages, use Parts.
// If Parts is non-empty, Content is ignored.
type Message struct {
	Role    Role          `json:"role"`
	Content string        `json:"content,omitempty"`
	Parts   []ContentPart `json:"-"` // Multimodal content parts (Responses API only)
}

// TokenUsage tracks token consumption for a request.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolCall represents a tool invocation requested by the model.
// Arguments MUST be valid JSON bytes and MUST preserve raw JSON (no reformatting).
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Tool is a placeholder interface for tool definitions.
// Full implementation is in the tools package (Task 07).
type Tool interface {
	Name() string
	Description() string
}

// ToolResources contains configuration for built-in tools.
type ToolResources struct {
	FileSearch *FileSearchResources `json:"file_search,omitempty"`
}

// FileSearchResources contains vector store IDs for file search.
type FileSearchResources struct {
	VectorStoreIDs []string `json:"vector_store_ids"`
}

// ChatRequest represents a request to a chat model.
type ChatRequest struct {
	Model       ModelID   `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float32  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"-"` // Tools are handled separately by providers

	// Responses API fields (ignored for Chat Completions API)
	Instructions       string          `json:"instructions,omitempty"`
	ReasoningEffort    ReasoningEffort `json:"reasoning_effort,omitempty"`
	BuiltInTools       []BuiltInTool   `json:"builtin_tools,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
	Truncation         string          `json:"truncation,omitempty"`
	ToolResources      *ToolResources  `json:"tool_resources,omitempty"`
}

// ChatResponse represents a response from a chat model.
// For providers returning multiple choices, v0.1 uses only the first choice.
type ChatResponse struct {
	ID        string     `json:"id"`
	Model     ModelID    `json:"model"`
	Output    string     `json:"output"`
	Usage     TokenUsage `json:"usage"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Responses API fields
	Reasoning *ReasoningOutput `json:"reasoning,omitempty"`
	Status    string           `json:"status,omitempty"`
}

// ChatChunk represents an incremental streaming response.
// Delta contains incremental assistant text.
type ChatChunk struct {
	Delta string `json:"delta"`
}
