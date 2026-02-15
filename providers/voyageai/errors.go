package voyageai

import (
	"encoding/json"
	"net/http"

	"github.com/petal-labs/iris/providers/internal/normalize"
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

	return normalize.ProviderError("voyageai", status, requestID, "", message, normalize.SentinelForStatus(status))
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("voyageai", err)
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("voyageai", err)
}
