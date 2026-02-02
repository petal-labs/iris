package perplexity

import "encoding/json"

// perplexityRequest represents a request to the Perplexity chat completions API.
type perplexityRequest struct {
	Model    string              `json:"model"`
	Messages []perplexityMessage `json:"messages"`
	Stream   bool                `json:"stream,omitempty"`

	// Standard OpenAI-compatible fields
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	Temperature *float32 `json:"temperature,omitempty"`

	// Tool calling
	Tools      []perplexityTool `json:"tools,omitempty"`
	ToolChoice any              `json:"tool_choice,omitempty"`

	// Perplexity-specific: Web Search Options
	WebSearchOptions *WebSearchOptions `json:"web_search_options,omitempty"`

	// Perplexity-specific: Search Filters (top-level for backward compat)
	SearchDomainFilter      []string `json:"search_domain_filter,omitempty"`
	SearchRecencyFilter     string   `json:"search_recency_filter,omitempty"`      // hour|day|week|month|year
	SearchAfterDateFilter   string   `json:"search_after_date_filter,omitempty"`   // %m/%d/%Y format
	SearchBeforeDateFilter  string   `json:"search_before_date_filter,omitempty"`  // %m/%d/%Y format
	LastUpdatedAfterFilter  string   `json:"last_updated_after_filter,omitempty"`  // %m/%d/%Y format
	LastUpdatedBeforeFilter string   `json:"last_updated_before_filter,omitempty"` // %m/%d/%Y format

	// Location-based search
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Country   string   `json:"country,omitempty"`

	// Output control
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	// Search mode
	SearchMode string `json:"search_mode,omitempty"` // web|academic|sec

	// Reasoning
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // minimal|low|medium|high

	// Results control
	ReturnImages           *bool `json:"return_images,omitempty"`
	ReturnRelatedQuestions *bool `json:"return_related_questions,omitempty"`
	NumSearchResults       *int  `json:"num_search_results,omitempty"` // default 10
}

// perplexityMessage represents a message in the Perplexity format.
type perplexityMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// perplexityTool represents a tool definition in the Perplexity format.
type perplexityTool struct {
	Type     string             `json:"type"`
	Function perplexityFunction `json:"function"`
}

// perplexityFunction represents a function definition for Perplexity tools.
type perplexityFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// WebSearchOptions configures Pro Search behavior.
type WebSearchOptions struct {
	SearchType string `json:"search_type,omitempty"` // fast|pro|auto
}

// ResponseFormat for structured outputs.
type ResponseFormat struct {
	Type       string      `json:"type"` // text|json_schema
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema for structured output schema.
type JSONSchema struct {
	Name   string                 `json:"name,omitempty"`
	Schema map[string]interface{} `json:"schema"`
}

// perplexityResponse represents a response from the Perplexity chat completions API.
type perplexityResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Created int64              `json:"created"`
	Object  string             `json:"object"` // "chat.completion"
	Choices []perplexityChoice `json:"choices"`
	Usage   *perplexityUsage   `json:"usage,omitempty"`

	// Perplexity-specific
	Citations     []string       `json:"citations,omitempty"`
	SearchResults []SearchResult `json:"search_results,omitempty"`
}

// perplexityChoice represents a single choice in a Perplexity response.
type perplexityChoice struct {
	Index        int                `json:"index"`
	Message      *perplexityRespMsg `json:"message,omitempty"`
	Delta        *perplexityRespMsg `json:"delta,omitempty"`
	FinishReason string             `json:"finish_reason,omitempty"`
}

// perplexityRespMsg represents the assistant message in a response.
type perplexityRespMsg struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	ToolCalls []perplexityToolCall `json:"tool_calls,omitempty"`
}

// perplexityToolCall represents a tool call in a Perplexity response.
type perplexityToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function perplexityFunctionCall `json:"function"`
}

// perplexityFunctionCall represents the function details in a tool call.
type perplexityFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SearchResult represents a search result from Perplexity.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Date        string `json:"date,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
	Snippet     string `json:"snippet"`
	Source      string `json:"source"` // "web"
}

// perplexityUsage represents token usage in a Perplexity response.
type perplexityUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CitationTokens   int `json:"citation_tokens,omitempty"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

// Streaming response types for Perplexity SSE protocol.

// perplexityStreamChunk represents a single chunk in a Perplexity streaming response.
type perplexityStreamChunk struct {
	ID      string                   `json:"id"`
	Model   string                   `json:"model"`
	Choices []perplexityStreamChoice `json:"choices"`
	Usage   *perplexityUsage         `json:"usage,omitempty"`

	// Perplexity-specific - may appear in final chunk
	Citations     []string       `json:"citations,omitempty"`
	SearchResults []SearchResult `json:"search_results,omitempty"`
}

// perplexityStreamChoice represents a single choice in a streaming chunk.
type perplexityStreamChoice struct {
	Index        int                   `json:"index"`
	Delta        perplexityStreamDelta `json:"delta"`
	FinishReason *string               `json:"finish_reason,omitempty"`
}

// perplexityStreamDelta represents the delta content in a streaming chunk.
type perplexityStreamDelta struct {
	Role      string                     `json:"role,omitempty"`
	Content   string                     `json:"content,omitempty"`
	ToolCalls []perplexityStreamToolCall `json:"tool_calls,omitempty"`
}

// perplexityStreamToolCall represents a tool call fragment in a streaming chunk.
type perplexityStreamToolCall struct {
	Index    int                      `json:"index"`
	ID       string                   `json:"id,omitempty"`
	Type     string                   `json:"type,omitempty"`
	Function perplexityStreamFunction `json:"function,omitempty"`
}

// perplexityStreamFunction represents a function fragment in a streaming tool call.
type perplexityStreamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
