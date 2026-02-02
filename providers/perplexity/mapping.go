package perplexity

import (
	"encoding/json"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// schemaProvider is an interface for tools that provide a JSON schema.
// This allows us to check if a core.Tool also implements the full tools.Tool interface.
type schemaProvider interface {
	Schema() tools.ToolSchema
}

// mapMessages converts Iris messages to Perplexity message format.
func mapMessages(msgs []core.Message) []perplexityMessage {
	result := make([]perplexityMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = perplexityMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// mapTools converts Iris tools to Perplexity tool format.
// Tools that implement schemaProvider will have their schema included.
func mapTools(irisTools []core.Tool) []perplexityTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]perplexityTool, len(irisTools))
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

		result[i] = perplexityTool{
			Type: "function",
			Function: perplexityFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		}
	}
	return result
}

// mapReasoningEffort converts Iris ReasoningEffort to Perplexity's reasoning_effort string.
// Perplexity supports: minimal, low, medium, high
func mapReasoningEffort(effort core.ReasoningEffort) string {
	switch effort {
	case core.ReasoningEffortLow:
		return "low"
	case core.ReasoningEffortMedium:
		return "medium"
	case core.ReasoningEffortHigh, core.ReasoningEffortXHigh:
		return "high"
	default:
		return ""
	}
}

// buildRequest creates a Perplexity API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, stream bool) *perplexityRequest {
	pReq := &perplexityRequest{
		Model:    string(req.Model),
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		pReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		pReq.MaxTokens = req.MaxTokens
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		pReq.Tools = mapTools(req.Tools)
		pReq.ToolChoice = "auto"
	}

	// Map reasoning effort if set
	if req.ReasoningEffort != "" && req.ReasoningEffort != core.ReasoningEffortNone {
		pReq.ReasoningEffort = mapReasoningEffort(req.ReasoningEffort)
	}

	return pReq
}

// mapResponse converts a Perplexity response to an Iris ChatResponse.
func mapResponse(resp *perplexityResponse) (*core.ChatResponse, error) {
	result := &core.ChatResponse{
		ID:    resp.ID,
		Model: core.ModelID(resp.Model),
	}

	// Map usage if present
	if resp.Usage != nil {
		result.Usage = core.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Extract content from first choice
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message != nil {
			result.Output = choice.Message.Content

			// Map tool calls if present
			if len(choice.Message.ToolCalls) > 0 {
				toolCalls, err := mapToolCalls(choice.Message.ToolCalls)
				if err != nil {
					return nil, err
				}
				result.ToolCalls = toolCalls
			}
		}
	}

	return result, nil
}

// mapToolCalls converts Perplexity tool calls to Iris ToolCalls.
func mapToolCalls(calls []perplexityToolCall) ([]core.ToolCall, error) {
	result := make([]core.ToolCall, len(calls))

	for i, call := range calls {
		// Validate that arguments is valid JSON
		if !json.Valid([]byte(call.Function.Arguments)) {
			return nil, ErrToolArgsInvalidJSON
		}

		result[i] = core.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(call.Function.Arguments),
		}
	}

	return result, nil
}
