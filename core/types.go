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
	FeatureStructuredOutput         Feature = "structured_output"
	FeatureBatch                    Feature = "batch"
)

// ResponseFormat specifies the output format constraint for chat responses.
type ResponseFormat string

const (
	// ResponseFormatText is the default format with no constraints.
	ResponseFormatText ResponseFormat = "text"
	// ResponseFormatJSON forces the model to output valid JSON.
	ResponseFormatJSON ResponseFormat = "json_object"
	// ResponseFormatJSONSchema forces output matching a specific JSON Schema.
	ResponseFormatJSONSchema ResponseFormat = "json_schema"
)

// JSONSchemaDefinition represents a JSON Schema for structured output.
// When provided, the model's output will conform to this schema.
type JSONSchemaDefinition struct {
	// Name is a required identifier for the schema (used by some providers).
	Name string `json:"name"`
	// Description explains what the schema represents (optional).
	Description string `json:"description,omitempty"`
	// Schema is the JSON Schema definition.
	Schema json.RawMessage `json:"schema"`
	// Strict enables strict schema validation (recommended).
	// When true, the model will always output valid JSON matching the schema.
	Strict bool `json:"strict,omitempty"`
}

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
	RoleTool      Role = "tool" // For tool result messages
)

// Message represents a single message in a conversation.
// For simple text messages, use Content. For multimodal messages, use Parts.
// If Parts is non-empty, Content is ignored.
type Message struct {
	Role        Role          `json:"role"`
	Content     string        `json:"content,omitempty"`
	Parts       []ContentPart `json:"-"`                      // Multimodal content parts (Responses API only)
	ToolCalls   []ToolCall    `json:"tool_calls,omitempty"`   // For assistant messages requesting tools
	ToolResults []ToolResult  `json:"tool_results,omitempty"` // For tool result messages (RoleTool)
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

// ToolResult represents the outcome of executing a tool.
// Use this for untyped tool results where the Content can be any JSON-serializable value.
type ToolResult struct {
	CallID  string `json:"call_id"`  // Must match ToolCall.ID from the response
	Content any    `json:"content"`  // Result data (will be JSON marshaled)
	IsError bool   `json:"is_error"` // True if this represents an error
}

// TypedToolResult is a type-safe tool result with compile-time type checking.
// Use this when you want type safety for tool results.
type TypedToolResult[T any] struct {
	CallID  string `json:"call_id"`
	Content T      `json:"content"`
	IsError bool   `json:"is_error"`
}

// ToUntyped converts a typed result to the untyped ToolResult for use with ChatBuilder.
func (r TypedToolResult[T]) ToUntyped() ToolResult {
	return ToolResult{
		CallID:  r.CallID,
		Content: r.Content,
		IsError: r.IsError,
	}
}

// ToolResultBuilder provides a fluent API for constructing tool results.
type ToolResultBuilder struct {
	results []ToolResult
}

// NewToolResults creates a new builder for tool results.
func NewToolResults() *ToolResultBuilder {
	return &ToolResultBuilder{
		results: make([]ToolResult, 0),
	}
}

// Success adds a successful tool result.
func (b *ToolResultBuilder) Success(callID string, content any) *ToolResultBuilder {
	b.results = append(b.results, ToolResult{
		CallID:  callID,
		Content: content,
		IsError: false,
	})
	return b
}

// Error adds a failed tool result.
func (b *ToolResultBuilder) Error(callID string, err error) *ToolResultBuilder {
	b.results = append(b.results, ToolResult{
		CallID:  callID,
		Content: err.Error(),
		IsError: true,
	})
	return b
}

// FromExecution adds a result from a tool execution, handling both success and error cases.
func (b *ToolResultBuilder) FromExecution(callID string, result any, err error) *ToolResultBuilder {
	if err != nil {
		return b.Error(callID, err)
	}
	return b.Success(callID, result)
}

// Build returns the accumulated results.
func (b *ToolResultBuilder) Build() []ToolResult {
	return b.results
}

// TypedToolResultBuilder provides a type-safe fluent API for constructing tool results.
type TypedToolResultBuilder[T any] struct {
	results []TypedToolResult[T]
}

// NewTypedToolResults creates a new type-safe builder for tool results.
func NewTypedToolResults[T any]() *TypedToolResultBuilder[T] {
	return &TypedToolResultBuilder[T]{
		results: make([]TypedToolResult[T], 0),
	}
}

// Success adds a successful typed tool result.
func (b *TypedToolResultBuilder[T]) Success(callID string, content T) *TypedToolResultBuilder[T] {
	b.results = append(b.results, TypedToolResult[T]{
		CallID:  callID,
		Content: content,
		IsError: false,
	})
	return b
}

// Build returns the accumulated results as untyped ToolResults for use with ChatBuilder.
func (b *TypedToolResultBuilder[T]) Build() []ToolResult {
	out := make([]ToolResult, len(b.results))
	for i, r := range b.results {
		out[i] = r.ToUntyped()
	}
	return out
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

	// Structured output fields
	ResponseFormat ResponseFormat        `json:"response_format,omitempty"`
	JSONSchema     *JSONSchemaDefinition `json:"json_schema,omitempty"`

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

// HasToolCalls reports whether the response contains any tool calls.
func (r *ChatResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// FirstToolCall returns the first tool call, or nil if there are none.
// This is convenient for single-tool scenarios:
//
//	if tc := resp.FirstToolCall(); tc != nil {
//	    // handle tool call
//	}
func (r *ChatResponse) FirstToolCall() *ToolCall {
	if len(r.ToolCalls) > 0 {
		return &r.ToolCalls[0]
	}
	return nil
}

// HasReasoning reports whether the response contains reasoning output.
func (r *ChatResponse) HasReasoning() bool {
	return r.Reasoning != nil && len(r.Reasoning.Summary) > 0
}

// ChatChunk represents an incremental streaming response.
// Delta contains incremental assistant text.
type ChatChunk struct {
	Delta string `json:"delta"`
}

// -----------------------------------------------------------------------------
// Batch API Types
// -----------------------------------------------------------------------------

// BatchID uniquely identifies a batch request.
type BatchID string

// BatchStatus represents the state of a batch request.
type BatchStatus string

const (
	// BatchStatusPending indicates the batch is queued but not yet processing.
	BatchStatusPending BatchStatus = "pending"
	// BatchStatusInProgress indicates the batch is currently being processed.
	BatchStatusInProgress BatchStatus = "in_progress"
	// BatchStatusCompleted indicates all requests in the batch have finished.
	BatchStatusCompleted BatchStatus = "completed"
	// BatchStatusFailed indicates the batch failed (check individual results).
	BatchStatusFailed BatchStatus = "failed"
	// BatchStatusCancelled indicates the batch was cancelled by the user.
	BatchStatusCancelled BatchStatus = "cancelled"
	// BatchStatusExpired indicates the batch expired before completion.
	BatchStatusExpired BatchStatus = "expired"
)

// BatchInfo contains metadata about a batch.
type BatchInfo struct {
	// ID is the unique identifier for this batch.
	ID BatchID `json:"id"`
	// Status is the current processing state.
	Status BatchStatus `json:"status"`
	// Total is the total number of requests in the batch.
	Total int `json:"total"`
	// Completed is the number of successfully completed requests.
	Completed int `json:"completed"`
	// Failed is the number of failed requests.
	Failed int `json:"failed"`
	// CreatedAt is when the batch was created.
	CreatedAt int64 `json:"created_at"`
	// CompletedAt is when the batch finished (nil if still processing).
	CompletedAt *int64 `json:"completed_at,omitempty"`
	// ExpiresAt is when the batch will expire if not completed.
	ExpiresAt *int64 `json:"expires_at,omitempty"`
	// Endpoint is the API endpoint used for this batch.
	Endpoint string `json:"endpoint,omitempty"`
	// ErrorFileID contains error details if the batch failed.
	ErrorFileID string `json:"error_file_id,omitempty"`
	// OutputFileID contains results when the batch completes.
	OutputFileID string `json:"output_file_id,omitempty"`
}

// IsComplete returns true if the batch has finished processing.
func (b *BatchInfo) IsComplete() bool {
	return b.Status == BatchStatusCompleted ||
		b.Status == BatchStatusFailed ||
		b.Status == BatchStatusCancelled ||
		b.Status == BatchStatusExpired
}

// BatchRequest wraps a ChatRequest with a custom ID for correlation.
type BatchRequest struct {
	// CustomID is a user-provided identifier for correlating results.
	// Must be unique within a batch and 64 characters or fewer.
	CustomID string `json:"custom_id"`
	// Request is the chat request to process.
	Request ChatRequest `json:"request"`
}

// BatchResult contains the response for a single request in a batch.
type BatchResult struct {
	// CustomID is the user-provided identifier from the request.
	CustomID string `json:"custom_id"`
	// Response is the chat response (nil if request failed).
	Response *ChatResponse `json:"response,omitempty"`
	// Error contains error details if the request failed.
	Error *BatchError `json:"error,omitempty"`
}

// BatchError contains error details for a failed batch request.
type BatchError struct {
	// Code is the error code (e.g., "rate_limit_exceeded").
	Code string `json:"code"`
	// Message is a human-readable error description.
	Message string `json:"message"`
}

// IsSuccess returns true if the batch request succeeded.
func (r *BatchResult) IsSuccess() bool {
	return r.Error == nil && r.Response != nil
}
