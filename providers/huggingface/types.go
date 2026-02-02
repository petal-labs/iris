package huggingface

import "encoding/json"

// hfRequest represents a request to the Hugging Face chat completions API.
// This follows the OpenAI-compatible format.
type hfRequest struct {
	Model       string      `json:"model"`
	Messages    []hfMessage `json:"messages"`
	Temperature *float32    `json:"temperature,omitempty"`
	MaxTokens   *int        `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream"`
	Tools       []hfTool    `json:"tools,omitempty"`
	ToolChoice  string      `json:"tool_choice,omitempty"`
}

// hfMessage represents a message in the HF format.
type hfMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// hfTool represents a tool definition in the HF format.
type hfTool struct {
	Type     string     `json:"type"`
	Function hfFunction `json:"function"`
}

// hfFunction represents a function definition for HF tools.
type hfFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// hfResponse represents a response from the HF chat completions API.
type hfResponse struct {
	ID      string     `json:"id"`
	Object  string     `json:"object"`
	Created int64      `json:"created"`
	Model   string     `json:"model"`
	Choices []hfChoice `json:"choices"`
	Usage   hfUsage    `json:"usage"`
}

// hfChoice represents a single choice in an HF response.
type hfChoice struct {
	Index        int       `json:"index"`
	Message      hfRespMsg `json:"message"`
	FinishReason string    `json:"finish_reason"`
}

// hfRespMsg represents the assistant message in a response.
type hfRespMsg struct {
	Role      string       `json:"role"`
	Content   string       `json:"content"`
	ToolCalls []hfToolCall `json:"tool_calls,omitempty"`
}

// hfToolCall represents a tool call in an HF response.
type hfToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function hfFunctionCall `json:"function"`
}

// hfFunctionCall represents the function details in a tool call.
type hfFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// hfUsage represents token usage in an HF response.
type hfUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Streaming response types for SSE protocol.

// hfStreamChunk represents a single chunk in a streaming response.
type hfStreamChunk struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []hfStreamChoice `json:"choices"`
	Usage   *hfUsage         `json:"usage,omitempty"`
}

// hfStreamChoice represents a single choice in a streaming chunk.
type hfStreamChoice struct {
	Index        int           `json:"index"`
	Delta        hfStreamDelta `json:"delta"`
	FinishReason *string       `json:"finish_reason,omitempty"`
}

// hfStreamDelta represents the delta content in a streaming chunk.
type hfStreamDelta struct {
	Role      string             `json:"role,omitempty"`
	Content   string             `json:"content,omitempty"`
	ToolCalls []hfStreamToolCall `json:"tool_calls,omitempty"`
}

// hfStreamToolCall represents a tool call fragment in a streaming chunk.
type hfStreamToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function hfStreamFunction `json:"function,omitempty"`
}

// hfStreamFunction represents a function fragment in a streaming tool call.
type hfStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
