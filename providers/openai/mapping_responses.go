package openai

import (
	"encoding/json"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// buildResponsesRequest creates a Responses API request from an Iris ChatRequest.
func buildResponsesRequest(req *core.ChatRequest, stream bool) *responsesRequest {
	respReq := &responsesRequest{
		Model:  string(req.Model),
		Input:  buildResponsesInput(req.Messages, req.Instructions),
		Stream: stream,
	}

	// Set instructions if provided (takes precedence over system messages in input)
	if req.Instructions != "" {
		respReq.Instructions = req.Instructions
	}

	// Set optional parameters
	if req.Temperature != nil {
		respReq.Temperature = req.Temperature
	}

	if req.MaxTokens != nil {
		respReq.MaxOutputTokens = req.MaxTokens
	}

	// Set reasoning parameters if provided
	if req.ReasoningEffort != "" {
		respReq.Reasoning = &responsesReasoningParam{
			Effort:  string(req.ReasoningEffort),
			Summary: "auto",
		}
	}

	// Set previous response ID for chaining
	if req.PreviousResponseID != "" {
		respReq.PreviousResponseID = req.PreviousResponseID
	}

	// Set truncation mode
	if req.Truncation != "" {
		respReq.Truncation = req.Truncation
	}

	// Map tools (both custom and built-in)
	respReq.Tools = mapResponsesTools(req.Tools, req.BuiltInTools)

	// Map tool resources
	if req.ToolResources != nil && req.ToolResources.FileSearch != nil {
		respReq.ToolResources = &responsesToolResources{
			FileSearch: &responsesFileSearchResources{
				VectorStoreIDs: req.ToolResources.FileSearch.VectorStoreIDs,
			},
		}
	}

	// Enable usage reporting for streaming
	if stream {
		respReq.StreamOptions = &streamOptions{
			IncludeUsage: true,
		}
	}

	return respReq
}

// buildResponsesInput creates the input for a Responses API request.
// It converts Iris messages to the Responses API format.
// If instructions are provided separately, system messages are filtered out.
func buildResponsesInput(msgs []core.Message, instructions string) responsesInput {
	if len(msgs) == 0 {
		return responsesInput{}
	}

	// If there's only one user message with no system messages, no instructions,
	// and no multimodal parts, we can use the simple text format.
	if len(msgs) == 1 && msgs[0].Role == core.RoleUser &&
		instructions == "" && len(msgs[0].Parts) == 0 {
		return responsesInput{Text: msgs[0].Content}
	}

	// Convert to message array format
	messages := make([]responsesInputMessage, 0, len(msgs))
	for _, msg := range msgs {
		// If instructions are provided separately, skip system messages
		// (they should be in the instructions field instead)
		if instructions != "" && msg.Role == core.RoleSystem {
			continue
		}

		role := string(msg.Role)
		// Responses API uses "developer" instead of "system" for system messages
		if msg.Role == core.RoleSystem {
			role = "developer"
		}

		// Handle multimodal content
		if len(msg.Parts) > 0 {
			parts := make([]responsesContentPart, 0, len(msg.Parts))
			for _, part := range msg.Parts {
				parts = append(parts, mapContentPart(part))
			}
			messages = append(messages, responsesInputMessage{
				Role:    role,
				Content: responsesContent{Parts: parts},
			})
		} else {
			messages = append(messages, responsesInputMessage{
				Role:    role,
				Content: responsesContent{Text: msg.Content},
			})
		}
	}

	return responsesInput{Messages: messages}
}

// mapContentPart converts a core.ContentPart to a responsesContentPart.
func mapContentPart(part core.ContentPart) responsesContentPart {
	switch p := part.(type) {
	case *core.InputText:
		return responsesContentPart{
			Type: "input_text",
			Text: p.Text,
		}
	case *core.InputImage:
		cp := responsesContentPart{
			Type:     "input_image",
			ImageURL: p.ImageURL,
			FileID:   p.FileID,
		}
		if p.Detail != "" {
			cp.Detail = string(p.Detail)
		}
		return cp
	case *core.InputFile:
		return responsesContentPart{
			Type:     "input_file",
			FileID:   p.FileID,
			FileURL:  p.FileURL,
			FileData: p.FileData,
			Filename: p.Filename,
		}
	default:
		return responsesContentPart{}
	}
}

// mapResponsesTools converts Iris tools and built-in tools to Responses API format.
func mapResponsesTools(irisTools []core.Tool, builtInTools []core.BuiltInTool) []responsesTool {
	var result []responsesTool

	// Add built-in tools first
	for _, t := range builtInTools {
		result = append(result, responsesTool{
			Type: t.Type,
		})
	}

	// Add custom function tools
	for _, t := range irisTools {
		var params json.RawMessage

		// Check if the tool provides a schema
		if sp, ok := t.(schemaProvider); ok {
			params = sp.Schema().JSONSchema
		}

		// Default to empty object if no schema
		if params == nil {
			params = json.RawMessage(`{}`)
		}

		result = append(result, responsesTool{
			Type:        "function",
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  params,
		})
	}

	return result
}

// schemaProvider interface is already defined in mapping.go
// We use tools.ToolSchema for the schema type.
var _ schemaProvider = (tools.Tool)(nil)

// mapResponsesResponse converts a Responses API response to an Iris ChatResponse.
func mapResponsesResponse(resp *responsesResponse) (*core.ChatResponse, error) {
	result := &core.ChatResponse{
		ID:     resp.ID,
		Model:  core.ModelID(resp.Model),
		Status: resp.Status,
	}

	// Map usage
	if resp.Usage != nil {
		result.Usage = core.TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Use output_text if available (simpler path)
	if resp.OutputText != "" {
		result.Output = resp.OutputText
	}

	// Process output items
	var toolCalls []core.ToolCall
	var reasoningSummaries []string

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			// Extract text content from message
			for _, content := range item.Content {
				if content.Type == "output_text" || content.Type == "text" {
					if result.Output == "" {
						result.Output = content.Text
					} else {
						result.Output += content.Text
					}
				}
			}

		case "reasoning":
			// Extract reasoning summary
			for _, summary := range item.Summary {
				if summary.Text != "" {
					reasoningSummaries = append(reasoningSummaries, summary.Text)
				}
			}

		case "function_call":
			// Extract function call
			if !json.Valid([]byte(item.Arguments)) {
				return nil, ErrToolArgsInvalidJSON
			}
			toolCalls = append(toolCalls, core.ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: json.RawMessage(item.Arguments),
			})
		}
	}

	// Set tool calls if any
	if len(toolCalls) > 0 {
		result.ToolCalls = toolCalls
	}

	// Set reasoning output if any
	if len(reasoningSummaries) > 0 {
		result.Reasoning = &core.ReasoningOutput{
			Summary: reasoningSummaries,
		}
	}

	return result, nil
}
