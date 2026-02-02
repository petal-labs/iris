package zai

import "encoding/json"

// Request types

// zaiRequest is the request body for the Z.ai chat completions API.
type zaiRequest struct {
	Model          string       `json:"model"`
	Messages       []zaiMessage `json:"messages"`
	Temperature    *float32     `json:"temperature,omitempty"`
	TopP           *float32     `json:"top_p,omitempty"`
	MaxTokens      *int         `json:"max_tokens,omitempty"`
	Stream         bool         `json:"stream"`
	DoSample       *bool        `json:"do_sample,omitempty"`
	Stop           []string     `json:"stop,omitempty"`
	Thinking       *zaiThinking `json:"thinking,omitempty"`
	Tools          []zaiTool    `json:"tools,omitempty"`
	ToolChoice     string       `json:"tool_choice,omitempty"`
	ToolStream     *bool        `json:"tool_stream,omitempty"`
	ResponseFormat *zaiRespFmt  `json:"response_format,omitempty"`
	RequestID      string       `json:"request_id,omitempty"`
	UserID         string       `json:"user_id,omitempty"`
}

// zaiMessage is a message in the conversation.
type zaiMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []zaiToolCallReq `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
}

// zaiThinking controls the thinking/reasoning mode.
type zaiThinking struct {
	Type          string `json:"type"` // "enabled" or "disabled"
	ClearThinking *bool  `json:"clear_thinking,omitempty"`
}

// zaiTool defines a tool that can be called.
type zaiTool struct {
	Type     string      `json:"type"` // "function"
	Function zaiFunction `json:"function"`
}

// zaiFunction defines a function tool.
type zaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// zaiToolCallReq is a tool call in a request message.
type zaiToolCallReq struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // "function"
	Function zaiToolCallFunc `json:"function"`
}

// zaiToolCallFunc is the function part of a tool call request.
type zaiToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// zaiRespFmt specifies the response format.
type zaiRespFmt struct {
	Type string `json:"type"` // "text" or "json_object"
}

// Response types

// zaiResponse is the response from the Z.ai chat completions API.
type zaiResponse struct {
	ID        string      `json:"id"`
	RequestID string      `json:"request_id"`
	Created   int64       `json:"created"`
	Model     string      `json:"model"`
	Choices   []zaiChoice `json:"choices"`
	Usage     zaiUsage    `json:"usage"`
}

// zaiChoice is a choice in the response.
type zaiChoice struct {
	Index        int        `json:"index"`
	Message      zaiRespMsg `json:"message"`
	Delta        zaiRespMsg `json:"delta"`
	FinishReason string     `json:"finish_reason"`
}

// zaiRespMsg is a message in the response.
type zaiRespMsg struct {
	Role             string        `json:"role"`
	Content          string        `json:"content"`
	ReasoningContent string        `json:"reasoning_content"`
	ToolCalls        []zaiToolCall `json:"tool_calls"`
}

// zaiToolCall is a tool call in the response.
type zaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // "function"
	Index    int             `json:"index"`
	Function zaiFunctionCall `json:"function"`
}

// zaiFunctionCall contains the function call details.
type zaiFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// zaiUsage contains token usage information.
type zaiUsage struct {
	PromptTokens        int                   `json:"prompt_tokens"`
	CompletionTokens    int                   `json:"completion_tokens"`
	TotalTokens         int                   `json:"total_tokens"`
	PromptTokensDetails *zaiPromptTokenDetail `json:"prompt_tokens_details,omitempty"`
}

// zaiPromptTokenDetail contains details about prompt token usage.
type zaiPromptTokenDetail struct {
	CachedTokens int `json:"cached_tokens"`
}

// Streaming types

// zaiStreamChunk is a chunk in the streaming response.
type zaiStreamChunk struct {
	ID        string            `json:"id"`
	RequestID string            `json:"request_id"`
	Created   int64             `json:"created"`
	Model     string            `json:"model"`
	Choices   []zaiStreamChoice `json:"choices"`
	Usage     *zaiUsage         `json:"usage,omitempty"`
}

// zaiStreamChoice is a choice in a streaming chunk.
type zaiStreamChoice struct {
	Index        int            `json:"index"`
	Delta        zaiStreamDelta `json:"delta"`
	FinishReason string         `json:"finish_reason"`
}

// zaiStreamDelta contains the delta content in a streaming chunk.
type zaiStreamDelta struct {
	Role             string              `json:"role"`
	Content          string              `json:"content"`
	ReasoningContent string              `json:"reasoning_content"`
	ToolCalls        []zaiStreamToolCall `json:"tool_calls"`
}

// zaiStreamToolCall is a tool call delta in streaming.
type zaiStreamToolCall struct {
	Index    int                   `json:"index"`
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	Function zaiStreamFunctionCall `json:"function"`
}

// zaiStreamFunctionCall contains streaming function call details.
type zaiStreamFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
