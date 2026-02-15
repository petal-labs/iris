package gemini

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
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

	sentinel := normalize.SentinelForStatusWithOverrides(status, map[int]error{
		http.StatusNotFound: core.ErrBadRequest,
	})

	return normalize.ProviderError("gemini", status, "", code, message, sentinel)
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("gemini", err)
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("gemini", err)
}
