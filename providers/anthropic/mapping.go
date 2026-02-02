package anthropic

import (
	"encoding/json"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// defaultMaxTokens is the default max_tokens value when not specified.
// Anthropic requires max_tokens, so we provide a reasonable default.
const defaultMaxTokens = 1024

// schemaProvider is an interface for tools that provide a JSON schema.
// This allows us to check if a core.Tool also implements the full tools.Tool interface.
type schemaProvider interface {
	Schema() tools.ToolSchema
}

// buildRequest creates an Anthropic API request from an Iris ChatRequest.
func buildRequest(req *core.ChatRequest, stream bool) *anthropicRequest {
	system, messages := mapMessages(req.Messages)

	maxTokens := defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	antReq := &anthropicRequest{
		Model:     string(req.Model),
		Messages:  messages,
		MaxTokens: maxTokens,
		System:    system,
		Stream:    stream,
	}

	// Only set optional fields if provided
	if req.Temperature != nil {
		antReq.Temperature = req.Temperature
	}

	// Map tools if present
	if len(req.Tools) > 0 {
		antReq.Tools = mapTools(req.Tools)
		antReq.ToolChoice = map[string]string{"type": "auto"}
	}

	return antReq
}

// mapMessages converts Iris messages to Anthropic format.
// It extracts system messages into a single string and converts
// user/assistant messages to the Anthropic content block format.
func mapMessages(msgs []core.Message) (system string, messages []anthropicMessage) {
	var systemParts []string

	for _, msg := range msgs {
		switch msg.Role {
		case core.RoleSystem:
			systemParts = append(systemParts, msg.Content)
		case core.RoleUser, core.RoleAssistant:
			messages = append(messages, anthropicMessage{
				Role: string(msg.Role),
				Content: []anthropicContentBlock{
					{
						Type: "text",
						Text: msg.Content,
					},
				},
			})
		}
	}

	// Concatenate system messages with double newlines
	if len(systemParts) > 0 {
		system = strings.Join(systemParts, "\n\n")
	}

	return system, messages
}

// mapTools converts Iris tools to Anthropic tool format.
// Tools that implement schemaProvider will have their schema included.
func mapTools(irisTools []core.Tool) []anthropicTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]anthropicTool, len(irisTools))
	for i, t := range irisTools {
		var inputSchema json.RawMessage

		// Check if the tool provides a schema
		if sp, ok := t.(schemaProvider); ok {
			inputSchema = sp.Schema().JSONSchema
		}

		// Default to empty object if no schema
		if inputSchema == nil {
			inputSchema = json.RawMessage(`{}`)
		}

		result[i] = anthropicTool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: inputSchema,
		}
	}
	return result
}

// mapResponse converts an Anthropic response to an Iris ChatResponse.
func mapResponse(resp *anthropicResponse) (*core.ChatResponse, error) {
	result := &core.ChatResponse{
		ID:    resp.ID,
		Model: core.ModelID(resp.Model),
		Usage: core.TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	// Extract text and tool calls from content blocks
	var textParts []string
	var toolCalls []core.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			// Validate that input is valid JSON
			if !json.Valid(block.Input) {
				return nil, ErrToolArgsInvalidJSON
			}
			toolCalls = append(toolCalls, core.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	result.Output = strings.Join(textParts, "")
	result.ToolCalls = toolCalls

	return result, nil
}
