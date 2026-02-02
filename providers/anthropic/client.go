package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// messagesPath is the API endpoint for messages.
const messagesPath = "/v1/messages"

// doChat performs a non-streaming chat request.
func (p *Anthropic) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build Anthropic request
	antReq := buildRequest(req, false)

	// Marshal request body
	body, err := json.Marshal(antReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	url := p.config.BaseURL + messagesPath
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
	requestID := resp.Header.Get("request-id")

	// Check for error status
	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Parse response
	var antResp anthropicResponse
	if err := json.Unmarshal(respBody, &antResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponse(&antResp)
}
