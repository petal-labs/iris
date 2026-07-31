package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ProviderError represents an error returned by a provider with full context.
type ProviderError struct {
	Provider  string
	Status    int
	RequestID string
	Code      string
	Message   string
	Err       error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
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

// Batch processing errors.
var (
	ErrBatchTimeout   = errors.New("batch processing timed out")
	ErrBatchNotFound  = errors.New("batch not found")
	ErrBatchCancelled = errors.New("batch was cancelled")
)

// Validation errors with actionable guidance.
var (
	ErrModelRequired = errors.New("model required: pass a model ID to Client.Chat(), e.g., client.Chat(\"gpt-4\")")
	ErrNoMessages    = errors.New("no messages: add at least one message using .System(), .User(), or .Assistant()")
)

// ErrTimeout indicates an Iris-imposed execution timeout elapsed before the
// provider call completed. It wraps context.DeadlineExceeded, so
// errors.Is(err, context.DeadlineExceeded) also holds.
var ErrTimeout = errors.New("execution timeout")

// newTimeoutError builds a timeout error carrying the elapsed budget. The
// returned error satisfies errors.Is for both ErrTimeout and
// context.DeadlineExceeded.
func newTimeoutError(d time.Duration) error {
	return fmt.Errorf("iris: %w after %s: %w", ErrTimeout, d, context.DeadlineExceeded)
}
