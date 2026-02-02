package openai

import "encoding/json"

// Responses API request/response types for OpenAI.

// responsesRequest represents a request to the OpenAI Responses API.
type responsesRequest struct {
	Model              string                   `json:"model"`
	Input              responsesInput           `json:"input"`
	Instructions       string                   `json:"instructions,omitempty"`
	MaxOutputTokens    *int                     `json:"max_output_tokens,omitempty"`
	Temperature        *float32                 `json:"temperature,omitempty"`
	Tools              []responsesTool          `json:"tools,omitempty"`
	ToolResources      *responsesToolResources  `json:"tool_resources,omitempty"`
	Reasoning          *responsesReasoningParam `json:"reasoning,omitempty"`
	PreviousResponseID string                   `json:"previous_response_id,omitempty"`
	Truncation         string                   `json:"truncation,omitempty"`
	Stream             bool                     `json:"stream,omitempty"`
	StreamOptions      *streamOptions           `json:"stream_options,omitempty"`
}

// responsesToolResources contains configuration for built-in tools.
type responsesToolResources struct {
	FileSearch *responsesFileSearchResources `json:"file_search,omitempty"`
}

// responsesFileSearchResources contains vector store IDs for file search.
type responsesFileSearchResources struct {
	VectorStoreIDs []string `json:"vector_store_ids"`
}

// streamOptions configures streaming behavior.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// responsesReasoningParam configures reasoning behavior.
type responsesReasoningParam struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"` // "auto", "concise", "detailed"
}

// responsesInput can be either a string or an array of messages.
// Custom marshaling handles both cases.
type responsesInput struct {
	Text     string
	Messages []responsesInputMessage
}

// MarshalJSON implements custom marshaling for responsesInput.
// If Text is set, marshals as a string. Otherwise, marshals Messages as an array.
func (r responsesInput) MarshalJSON() ([]byte, error) {
	if r.Text != "" {
		return json.Marshal(r.Text)
	}
	return json.Marshal(r.Messages)
}

// responsesContent can be either a string or an array of content parts.
// Custom marshaling handles both cases.
type responsesContent struct {
	Text  string                 // Simple text content
	Parts []responsesContentPart // Multimodal content parts
}

// MarshalJSON implements custom marshaling for responsesContent.
// If Parts is set, marshals as array. Otherwise marshals Text as string.
func (c responsesContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return json.Marshal(c.Parts)
	}
	return json.Marshal(c.Text)
}

// UnmarshalJSON implements custom unmarshaling for responsesContent.
// Handles both string and array formats.
func (c *responsesContent) UnmarshalJSON(data []byte) error {
	// Try string first
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		c.Text = text
		c.Parts = nil
		return nil
	}

	// Try array of parts
	var parts []responsesContentPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	c.Parts = parts
	c.Text = ""
	return nil
}

// responsesInputMessage represents a message in the Responses API input.
// Content can be simple text or multimodal content parts.
type responsesInputMessage struct {
	Role    string           `json:"role"`
	Content responsesContent `json:"content"`
}

// responsesContentPart represents a content part in a Responses API input message.
// Type determines which fields are used.
type responsesContentPart struct {
	Type string `json:"type"`

	// For input_text
	Text string `json:"text,omitempty"`

	// For input_image
	ImageURL string `json:"image_url,omitempty"` // URL or data URL
	Detail   string `json:"detail,omitempty"`    // auto, low, high

	// For input_image and input_file
	FileID string `json:"file_id,omitempty"`

	// For input_file
	FileURL  string `json:"file_url,omitempty"`
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// responsesTool represents a tool in the Responses API.
type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// responsesResponse represents a response from the OpenAI Responses API.
type responsesResponse struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	CreatedAt         int64             `json:"created_at"`
	Model             string            `json:"model"`
	Status            string            `json:"status"`
	Output            []responsesOutput `json:"output"`
	OutputText        string            `json:"output_text,omitempty"`
	Usage             *responsesUsage   `json:"usage,omitempty"`
	Error             *responsesError   `json:"error,omitempty"`
	IncompleteDetails *incompleteInfo   `json:"incomplete_details,omitempty"`
}

// incompleteInfo provides details when a response is incomplete.
type incompleteInfo struct {
	Reason string `json:"reason,omitempty"`
}

// responsesOutput represents an output item in a Responses API response.
// The Type field determines which other fields are populated.
type responsesOutput struct {
	Type   string `json:"type"` // "reasoning", "message", "function_call"
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
	Role   string `json:"role,omitempty"`

	// For reasoning type
	Summary []responsesReasoningSummary `json:"summary,omitempty"`

	// For message type
	Content []responsesMessageContent `json:"content,omitempty"`

	// For function_call type
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// responsesReasoningSummary contains a summary of reasoning.
type responsesReasoningSummary struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// responsesMessageContent represents content in a message output.
type responsesMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// responsesUsage tracks token usage for a Responses API request.
type responsesUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	TotalTokens     int `json:"total_tokens"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// responsesError represents an error in the Responses API.
type responsesError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Streaming event types for the Responses API.

// responsesStreamEvent represents a streaming event from the Responses API.
type responsesStreamEvent struct {
	Type     string          `json:"type"`
	Response json.RawMessage `json:"response,omitempty"`
	Item     json.RawMessage `json:"item,omitempty"`
	Delta    json.RawMessage `json:"delta,omitempty"`
	// For content delta
	ContentIndex int    `json:"content_index,omitempty"`
	OutputIndex  int    `json:"output_index,omitempty"`
	ItemID       string `json:"item_id,omitempty"`
}

// responsesContentDelta represents a content delta in streaming.
type responsesContentDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// responsesFunctionCallDelta represents a function call delta in streaming.
type responsesFunctionCallDelta struct {
	Arguments string `json:"arguments,omitempty"`
}
