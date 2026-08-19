package anthropic

import (
	"encoding/json"
	"strings"

	"github.com/petal-labs/iris/core"
)

// defaultMaxTokens is the default max_tokens value when not specified.
// Anthropic requires max_tokens, so we provide a reasonable default.
const defaultMaxTokens = 1024

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

		case core.RoleTool:
			// Tool result messages: create a user message with tool_result content blocks
			content := make([]anthropicContentBlock, 0, len(msg.ToolResults))
			for _, tr := range msg.ToolResults {
				content = append(content, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: tr.CallID,
					Content:   marshalToolResultContent(tr.Content),
					IsError:   tr.IsError,
				})
			}
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: content,
			})

		case core.RoleAssistant:
			// Assistant messages may include tool calls (tool_use blocks)
			var content []anthropicContentBlock

			// Add text content if present
			if msg.Content != "" {
				content = append(content, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}

			// Add tool_use blocks for tool calls
			for _, tc := range msg.ToolCalls {
				content = append(content, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Arguments,
				})
			}

			// Only add message if there's content
			if len(content) > 0 {
				messages = append(messages, anthropicMessage{
					Role:    "assistant",
					Content: content,
				})
			}

		case core.RoleUser:
			content := mapUserContent(msg)
			if len(content) == 0 {
				continue
			}
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: content,
			})
		}
	}

	// Concatenate system messages with double newlines
	if len(systemParts) > 0 {
		system = strings.Join(systemParts, "\n\n")
	}

	return system, messages
}

func mapUserContent(msg core.Message) []anthropicContentBlock {
	if len(msg.Parts) == 0 {
		return []anthropicContentBlock{{Type: "text", Text: msg.Content}}
	}

	content := make([]anthropicContentBlock, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		if block, ok := mapContentPart(part); ok {
			content = append(content, block)
		}
	}
	return content
}

func mapContentPart(part core.ContentPart) (anthropicContentBlock, bool) {
	switch value := part.(type) {
	case core.InputText:
		return anthropicContentBlock{Type: "text", Text: value.Text}, true
	case *core.InputText:
		if value != nil {
			return anthropicContentBlock{Type: "text", Text: value.Text}, true
		}
	case core.InputImage:
		return mapInputImage(value)
	case *core.InputImage:
		if value != nil {
			return mapInputImage(*value)
		}
	}
	return anthropicContentBlock{}, false
}

func mapInputImage(image core.InputImage) (anthropicContentBlock, bool) {
	if image.ImageURL == "" || image.FileID != "" {
		return anthropicContentBlock{}, false
	}
	if strings.HasPrefix(image.ImageURL, "data:") {
		mediaType, data, ok := parseImageDataURL(image.ImageURL)
		if !ok {
			return anthropicContentBlock{}, false
		}
		return anthropicContentBlock{
			Type: "image",
			Source: &anthropicImageSource{
				Type:      "base64",
				MediaType: mediaType,
				Data:      data,
			},
		}, true
	}
	return anthropicContentBlock{
		Type: "image",
		Source: &anthropicImageSource{
			Type: "url",
			URL:  image.ImageURL,
		},
	}, true
}

func parseImageDataURL(value string) (mediaType, data string, ok bool) {
	if !strings.HasPrefix(value, "data:") {
		return "", "", false
	}
	metadata, data, found := strings.Cut(strings.TrimPrefix(value, "data:"), ",")
	if !found || data == "" {
		return "", "", false
	}
	mediaType, encoding, found := strings.Cut(metadata, ";")
	if !found || encoding != "base64" || !strings.HasPrefix(mediaType, "image/") {
		return "", "", false
	}
	return mediaType, data, true
}

// marshalToolResultContent converts tool result content to a string.
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

// mapTools converts Iris tools to Anthropic tool format.
// A tool with an empty schema (no parameters) is transmitted with an empty
// object schema.
func mapTools(irisTools []core.Tool) []anthropicTool {
	if len(irisTools) == 0 {
		return nil
	}

	result := make([]anthropicTool, len(irisTools))
	for i, t := range irisTools {
		inputSchema := t.Schema().JSONSchema

		// Default to empty object for no-parameter tools
		if len(inputSchema) == 0 {
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
