package zai

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

// mapMessages converts Iris messages to Z.ai message format.
func mapMessages(msgs []core.Message) []zaiMessage {
	result := make([]zaiMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = zaiMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// mapTools converts Iris tools to Z.ai tool format.
// Tools that implement schemaProvider will have their schema included.
func mapTools(irisTools []core.Tool) []zaiTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]zaiTool, len(irisTools))
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

		result[i] = zaiTool{
			Type: "function",
			Function: zaiFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		}
	}
	return result
}

// mapThinking converts Iris ReasoningEffort to Z.ai thinking parameter.
// Z.ai uses thinking.type "enabled" or "disabled".
func mapThinking(effort core.ReasoningEffort) *zaiThinking {
	switch effort {
	case core.ReasoningEffortNone, "":
		return nil
	case core.ReasoningEffortLow, core.ReasoningEffortMedium, core.ReasoningEffortHigh, core.ReasoningEffortXHigh:
		// Any reasoning effort enables thinking
		return &zaiThinking{
			Type: "enabled",
		}
	default:
		return nil
	}
}

// buildRequest creates a Z.ai API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, stream bool) *zaiRequest {
	zaiReq := &zaiRequest{
		Model:    string(req.Model),
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		zaiReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		zaiReq.MaxTokens = req.MaxTokens
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		zaiReq.Tools = mapTools(req.Tools)
		zaiReq.ToolChoice = "auto"
	}

	// Map reasoning effort to thinking parameter
	if req.ReasoningEffort != "" && req.ReasoningEffort != core.ReasoningEffortNone {
		zaiReq.Thinking = mapThinking(req.ReasoningEffort)
	}

	return zaiReq
}

// mapResponse converts a Z.ai response to an Iris ChatResponse.
func mapResponse(resp *zaiResponse) (*core.ChatResponse, error) {
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

		// Map reasoning content if present
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

// mapToolCalls converts Z.ai tool calls to Iris ToolCalls.
// Z.ai returns arguments as a JSON object, not a string.
func mapToolCalls(calls []zaiToolCall) ([]core.ToolCall, error) {
	result := make([]core.ToolCall, len(calls))

	for i, call := range calls {
		// Arguments are already JSON in Z.ai response
		args := call.Function.Arguments
		if args == nil {
			args = json.RawMessage(`{}`)
		}

		// Validate that arguments is valid JSON
		if !json.Valid(args) {
			return nil, ErrToolArgsInvalidJSON
		}

		result[i] = core.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		}
	}

	return result, nil
}
