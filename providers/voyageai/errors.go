package voyageai

import (
	"encoding/json"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// voyageErrorResponse represents an error response from the Voyage AI API.
type voyageErrorResponse struct {
	Detail string `json:"detail"`
}

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte, requestID string) error {
	// Parse error response if possible
	var errResp voyageErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Detail
	if message == "" {
		message = http.StatusText(status)
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
		Provider:  "voyageai",
		Status:    status,
		RequestID: requestID,
		Message:   message,
		Err:       sentinel,
	}
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return &core.ProviderError{
		Provider: "voyageai",
		Message:  err.Error(),
		Err:      core.ErrNetwork,
	}
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return &core.ProviderError{
		Provider: "voyageai",
		Message:  err.Error(),
		Err:      core.ErrDecode,
	}
}
