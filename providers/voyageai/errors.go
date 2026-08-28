package voyageai

import (
	"encoding/json"
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

// voyageErrorResponse represents an error response from the Voyage AI API.
type voyageErrorResponse struct {
	Detail string `json:"detail"`
}

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte, requestID string, headers ...http.Header) error {
	// Parse error response if possible
	var errResp voyageErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Detail
	bodyStr := truncateBody(body)
	if message == "" {
		if bodyStr != "" {
			message = bodyStr // surface the real body instead of http.StatusText
		} else {
			message = http.StatusText(status)
		}
	}

	pe := normalize.ProviderError("voyageai", status, requestID, "", message, normalize.SentinelForStatus(status))
	pe.(*core.ProviderError).Body = bodyStr
	return normalize.WithRetryAfter(pe, headers...)
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("voyageai", err)
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("voyageai", err)
}
