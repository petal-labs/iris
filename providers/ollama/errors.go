package ollama

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
)

// parseErrorResponse reads and parses an error response from Ollama.
func parseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return normalize.WithRetryAfter(normalize.ProviderError(
			"ollama",
			resp.StatusCode,
			"",
			"read_error",
			fmt.Sprintf("failed to read error response: %v", err),
			fmt.Errorf("%w: %w", core.ErrNetwork, err),
		), resp.Header)
	}

	var errResp ollamaErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// If we can't parse JSON, use the raw body
		return normalize.WithRetryAfter(mapOllamaError(resp.StatusCode, string(body)), resp.Header)
	}

	if errResp.Error != "" {
		return normalize.WithRetryAfter(mapOllamaError(resp.StatusCode, errResp.Error), resp.Header)
	}

	return normalize.WithRetryAfter(mapOllamaError(resp.StatusCode, "unknown error"), resp.Header)
}

// mapOllamaError converts an Ollama error to a core.ProviderError.
func mapOllamaError(statusCode int, errMsg string) error {
	var errType string

	switch statusCode {
	case http.StatusBadRequest:
		errType = "bad_request"
	case http.StatusNotFound:
		errType = "model_not_found"
	case http.StatusTooManyRequests:
		errType = "rate_limited"
	case http.StatusInternalServerError:
		errType = "internal_error"
	case http.StatusBadGateway:
		errType = "gateway_error"
	case http.StatusUnauthorized:
		errType = "unauthorized"
	case http.StatusForbidden:
		errType = "forbidden"
	default:
		errType = "unknown"
	}

	baseErr := normalize.SentinelForStatus(statusCode)

	return normalize.ProviderError("ollama", statusCode, "", errType, errMsg, baseErr)
}

// newStreamError creates an error from an inline stream error.
func newStreamError(errMsg string) error {
	return normalize.ProviderError("ollama", 0, "", "stream_error", errMsg, core.ErrServer)
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	providerErr := normalize.NetworkError("ollama", err).(*core.ProviderError)
	providerErr.Code = "network_error"
	return providerErr
}

// newDecodeError creates a ProviderError for JSON encoding and decoding failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("ollama", err)
}

// newDecodeErrorWithBody creates a ProviderError for JSON decoding failures
// and preserves the payload that could not be decoded.
func newDecodeErrorWithBody(err error, body []byte) error {
	return normalize.DecodeErrorWithBody("ollama", err, body)
}
