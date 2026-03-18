package azurefoundry

import (
	"encoding/json"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// ----------------------------------------------------------------------------
// Request Types
// ----------------------------------------------------------------------------

// azureRequest represents a request to the Azure AI chat completions API.
type azureRequest struct {
	Model               string               `json:"model,omitempty"`
	Messages            []azureMessage       `json:"messages"`
	Temperature         *float32             `json:"temperature,omitempty"`
	TopP                *float32             `json:"top_p,omitempty"`
	MaxTokens           *int                 `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                 `json:"max_completion_tokens,omitempty"`
	Stop                []string             `json:"stop,omitempty"`
	Stream              bool                 `json:"stream"`
	Tools               []azureTool          `json:"tools,omitempty"`
	ToolChoice          any                  `json:"tool_choice,omitempty"`
	ResponseFormat      *azureResponseFormat `json:"response_format,omitempty"`
	Seed                *int                 `json:"seed,omitempty"`
	FrequencyPenalty    *float32             `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float32             `json:"presence_penalty,omitempty"`
	User                string               `json:"user,omitempty"`
}

// azureMessage represents a message in the Azure format.
type azureMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content,omitempty"` // string or []contentPart for multimodal
	Name       string          `json:"name,omitempty"`
	ToolCalls  []azureToolCall `json:"tool_calls,omitempty"`   // For assistant messages requesting tools
	ToolCallID string          `json:"tool_call_id,omitempty"` // For tool result messages
}

// azureTool represents a tool definition in the Azure format.
type azureTool struct {
	Type     string        `json:"type"` // "function"
	Function azureFunction `json:"function"`
}

// azureFunction represents a function definition for Azure tools.
type azureFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

// azureResponseFormat represents the response_format parameter.
type azureResponseFormat struct {
	Type       string           `json:"type"` // "text", "json_object", "json_schema"
	JSONSchema *azureJSONSchema `json:"json_schema,omitempty"`
}

// azureJSONSchema represents the JSON schema configuration for structured output.
type azureJSONSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema"`
	Strict      bool            `json:"strict,omitempty"`
}

// ----------------------------------------------------------------------------
// Response Types
// ----------------------------------------------------------------------------

// azureResponse represents a response from the Azure AI chat completions API.
type azureResponse struct {
	ID                  string              `json:"id"`
	Object              string              `json:"object"`
	Created             int64               `json:"created"`
	Model               string              `json:"model"`
	Choices             []azureChoice       `json:"choices"`
	Usage               azureUsage          `json:"usage"`
	SystemFingerprint   string              `json:"system_fingerprint,omitempty"`
	PromptFilterResults []azureFilterResult `json:"prompt_filter_results,omitempty"`
}

// azureChoice represents a single choice in an Azure response.
type azureChoice struct {
	Index                int                  `json:"index"`
	Message              *azureRespMsg        `json:"message,omitempty"`
	Delta                *azureRespMsg        `json:"delta,omitempty"` // For streaming
	FinishReason         string               `json:"finish_reason"`
	ContentFilterResults *azureContentFilters `json:"content_filter_results,omitempty"`
}

// azureRespMsg represents the assistant message in a response.
type azureRespMsg struct {
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCalls []azureToolCall `json:"tool_calls,omitempty"`
	Refusal   string          `json:"refusal,omitempty"`
}

// azureToolCall represents a tool call in an Azure response.
type azureToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function azureFunctionCall `json:"function"`
	Index    *int              `json:"index,omitempty"` // For streaming tool calls
}

// azureFunctionCall represents the function details in a tool call.
type azureFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// azureUsage represents token usage in an Azure response.
type azureUsage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	TotalTokens             int                     `json:"total_tokens"`
	CompletionTokensDetails *azureCompletionDetails `json:"completion_tokens_details,omitempty"`
	PromptTokensDetails     *azurePromptDetails     `json:"prompt_tokens_details,omitempty"`
}

// azureCompletionDetails contains detailed token usage for completions.
type azureCompletionDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
}

// azurePromptDetails contains detailed token usage for prompts.
type azurePromptDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
}

// ----------------------------------------------------------------------------
// Content Filtering Types
// ----------------------------------------------------------------------------

// azureFilterResult represents content filtering results for a prompt.
type azureFilterResult struct {
	PromptIndex          int                 `json:"prompt_index"`
	ContentFilterResults azureContentFilters `json:"content_filter_results"`
}

// azureContentFilters contains all content filter results.
type azureContentFilters struct {
	Hate                  *azureFilterSeverity `json:"hate,omitempty"`
	SelfHarm              *azureFilterSeverity `json:"self_harm,omitempty"`
	Sexual                *azureFilterSeverity `json:"sexual,omitempty"`
	Violence              *azureFilterSeverity `json:"violence,omitempty"`
	Jailbreak             *azureFilterDetected `json:"jailbreak,omitempty"`
	ProtectedMaterialText *azureFilterDetected `json:"protected_material_text,omitempty"`
	ProtectedMaterialCode *azureFilterDetected `json:"protected_material_code,omitempty"`
}

// azureFilterSeverity represents a severity-based filter result.
type azureFilterSeverity struct {
	Filtered bool   `json:"filtered"`
	Severity string `json:"severity"` // "safe", "low", "medium", "high"
}

// azureFilterDetected represents a detection-based filter result.
type azureFilterDetected struct {
	Filtered bool `json:"filtered"`
	Detected bool `json:"detected"`
}

