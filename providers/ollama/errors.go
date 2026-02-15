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
		return &core.ProviderError{
			Provider: "ollama",
			Code:     "read_error",
			Message:  fmt.Sprintf("failed to read error response: %v", err),
			Status:   resp.StatusCode,
		}
	}

	var errResp ollamaErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// If we can't parse JSON, use the raw body
		return mapOllamaError(resp.StatusCode, string(body))
	}

	if errResp.Error != "" {
		return mapOllamaError(resp.StatusCode, errResp.Error)
	}

	return mapOllamaError(resp.StatusCode, "unknown error")
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

	baseErr := normalize.SentinelForStatusWithOverrides(statusCode, map[int]error{
		http.StatusNotFound: core.ErrBadRequest,
	})

	return &core.ProviderError{
		Provider: "ollama",
		Code:     errType,
		Message:  errMsg,
		Status:   statusCode,
		Err:      baseErr,
	}
}

// newStreamError creates an error from an inline stream error.
func newStreamError(errMsg string) error {
	return &core.ProviderError{
		Provider: "ollama",
		Code:     "stream_error",
		Message:  errMsg,
		Status:   0,
		Err:      core.ErrServer,
	}
}
