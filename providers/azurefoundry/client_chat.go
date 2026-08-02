package azurefoundry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/petal-labs/iris/core"
)

// cancelOnCloseBody wraps a response body so that Close also releases an
// associated context.CancelFunc. Used to tie a per-request timeout context
// to the lifetime of a streamed response body, which is read and closed by
// a goroutine started after the originating call returns.
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

// Close closes the underlying body and then calls cancel.
func (c *cancelOnCloseBody) Close() error {
	defer c.cancel()
	return c.ReadCloser.Close()
}

// doChat performs a non-streaming chat completion request.
func (p *AzureFoundry) doChat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	// Apply timeout if configured
	if p.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.config.Timeout)
		defer cancel()
	}

	// Build Azure request
	azReq := buildRequest(req, false)

	// Marshal request body
	body, err := json.Marshal(azReq)
	if err != nil {
		return nil, newDecodeError(err)
	}

	// Build URL
	url, err := p.buildChatURL(req.Model)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, newNetworkError(err)
	}

	// Set headers (may require context for token refresh)
	headers, err := p.buildHeaders(ctx)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
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
	requestID := extractRequestID(resp.Header)

	// Check for error status
	if resp.StatusCode >= 400 {
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Parse response
	var azResp azureResponse
	if err := json.Unmarshal(respBody, &azResp); err != nil {
		return nil, newDecodeError(err)
	}

	// Check for content filtering in response
	if len(azResp.Choices) > 0 {
		choice := azResp.Choices[0]
		if choice.ContentFilterResults != nil && choice.ContentFilterResults.IsFiltered() {
			return nil, newContentFilterError(choice.ContentFilterResults)
		}
	}

	// Map to Iris response
	return mapChatResponse(&azResp)
}

// doStreamChat performs a streaming chat completion request.
func (p *AzureFoundry) doStreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	// Apply timeout if configured. The timeout must outlive this function:
	// the SSE stream is read by a goroutine started after doStreamChat
	// returns, so cancel is NOT deferred here (that would cancel the
	// context before the goroutine finishes). Instead, cancel is attached
	// to the response body's Close, which the reader goroutine calls when
	// it is done (success, error, or context cancellation).
	var cancel context.CancelFunc
	if p.config.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, p.config.Timeout)
	}

	// Build Azure request with streaming enabled
	azReq := buildRequest(req, true)

	// Marshal request body
	body, err := json.Marshal(azReq)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, newDecodeError(err)
	}

	// Build URL
	url, err := p.buildChatURL(req.Model)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, err
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, newNetworkError(err)
	}

	// Set headers
	headers, err := p.buildHeaders(ctx)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, err
	}
	for key, values := range headers {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	// Execute request
	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, newNetworkError(err)
	}
	if cancel != nil {
		resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
	}

	// Extract request ID
	requestID := extractRequestID(resp.Header)

	// Check for error status before streaming
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, newNetworkError(err)
		}
		return nil, normalizeError(resp.StatusCode, respBody, requestID)
	}

	// Process SSE stream. The stream-reading goroutine closes resp.Body
	// when it finishes, which releases the timeout context via cancel.
	return p.processSSEStream(ctx, resp, req.Model)
}

// mapChatResponse converts an Azure response to an Iris ChatResponse.
func mapChatResponse(resp *azureResponse) (*core.ChatResponse, error) {
	result := &core.ChatResponse{
		ID:    resp.ID,
		Model: core.ModelID(resp.Model),
		Usage: mapUsage(resp.Usage),
	}

	// Extract content from first choice
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message != nil {
			result.Output = choice.Message.Content

			// Map tool calls if present
			if len(choice.Message.ToolCalls) > 0 {
				toolCalls, err := mapToolCallsWithValidation(choice.Message.ToolCalls)
				if err != nil {
					return nil, err
				}
				result.ToolCalls = toolCalls
			}
		}
	}

	return result, nil
}

// mapToolCallsWithValidation converts Azure tool calls to Iris ToolCalls with JSON validation.
func mapToolCallsWithValidation(calls []azureToolCall) ([]core.ToolCall, error) {
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

// extractRequestID extracts the request ID from Azure response headers.
// Azure uses different header names depending on the endpoint.
func extractRequestID(h http.Header) string {
	// Try Azure-specific headers first
	if id := h.Get("x-ms-request-id"); id != "" {
		return id
	}
	if id := h.Get("apim-request-id"); id != "" {
		return id
	}
	// Fall back to OpenAI-style header
	return h.Get("x-request-id")
}

// retryableStatus returns true if the HTTP status code indicates a retryable error.
func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// calculateBackoff returns the backoff duration for a given attempt.
func calculateBackoff(attempt int, baseDelay time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 10 {
		attempt = 10 // Cap to prevent overflow
	}
	delay := baseDelay * time.Duration(1<<attempt)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}
