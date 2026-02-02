package core

import (
	"errors"
	"fmt"
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

// Validation errors.
var (
	ErrModelRequired = errors.New("model required")
	ErrNoMessages    = errors.New("no messages")
)
