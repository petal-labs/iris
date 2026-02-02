package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// chatCompletionsPath is the API endpoint for chat completions.
const chatCompletionsPath = "/chat/completions"

// doChat performs a non-streaming chat completion request.
func (p *OpenAI) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build OpenAI request
	oaiReq := buildRequest(req, false)

	// Marshal request body
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	url := p.config.BaseURL + chatCompletionsPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Set headers
	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Execute request
	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, newNetworkError(err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Extract request ID from response headers
	requestID := resp.Header.Get("x-request-id")

	// Check for error status
	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Parse response
	var oaiResp openAIResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponse(&oaiResp)
}

// mapResponse converts an OpenAI response to an Iris ChatResponse.
func mapResponse(resp *openAIResponse) (*core.ChatResponse, error) {
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

// mapToolCalls converts OpenAI tool calls to Iris ToolCalls.
func mapToolCalls(calls []openAIToolCall) ([]core.ToolCall, error) {
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
