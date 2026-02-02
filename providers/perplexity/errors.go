package perplexity

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// ErrToolArgsInvalidJSON is returned when tool call arguments contain invalid JSON.
var ErrToolArgsInvalidJSON = errors.New("tool args invalid json")

// perplexityErrorResponse represents an error response from the Perplexity API.
type perplexityErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte, requestID string) error {
	// Parse error response if possible
	var errResp perplexityErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Error.Message
	if message == "" {
		message = http.StatusText(status)
	}

	code := errResp.Error.Code
	if code == "" {
		code = errResp.Error.Type
	}

	// Determine sentinel error based on status
	var sentinel error
	switch {
	case status == http.StatusBadRequest:
		sentinel = core.ErrBadRequest
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		sentinel = core.ErrUnauthorized
	case status == http.StatusTooManyRequests:
		sentinel = core.ErrRateLimited
	case status >= 500:
		sentinel = core.ErrServer
	default:
		sentinel = core.ErrServer
	}

	return &core.ProviderError{
		Provider:  "perplexity",
		Status:    status,
		RequestID: requestID,
		Code:      code,
		Message:   message,
		Err:       sentinel,
	}
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return &core.ProviderError{
		Provider: "perplexity",
		Message:  err.Error(),
		Err:      core.ErrNetwork,
	}
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return &core.ProviderError{
		Provider: "perplexity",
		Message:  err.Error(),
		Err:      core.ErrDecode,
	}
}
