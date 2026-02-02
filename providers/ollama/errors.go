package ollama

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/petal-labs/iris/core"
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
	var baseErr error

	switch statusCode {
	case http.StatusBadRequest:
		errType = "bad_request"
		baseErr = core.ErrBadRequest
	case http.StatusNotFound:
		errType = "model_not_found"
		baseErr = core.ErrBadRequest
	case http.StatusTooManyRequests:
		errType = "rate_limited"
		baseErr = core.ErrRateLimited
	case http.StatusInternalServerError:
		errType = "internal_error"
		baseErr = core.ErrServer
	case http.StatusBadGateway:
		errType = "gateway_error"
		baseErr = core.ErrServer
	case http.StatusUnauthorized:
		errType = "unauthorized"
		baseErr = core.ErrUnauthorized
	case http.StatusForbidden:
		errType = "forbidden"
		baseErr = core.ErrUnauthorized
	default:
		errType = "unknown"
		baseErr = core.ErrServer
	}

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
