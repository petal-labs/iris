package openai

import (
	"encoding/json"

	"github.com/petal-labs/iris/core"
)

// mapMessages converts Iris messages to OpenAI message format.
func mapMessages(msgs []core.Message) []openAIMessage {
	result := make([]openAIMessage, 0, len(msgs))

	for _, msg := range msgs {
		switch msg.Role {
		case core.RoleTool:
			// Tool result messages: expand into individual messages per result
			for _, tr := range msg.ToolResults {
				content := marshalToolResultContent(tr.Content)
				result = append(result, openAIMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: tr.CallID,
				})
			}

		case core.RoleAssistant:
			// Assistant messages may include tool calls
			oaiMsg := openAIMessage{
				Role:    "assistant",
				Content: msg.Content,
			}
			if len(msg.ToolCalls) > 0 {
				oaiMsg.ToolCalls = mapToolCallsToOpenAI(msg.ToolCalls)
			}
			result = append(result, oaiMsg)

		default:
			// System, User messages
			result = append(result, openAIMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	return result
}

// mapToolCallsToOpenAI converts Iris ToolCalls to OpenAI format.
func mapToolCallsToOpenAI(calls []core.ToolCall) []openAIToolCall {
	result := make([]openAIToolCall, len(calls))
	for i, tc := range calls {
		result[i] = openAIToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: openAIFunctionCall{
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
			return "{\"error\": \"failed to marshal tool result\"}"
		}
		return string(data)
	}
}

// mapTools converts Iris tools to OpenAI tool format.
// A tool with an empty schema (no parameters) is transmitted with an empty
// object schema.
func mapTools(irisTools []core.Tool) []openAITool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]openAITool, len(irisTools))
	for i, t := range irisTools {
		params := t.Schema().JSONSchema

		// Default to empty object for no-parameter tools
		if len(params) == 0 {
			params = json.RawMessage(`{}`)
		}

		result[i] = openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		}
	}
	return result
}

// buildRequest creates an OpenAI API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, stream bool) *openAIRequest {
	oaiReq := &openAIRequest{
		Model:    string(req.Model),
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		oaiReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		oaiReq.MaxTokens = req.MaxTokens
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		oaiReq.Tools = mapTools(req.Tools)
		oaiReq.ToolChoice = "auto"
	}

	// Map response format for structured output
	oaiReq.ResponseFormat = mapResponseFormat(req)

	return oaiReq
}

// mapResponseFormat converts Iris response format to OpenAI format.
func mapResponseFormat(req *core.ChatRequest) *openAIResponseFormat {
	switch req.ResponseFormat {
	case core.ResponseFormatJSON:
		return &openAIResponseFormat{Type: "json_object"}
	case core.ResponseFormatJSONSchema:
		if req.JSONSchema == nil {
			return nil
		}
		return &openAIResponseFormat{
			Type: "json_schema",
			JSONSchema: &openAIJSONSchema{
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
