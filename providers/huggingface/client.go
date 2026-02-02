package huggingface

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
)

// chatCompletionsPath is the API endpoint for chat completions.
const chatCompletionsPath = "/v1/chat/completions"

// buildModelString constructs the model identifier with optional provider policy suffix.
// If the model already contains a colon (explicit provider), it's returned as-is.
// Otherwise, if a provider policy is configured, it's appended.
func (p *HuggingFace) buildModelString(model core.ModelID) string {
	modelStr := string(model)

	// If model already has a suffix (contains colon), don't modify it
	if strings.Contains(modelStr, ":") {
		return modelStr
	}

	// Append provider policy if configured and not "auto"
	if p.config.ProviderPolicy != "" && p.config.ProviderPolicy != PolicyAuto {
		return modelStr + ":" + p.config.ProviderPolicy
	}

	return modelStr
}

// chatURL returns the full URL for the chat completions endpoint.
func (p *HuggingFace) chatURL() string {
	return p.config.BaseURL + chatCompletionsPath
}

// doChat performs a non-streaming chat completion request.
func (p *HuggingFace) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build model string with provider policy
	model := p.buildModelString(req.Model)

	// Build HF request
	hfReq := buildRequest(req, model, false)

	// Marshal request body
	body, err := json.Marshal(hfReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatURL(), bytes.NewReader(body))
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
	var hfResp hfResponse
	if err := json.Unmarshal(respBody, &hfResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Map to Iris response
	return mapResponse(&hfResp)
}
