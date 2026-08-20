package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
)

// maxBodyLen caps how much of a raw response body is retained on
// ProviderError.Body so oversized error pages don't bloat logs.
const maxBodyLen = 4096

// truncateBody trims and caps a raw response body for inclusion on
// ProviderError.Body.
func truncateBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxBodyLen {
		return s[:maxBodyLen] + "…(truncated)"
	}
	return s
}

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
	bodyStr := truncateBody(body)
	if message == "" {
		if bodyStr != "" {
			message = bodyStr // surface the real body instead of http.StatusText
		} else {
			message = http.StatusText(status)
		}
	}

	code := errResp.Error.Type
	if code == "" {
		code = "unknown_error"
	}

	pe := normalize.ProviderError("anthropic", status, requestID, code, message, normalize.SentinelForStatus(status))
	pe.(*core.ProviderError).Body = bodyStr
	return pe
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("anthropic", err)
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("anthropic", err)
}
