package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
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

	sentinel := normalize.SentinelForStatusWithOverrides(status, map[int]error{
		http.StatusNotFound: core.ErrNotFound,
	})

	return normalize.ProviderError("anthropic", status, requestID, code, message, sentinel)
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("anthropic", err)
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("anthropic", err)
}
