package anthropic

import "encoding/json"

// anthropicRequest represents a request to the Anthropic Messages API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Temperature *float32           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	ToolChoice  interface{}        `json:"tool_choice,omitempty"`
}

// anthropicMessage represents a message in the Anthropic format.
type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

// anthropicContentBlock represents a content block in a message.
type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// For tool_result blocks
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

// anthropicTool represents a tool definition in the Anthropic format.
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicResponse represents a response from the Anthropic Messages API.
type anthropicResponse struct {
	ID           string                     `json:"id"`
	Type         string                     `json:"type"`
	Role         string                     `json:"role"`
	Content      []anthropicResponseContent `json:"content"`
	Model        string                     `json:"model"`
	StopReason   string                     `json:"stop_reason"`
	StopSequence *string                    `json:"stop_sequence"`
	Usage        anthropicUsage             `json:"usage"`
}

// anthropicResponseContent represents a content block in a response.
type anthropicResponseContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`    // for tool_use
	Name  string          `json:"name,omitempty"`  // for tool_use
	Input json.RawMessage `json:"input,omitempty"` // for tool_use
}

// anthropicUsage represents token usage in an Anthropic response.
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Streaming types

// anthropicStreamEvent represents a streaming event from the Anthropic API.
// The Type field determines which other fields are populated.
type anthropicStreamEvent struct {
	Type string `json:"type"`
	// For message_start
	Message *anthropicResponse `json:"message,omitempty"`
	// For content_block_start
	Index        int                       `json:"index,omitempty"`
	ContentBlock *anthropicResponseContent `json:"content_block,omitempty"`
	// For content_block_delta
	Delta *anthropicDelta `json:"delta,omitempty"`
	// For message_delta
	Usage *anthropicUsage `json:"usage,omitempty"`
	// For error
	Error *anthropicError `json:"error,omitempty"`
}

// anthropicDelta represents a delta update in streaming.
type anthropicDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// anthropicError represents an error from the Anthropic API.
type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// anthropicErrorResponse represents an error response from the API.
type anthropicErrorResponse struct {
	Type  string         `json:"type"`
	Error anthropicError `json:"error"`
}
