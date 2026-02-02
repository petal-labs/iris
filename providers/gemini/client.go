package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// doChat performs a non-streaming chat request.
func (p *Gemini) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build Gemini request
	gemReq := buildRequest(req)

	// Marshal request body
	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request - model is in URL path
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", p.config.BaseURL, req.Model)
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

	// Check for error status
	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody)
	}

	// Parse response
	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponse(&gemResp, string(req.Model))
}
