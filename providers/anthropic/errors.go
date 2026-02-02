package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// ErrToolArgsInvalidJSON is returned when tool call arguments contain invalid JSON.
var ErrToolArgsInvalidJSON = errors.New("tool args invalid json")

// ErrFileNotDownloadable is returned when attempting to download a user-uploaded file.
var ErrFileNotDownloadable = errors.New("file not downloadable")

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte, requestID string) error {
	// Parse error response if possible
	var errResp anthropicErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Error.Message
	if message == "" {
		message = http.StatusText(status)
	}

	code := errResp.Error.Type
	if code == "" {
		code = "unknown_error"
	}

	// Determine sentinel error based on status
	var sentinel error
	switch {
	case status == http.StatusBadRequest:
		sentinel = core.ErrBadRequest
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		sentinel = core.ErrUnauthorized
	case status == http.StatusNotFound:
		sentinel = core.ErrNotFound
	case status == http.StatusTooManyRequests:
		sentinel = core.ErrRateLimited
	case status >= 500:
		sentinel = core.ErrServer
	default:
		sentinel = core.ErrServer
	}

	return &core.ProviderError{
		Provider:  "anthropic",
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
		Provider: "anthropic",
		Message:  err.Error(),
		Err:      core.ErrNetwork,
	}
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return &core.ProviderError{
		Provider: "anthropic",
		Message:  err.Error(),
		Err:      core.ErrDecode,
	}
}
