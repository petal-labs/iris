// Package normalize provides shared provider error normalization helpers.
package normalize

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
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
	bodyStr := truncateBody(body)
	if message == "" {
		if bodyStr != "" {
			message = bodyStr // surface the real body instead of http.StatusText
		} else {
			message = http.StatusText(status)
		}
	}

	code := errResp.Error.Code
	if code == "" {
		code = errResp.Error.Type
	}

	pe := ProviderError(provider, status, requestID, code, message, SentinelForStatus(status))
	pe.(*core.ProviderError).Body = bodyStr
	return pe
}

// NetworkError wraps transport failures as provider-specific network errors.
func NetworkError(provider string, err error) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Err:      fmt.Errorf("%w: %w", core.ErrNetwork, err),
	}
}

// DecodeError wraps decode/parsing failures as provider-specific decode errors.
func DecodeError(provider string, err error) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Err:      fmt.Errorf("%w: %w", core.ErrDecode, err),
	}
}

// DecodeErrorWithBody wraps decode/parsing failures as provider-specific
// decode errors, additionally preserving the raw response body (truncated)
// that failed to decode so callers can inspect what the provider actually
// sent back.
func DecodeErrorWithBody(provider string, err error, body []byte) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Body:     truncateBody(body),
		Err:      fmt.Errorf("%w: %w", core.ErrDecode, err),
	}
}

// ProviderError constructs a normalized ProviderError.
// If message is empty, HTTP status text is used.
// If sentinel is nil, default status-based mapping is applied.
func ProviderError(provider string, status int, requestID, code, message string, sentinel error) error {
	if message == "" {
		message = http.StatusText(status)
	}
	if sentinel == nil {
		sentinel = SentinelForStatus(status)
	}
	return &core.ProviderError{
		Provider:  provider,
		Status:    status,
		RequestID: requestID,
		Code:      code,
		Message:   message,
		Err:       sentinel,
	}
}

// SentinelForStatus maps an HTTP status code to a core sentinel error.
func SentinelForStatus(status int) error {
	return SentinelForStatusWithOverrides(status, nil)
}

// SentinelForStatusWithOverrides maps an HTTP status code to a core sentinel error,
// then applies any exact status overrides from the provided map.
func SentinelForStatusWithOverrides(status int, overrides map[int]error) error {
	if overrides != nil {
		if override, ok := overrides[status]; ok && override != nil {
			return override
		}
	}

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
