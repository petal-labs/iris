package zai

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
func (p *Zai) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build Z.ai request
	zaiReq := buildRequest(req, false)

	// Marshal request body
	body, err := json.Marshal(zaiReq)
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

	// Extract request ID from response body (Z.ai returns it in the response JSON)
	var tempResp struct {
		RequestID string `json:"request_id"`
	}
	_ = json.Unmarshal(respBody, &tempResp)
	requestID := tempResp.RequestID

	// Check for error status
	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Parse response
	var zaiResp zaiResponse
	if err := json.Unmarshal(respBody, &zaiResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponse(&zaiResp)
}
