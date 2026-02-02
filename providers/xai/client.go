package xai

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
func (p *Xai) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build xAI request
	xaiReq := buildRequest(req, false)

	// Marshal request body
	body, err := json.Marshal(xaiReq)
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
	var xaiResp xaiResponse
	if err := json.Unmarshal(respBody, &xaiResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponse(&xaiResp)
}
