package xai

import "encoding/json"

// xaiRequest represents a request to the xAI chat completions API.
type xaiRequest struct {
	Model           string       `json:"model"`
	Messages        []xaiMessage `json:"messages"`
	Temperature     *float32     `json:"temperature,omitempty"`
	MaxTokens       *int         `json:"max_tokens,omitempty"`
	Stream          bool         `json:"stream"`
	Tools           []xaiTool    `json:"tools,omitempty"`
	ToolChoice      any          `json:"tool_choice,omitempty"`
	ReasoningEffort string       `json:"reasoning_effort,omitempty"`
}

// xaiMessage represents a message in the xAI format.
type xaiMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// xaiTool represents a tool definition in the xAI format.
type xaiTool struct {
	Type     string      `json:"type"`
	Function xaiFunction `json:"function"`
}

// xaiFunction represents a function definition for xAI tools.
type xaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// xaiResponse represents a response from the xAI chat completions API.
type xaiResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []xaiChoice `json:"choices"`
	Usage   xaiUsage    `json:"usage"`
}

// xaiChoice represents a single choice in an xAI response.
type xaiChoice struct {
	Index        int        `json:"index"`
	Message      xaiRespMsg `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

// xaiRespMsg represents the assistant message in a response.
type xaiRespMsg struct {
	Role             string        `json:"role"`
	Content          string        `json:"content"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	ToolCalls        []xaiToolCall `json:"tool_calls,omitempty"`
}

// xaiToolCall represents a tool call in an xAI response.
type xaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function xaiFunctionCall `json:"function"`
}

// xaiFunctionCall represents the function details in a tool call.
type xaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// xaiUsage represents token usage in an xAI response.
type xaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

// Streaming response types for xAI SSE protocol.

// xaiStreamChunk represents a single chunk in an xAI streaming response.
type xaiStreamChunk struct {
	ID      string            `json:"id"`
	Model   string            `json:"model"`
	Choices []xaiStreamChoice `json:"choices"`
	Usage   *xaiUsage         `json:"usage,omitempty"`
}

// xaiStreamChoice represents a single choice in a streaming chunk.
type xaiStreamChoice struct {
	Index        int            `json:"index"`
	Delta        xaiStreamDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason,omitempty"`
}

// xaiStreamDelta represents the delta content in a streaming chunk.
type xaiStreamDelta struct {
	Role      string              `json:"role,omitempty"`
	Content   string              `json:"content,omitempty"`
	ToolCalls []xaiStreamToolCall `json:"tool_calls,omitempty"`
}

// xaiStreamToolCall represents a tool call fragment in a streaming chunk.
type xaiStreamToolCall struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function xaiStreamFunction `json:"function,omitempty"`
}

// xaiStreamFunction represents a function fragment in a streaming tool call.
type xaiStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