// IsFiltered returns true if any content was filtered.
func (f *azureContentFilters) IsFiltered() bool {
	if f == nil {
		return false
	}
	return (f.Hate != nil && f.Hate.Filtered) ||
		(f.SelfHarm != nil && f.SelfHarm.Filtered) ||
		(f.Sexual != nil && f.Sexual.Filtered) ||
		(f.Violence != nil && f.Violence.Filtered) ||
		(f.Jailbreak != nil && f.Jailbreak.Filtered) ||
		(f.ProtectedMaterialText != nil && f.ProtectedMaterialText.Filtered) ||
		(f.ProtectedMaterialCode != nil && f.ProtectedMaterialCode.Filtered)
}

// ----------------------------------------------------------------------------
// Mapping Functions
// ----------------------------------------------------------------------------

// schemaProvider is an interface for tools that provide a JSON schema.
type schemaProvider interface {
	Schema() tools.ToolSchema
}

// mapMessages converts Iris messages to Azure message format.
func mapMessages(msgs []core.Message) []azureMessage {
	result := make([]azureMessage, 0, len(msgs))

	for _, msg := range msgs {
		switch msg.Role {
		case core.RoleTool:
			// Tool result messages: expand into individual messages per result
			for _, tr := range msg.ToolResults {
				content := marshalToolResultContent(tr.Content)
				result = append(result, azureMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: tr.CallID,
				})
			}

		case core.RoleAssistant:
			// Assistant messages may include tool calls
			azMsg := azureMessage{
				Role:    "assistant",
				Content: msg.Content,
			}
			if len(msg.ToolCalls) > 0 {
				azMsg.ToolCalls = mapToolCallsToAzure(msg.ToolCalls)
			}
			result = append(result, azMsg)

		default:
			// System, User messages
			result = append(result, azureMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	return result
}

// mapToolCallsToAzure converts Iris ToolCalls to Azure format.
func mapToolCallsToAzure(calls []core.ToolCall) []azureToolCall {
	result := make([]azureToolCall, len(calls))
	for i, tc := range calls {
		result[i] = azureToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: azureFunctionCall{
				Name:      tc.Name,
				Arguments: string(tc.Arguments),
			},
		}
	}
	return result
}

// marshalToolResultContent converts tool result content to a JSON string.
func marshalToolResultContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return `{"error": "failed to marshal tool result"}`
		}
		return string(data)
	}
}

// mapTools converts Iris tools to Azure tool format.
func mapTools(irisTools []core.Tool) []azureTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]azureTool, len(irisTools))
	for i, t := range irisTools {
		var params json.RawMessage

		// Check if the tool provides a schema
		if sp, ok := t.(schemaProvider); ok {
			params = sp.Schema().JSONSchema
		}

		// Default to empty object if no schema
		if params == nil {
			params = json.RawMessage(`{}`)
		}

		result[i] = azureTool{
			Type: "function",
			Function: azureFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		}
	}
	return result
}

// buildRequest creates an Azure API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, stream bool) *azureRequest {
	azReq := &azureRequest{
		Model:    string(req.Model),
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		azReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		azReq.MaxTokens = req.MaxTokens
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		azReq.Tools = mapTools(req.Tools)
		azReq.ToolChoice = "auto"
	}

	// Map response format for structured output
	azReq.ResponseFormat = mapResponseFormat(req)

	return azReq
}

// mapResponseFormat converts Iris response format to Azure format.
func mapResponseFormat(req *core.ChatRequest) *azureResponseFormat {
	switch req.ResponseFormat {
	case core.ResponseFormatJSON:
		return &azureResponseFormat{Type: "json_object"}
	case core.ResponseFormatJSONSchema:
		if req.JSONSchema == nil {
			return nil
		}
		return &azureResponseFormat{
			Type: "json_schema",
			JSONSchema: &azureJSONSchema{
				Name:        req.JSONSchema.Name,
				Description: req.JSONSchema.Description,
				Schema:      req.JSONSchema.Schema,
				Strict:      req.JSONSchema.Strict,
			},
		}
	default:
		// ResponseFormatText or empty: no response_format constraint
		return nil
	}
}

// ----------------------------------------------------------------------------
// Response Mapping
// ----------------------------------------------------------------------------

// mapResponse converts an Azure response to an Iris ChatResponse.
func mapResponse(resp *azureResponse) *core.ChatResponse {
	if resp == nil || len(resp.Choices) == 0 {
		return &core.ChatResponse{
			ID:    resp.ID,
			Model: core.ModelID(resp.Model),
		}
	}

	choice := resp.Choices[0]
	result := &core.ChatResponse{
		ID:    resp.ID,
		Model: core.ModelID(resp.Model),
		Usage: mapUsage(resp.Usage),
	}

	if choice.Message != nil {
		result.Output = choice.Message.Content
		result.ToolCalls = mapToolCallsFromAzure(choice.Message.ToolCalls)
	}

	return result
}

// mapUsage converts Azure token usage to Iris format.
func mapUsage(usage azureUsage) core.TokenUsage {
	return core.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

// mapToolCallsFromAzure converts Azure tool calls to Iris format.
func mapToolCallsFromAzure(calls []azureToolCall) []core.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	result := make([]core.ToolCall, len(calls))
	for i, tc := range calls {
		result[i] = core.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		}
	}
	return result
}
