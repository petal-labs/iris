package normalize

import (
	"encoding/json"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// openAIStyleErrorResponse represents providers that return:
// {"error":{"message":"...","type":"...","code":"..."}}
type openAIStyleErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// OpenAIStyleProviderError normalizes providers that use OpenAI-style error envelopes.
func OpenAIStyleProviderError(provider string, status int, body []byte, requestID string) error {
	var errResp openAIStyleErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Error.Message
	if message == "" {
		message = http.StatusText(status)
	}

	code := errResp.Error.Code
	if code == "" {
		code = errResp.Error.Type
	}

	return &core.ProviderError{
		Provider:  provider,
		Status:    status,
		RequestID: requestID,
		Code:      code,
		Message:   message,
		Err:       sentinelForStatus(status),
	}
}

// NetworkError wraps transport failures as provider-specific network errors.
func NetworkError(provider string, err error) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Err:      core.ErrNetwork,
	}
}

// DecodeError wraps decode/parsing failures as provider-specific decode errors.
func DecodeError(provider string, err error) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Err:      core.ErrDecode,
	}
}

func sentinelForStatus(status int) error {
	switch {
	case status == http.StatusBadRequest:
		return core.ErrBadRequest
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return core.ErrUnauthorized
	case status == http.StatusTooManyRequests:
		return core.ErrRateLimited
	case status >= 500:
		return core.ErrServer
	default:
		return core.ErrServer
	}
}
