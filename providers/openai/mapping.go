package openai

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

// mapMessages converts Iris messages to OpenAI message format.
func mapMessages(msgs []core.Message) []openAIMessage {
	result := make([]openAIMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = openAIMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// mapTools converts Iris tools to OpenAI tool format.
// Tools that implement schemaProvider will have their schema included.
func mapTools(irisTools []core.Tool) []openAITool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]openAITool, len(irisTools))
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

	return oaiReq
}
