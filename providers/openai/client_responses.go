package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// responsesPath is the API endpoint for the Responses API.
const responsesPath = "/responses"

// doResponsesChat performs a non-streaming request to the Responses API.
func (p *OpenAI) doResponsesChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build Responses API request
	respReq := buildResponsesRequest(req, false)

	// Marshal request body
	body, err := json.Marshal(respReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	url := p.config.BaseURL + responsesPath
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
	var respResp responsesResponse
	if err := json.Unmarshal(respBody, &respResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponsesResponse(&respResp)
}
