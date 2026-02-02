package openai

import "encoding/json"

// openAIRequest represents a request to the OpenAI chat completions API.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature *float32        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  string          `json:"tool_choice,omitempty"`
}

// openAIMessage represents a message in the OpenAI format.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAITool represents a tool definition in the OpenAI format.
type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

// openAIFunction represents a function definition for OpenAI tools.
type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// openAIResponse represents a response from the OpenAI chat completions API.
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

// openAIChoice represents a single choice in an OpenAI response.
type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIRespMsg `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// openAIRespMsg represents the assistant message in a response.
type openAIRespMsg struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

// openAIToolCall represents a tool call in an OpenAI response.
type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

// openAIFunctionCall represents the function details in a tool call.
type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAIUsage represents token usage in an OpenAI response.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
