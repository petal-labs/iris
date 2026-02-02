package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// doChat sends a non-streaming chat request to the Ollama API.
func (p *Ollama) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Build request body
	ollamaReq := mapRequest(req, false)

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := p.config.BaseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Send request
	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "ollama",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for inline error
	if ollamaResp.Error != "" {
		return nil, mapOllamaError(resp.StatusCode, ollamaResp.Error)
	}

	return mapResponse(&ollamaResp), nil
}
