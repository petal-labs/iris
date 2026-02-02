package xai

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

// mapMessages converts Iris messages to xAI message format.
func mapMessages(msgs []core.Message) []xaiMessage {
	result := make([]xaiMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = xaiMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// mapTools converts Iris tools to xAI tool format.
// Tools that implement schemaProvider will have their schema included.
func mapTools(irisTools []core.Tool) []xaiTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]xaiTool, len(irisTools))
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

		result[i] = xaiTool{
			Type: "function",
			Function: xaiFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		}
	}
	return result
}

// mapReasoningEffort converts Iris ReasoningEffort to xAI's reasoning_effort string.
// xAI only supports "low" and "high" values.
func mapReasoningEffort(effort core.ReasoningEffort) string {
	switch effort {
	case core.ReasoningEffortLow:
		return "low"
	case core.ReasoningEffortMedium, core.ReasoningEffortHigh, core.ReasoningEffortXHigh:
		// xAI only has low/high, map medium and above to high
		return "high"
	default:
		return ""
	}
}

// buildRequest creates an xAI API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, stream bool) *xaiRequest {
	xaiReq := &xaiRequest{
		Model:    string(req.Model),
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		xaiReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		xaiReq.MaxTokens = req.MaxTokens
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		xaiReq.Tools = mapTools(req.Tools)
		xaiReq.ToolChoice = "auto"
	}

	// Map reasoning effort if set
	if req.ReasoningEffort != "" && req.ReasoningEffort != core.ReasoningEffortNone {
		xaiReq.ReasoningEffort = mapReasoningEffort(req.ReasoningEffort)
	}

	return xaiReq
}

// mapResponse converts an xAI response to an Iris ChatResponse.
func mapResponse(resp *xaiResponse) (*core.ChatResponse, error) {
	result := &core.ChatResponse{
		ID:    resp.ID,
		Model: core.ModelID(resp.Model),
		Usage: core.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Extract content from first choice
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.Output = choice.Message.Content

		// Map reasoning content if present (grok-3-mini)
		if choice.Message.ReasoningContent != "" {
			result.Reasoning = &core.ReasoningOutput{
				Summary: []string{choice.Message.ReasoningContent},
			}
		}

		// Map tool calls if present
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls, err := mapToolCalls(choice.Message.ToolCalls)
			if err != nil {
				return nil, err
			}
			result.ToolCalls = toolCalls
		}
	}

	return result, nil
}

// mapToolCalls converts xAI tool calls to Iris ToolCalls.
func mapToolCalls(calls []xaiToolCall) ([]core.ToolCall, error) {
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
