package gemini

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// File-specific error sentinels.
var (
	// ErrFileProcessing indicates the file is still being processed.
	ErrFileProcessing = errors.New("file is still processing")

	// ErrFileFailed indicates file processing failed.
	ErrFileFailed = errors.New("file processing failed")
)

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte) error {
	// Parse error response if possible
	var errResp geminiErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Error.Message
	if message == "" {
		message = http.StatusText(status)
	}

	code := errResp.Error.Status
	if code == "" {
		code = "unknown_error"
	}

	// Determine sentinel error based on status
	var sentinel error
	switch {
	case status == http.StatusBadRequest || status == http.StatusNotFound:
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
		Provider: "gemini",
		Status:   status,
		Code:     code,
		Message:  message,
		Err:      sentinel,
	}
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return &core.ProviderError{
		Provider: "gemini",
		Message:  err.Error(),
		Err:      core.ErrNetwork,
	}
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return &core.ProviderError{
		Provider: "gemini",
		Message:  err.Error(),
		Err:      core.ErrDecode,
	}
}
