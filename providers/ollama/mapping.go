package ollama

import (
	"encoding/json"
	"fmt"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// mapRequest converts a core.ChatRequest to an ollamaRequest.
func mapRequest(req *core.ChatRequest, stream bool) *ollamaRequest {
	ollamaReq := &ollamaRequest{
		Model:    string(req.Model),
		Messages: mapMessages(req.Messages),
		Stream:   stream,
	}

	// Map tools
	if len(req.Tools) > 0 {
		ollamaReq.Tools = mapTools(req.Tools)
	}

	// Map thinking/reasoning
	if think := mapThinking(req.ReasoningEffort); think != nil {
		ollamaReq.Think = think
	}

	// Map options (temperature, max tokens, etc.)
	if opts := mapOptions(req); opts != nil {
		ollamaReq.Options = opts
	}

	return ollamaReq
}

// mapMessages converts core messages to Ollama messages.
func mapMessages(messages []core.Message) []ollamaMessage {
	result := make([]ollamaMessage, 0, len(messages))

	for _, msg := range messages {
		ollamaMsg := ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}

		result = append(result, ollamaMsg)
	}

	return result
}

// schemaProvider is an interface for tools that provide a JSON schema.
type schemaProvider interface {
	Schema() tools.ToolSchema
}

// mapTools converts core tools to Ollama tools.
func mapTools(coreTools []core.Tool) []ollamaTool {
	if len(coreTools) == 0 {
		return nil
	}

	result := make([]ollamaTool, 0, len(coreTools))

	for _, t := range coreTools {
		var params map[string]interface{}

		// Check if the tool provides a schema
		if sp, ok := t.(schemaProvider); ok {
			schema := sp.Schema()
			if len(schema.JSONSchema) > 0 {
				if err := json.Unmarshal(schema.JSONSchema, &params); err != nil {
					params = map[string]interface{}{}
				}
			}
		}

		// Default to empty object if no params
		if params == nil {
			params = map[string]interface{}{}
		}

		result = append(result, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		})
	}

	return result
}

// mapThinking converts ReasoningEffort to Ollama's think parameter.
func mapThinking(effort core.ReasoningEffort) *bool {
	switch effort {
	case core.ReasoningEffortNone, "":
		return nil
	case core.ReasoningEffortLow, core.ReasoningEffortMedium, core.ReasoningEffortHigh, core.ReasoningEffortXHigh:
		think := true
		return &think
	default:
		return nil
	}
}

// mapOptions converts request parameters to Ollama options.
func mapOptions(req *core.ChatRequest) *ollamaOptions {
	opts := &ollamaOptions{}
	hasOpts := false

	if req.Temperature != nil && *req.Temperature > 0 {
		opts.Temperature = *req.Temperature
		hasOpts = true
	}

	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		opts.NumPredict = *req.MaxTokens
		hasOpts = true
	}

	if !hasOpts {
		return nil
	}

	return opts
}

// mapResponse converts an Ollama response to a core.ChatResponse.
func mapResponse(resp *ollamaResponse) *core.ChatResponse {
	chatResp := &core.ChatResponse{
		ID:     resp.CreatedAt, // Ollama doesn't have a response ID, use timestamp
		Model:  core.ModelID(resp.Model),
		Output: resp.Message.Content,
	}

	// Map tool calls
	if len(resp.Message.ToolCalls) > 0 {
		chatResp.ToolCalls = mapToolCalls(resp.Message.ToolCalls)
	}

	// Map thinking/reasoning
	if resp.Message.Thinking != "" {
		chatResp.Reasoning = &core.ReasoningOutput{
			Summary: []string{resp.Message.Thinking},
		}
	}

	// Map usage from durations
	chatResp.Usage = mapUsage(resp)

	return chatResp
}

// mapToolCalls converts Ollama tool calls to core tool calls.
func mapToolCalls(toolCalls []ollamaToolCall) []core.ToolCall {
	result := make([]core.ToolCall, 0, len(toolCalls))

	for i, tc := range toolCalls {
		// Ollama doesn't provide tool call IDs, generate one
		callID := fmt.Sprintf("call_%d", i)

		// Convert arguments map to JSON
		argsJSON, err := json.Marshal(tc.Function.Arguments)
		if err != nil {
			argsJSON = json.RawMessage(`{}`)
		}

		result = append(result, core.ToolCall{
			ID:        callID,
			Name:      tc.Function.Name,
			Arguments: argsJSON,
		})
	}

	return result
}

// mapUsage calculates token usage from Ollama response.
func mapUsage(resp *ollamaResponse) core.TokenUsage {
	return core.TokenUsage{
		PromptTokens:     resp.PromptEvalCount,
		CompletionTokens: resp.EvalCount,
		TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
	}
}
