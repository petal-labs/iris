package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ProviderError represents an error returned by a provider with full context.
type ProviderError struct {
	Provider   string
	Status     int
	RequestID  string
	Code       string
	Message    string
	Body       string        // raw response body (truncated); preserved when Message lacks detail
	RetryAfter time.Duration // server-advised minimum delay before retrying
	Err        error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	// Omit the (status,code) suffix for pure network/decode errors.
	if e.Status == 0 && e.Code == "" {
		return fmt.Sprintf("%s: %s", e.Provider, e.Message)
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (status=%d, code=%s, request_id=%s)",
			e.Provider, e.Message, e.Status, e.Code, e.RequestID)
	}
	return fmt.Sprintf("%s: %s (status=%d, code=%s)",
		e.Provider, e.Message, e.Status, e.Code)
}

// Unwrap returns the underlying error for error chaining.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// Sentinel errors for classification.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrRateLimited  = errors.New("rate limited")
	ErrBadRequest   = errors.New("bad request")
	ErrNotFound     = errors.New("not found")
	ErrServer       = errors.New("server error")
	ErrNetwork      = errors.New("network error")
	ErrDecode       = errors.New("decode error")
	ErrNotSupported = errors.New("operation not supported")
)

// ErrNilStream indicates that DrainStream was called without a stream.
var ErrNilStream = errors.New("nil chat stream")

// Batch processing errors.
var (
	ErrBatchTimeout   = errors.New("batch processing timed out")
	ErrBatchNotFound  = fmt.Errorf("batch %w", ErrNotFound)
	ErrBatchCancelled = errors.New("batch was cancelled")
)

// Validation errors with actionable guidance.
var (
	ErrModelRequired = errors.New("model required: pass a model ID to Client.Chat(), e.g., client.Chat(\"gpt-4\")")
	ErrNoMessages    = errors.New("no messages: add at least one message using .System(), .User(), or .Assistant()")
)

// ErrInvalidSchema indicates a JSON Schema is not compatible with strict
// structured output mode (e.g., missing "additionalProperties": false or a
// "required" array that does not cover all declared properties).
var ErrInvalidSchema = errors.New("invalid json schema for strict structured output")

// ErrStructuredOutputUnsupported indicates a request asked for JSON or
// JSON-Schema structured output (ResponseFormatJSON or
// ResponseFormatJSONSchema) targeting a provider that does not support
// core.FeatureStructuredOutput. validate() returns this before the request
// is sent so callers fail fast instead of silently receiving unconstrained
// output.
var ErrStructuredOutputUnsupported = errors.New("structured output not supported by this provider or model")

// ErrSearchUnsupported indicates a request carried core.SearchOptions
// targeting a provider that does not support core.FeatureWebSearch.
// validate() returns this before the request is sent so callers fail fast
// instead of sending a request whose search directives would be ignored.
var ErrSearchUnsupported = errors.New("web search options not supported by this provider")

// ErrTimeout indicates an Iris-imposed execution timeout elapsed before the
// provider call completed. It wraps context.DeadlineExceeded, so
// errors.Is(err, context.DeadlineExceeded) also holds.
var ErrTimeout = errors.New("execution timeout")

// newTimeoutError builds a timeout error carrying the elapsed budget, provider, and model. The
// returned error satisfies errors.Is for both ErrTimeout and
// context.DeadlineExceeded.
func newTimeoutError(d time.Duration, provider string, model ModelID) error {
	return fmt.Errorf("iris: %w after %s (provider=%s, model=%s): %w",
		ErrTimeout, d, provider, model, context.DeadlineExceeded)
}

// newStreamIdleError builds an error reported when a stream produces no
// chunks for at least d (see WithStreamIdleTimeout). The returned error
// satisfies errors.Is for both ErrTimeout and context.DeadlineExceeded.
func newStreamIdleError(d time.Duration) error {
	return fmt.Errorf("iris: stream idle %w after %s: %w", ErrTimeout, d, context.DeadlineExceeded)
}
