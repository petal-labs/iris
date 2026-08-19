package huggingface

import (
	"encoding/json"

	"github.com/petal-labs/iris/core"
)

// mapMessages converts Iris messages to HF message format.
func mapMessages(msgs []core.Message) []hfMessage {
	result := make([]hfMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = hfMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// mapTools converts Iris tools to HF tool format.
func mapTools(irisTools []core.Tool) []hfTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]hfTool, len(irisTools))
	for i, t := range irisTools {
		params := t.Schema().JSONSchema

		// Default to empty object for no-parameter tools
		if len(params) == 0 {
			params = json.RawMessage(`{}`)
		}

		result[i] = hfTool{
			Type: "function",
			Function: hfFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		}
	}
	return result
}

// buildRequest creates an HF API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, model string, stream bool) *hfRequest {
	hfReq := &hfRequest{
		Model:    model,
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		hfReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		hfReq.MaxTokens = req.MaxTokens
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		hfReq.Tools = mapTools(req.Tools)
		hfReq.ToolChoice = "auto"
	}

	return hfReq
}

// mapResponse converts an HF response to an Iris ChatResponse.
func mapResponse(resp *hfResponse) (*core.ChatResponse, error) {
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

// mapToolCalls converts HF tool calls to Iris ToolCalls.
func mapToolCalls(calls []hfToolCall) ([]core.ToolCall, error) {
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
