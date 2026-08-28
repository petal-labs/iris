package openai

import (
	"errors"
	"net/http"

	"github.com/petal-labs/iris/providers/internal/normalize"
)

// ErrToolArgsInvalidJSON is returned when tool call arguments contain invalid JSON.
var ErrToolArgsInvalidJSON = errors.New("tool args invalid json")

// openAIErrorResponse represents an OpenAI API error response.
// It is retained for test fixtures.
type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte, requestID string, headers ...http.Header) error {
	return normalize.WithRetryAfter(
		normalize.OpenAIStyleProviderError("openai", status, body, requestID), headers...,
	)
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("openai", err)
}

// newDecodeError creates a ProviderError for JSON decode failures, preserving
// the raw body that failed to decode.
func newDecodeError(err error, body []byte) error {
	return normalize.DecodeErrorWithBody("openai", err, body)
}
