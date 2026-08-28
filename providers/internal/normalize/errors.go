// Package normalize provides shared provider error normalization helpers.
package normalize

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
)

// WithRetryAfter attaches a server-advised retry delay to a normalized
// ProviderError. It accepts the standard Retry-After header plus common
// provider-specific millisecond and reset headers.
func WithRetryAfter(err error, headers ...http.Header) error {
	if err == nil || len(headers) == 0 {
		return err
	}

	var providerErr *core.ProviderError
	if !errors.As(err, &providerErr) {
		return err
	}
	providerErr.RetryAfter = retryAfterDuration(headers[0])
	return err
}

func retryAfterDuration(headers http.Header) time.Duration {
	if value := headers.Get("Retry-After"); value != "" {
		if delay := parseRetryAfterValue(value); delay > 0 {
			return delay
		}
	}
	if value := headers.Get("x-ms-retry-after-ms"); value != "" {
		if milliseconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && milliseconds > 0 {
			return time.Duration(milliseconds) * time.Millisecond
		}
	}
	for _, name := range []string{"x-ratelimit-reset-requests", "x-ratelimit-reset-tokens"} {
		if value := headers.Get(name); value != "" {
			if delay := parseRetryAfterValue(value); delay > 0 {
				return delay
			}
		}
	}
	return 0
}

func parseRetryAfterValue(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		if delay := time.Until(date); delay > 0 {
			return delay
		}
	}
	if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
		return duration
	}
	return 0
}

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

// RequireAPIKey returns a descriptive *core.ProviderError wrapping
// core.ErrUnauthorized when the API key is empty (or whitespace-only), so a
// provider can fail fast before making an unauthenticated request. Returns nil
// when the key is present.
func RequireAPIKey(provider string, key core.Secret) error {
	if key.IsEmpty() {
		return &core.ProviderError{
			Provider: provider,
			Message:  "API key is empty; provide a non-empty key to " + provider + ".New or configure your secret source",
			Err:      core.ErrUnauthorized,
		}
	}
	return nil
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
	case status == http.StatusNotFound:
		return core.ErrNotFound
	case status == http.StatusTooManyRequests:
		return core.ErrRateLimited
	case status >= 500:
		return core.ErrServer
	default:
		return core.ErrServer
	}
}
