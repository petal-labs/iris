package core

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy determines retry behavior for failed requests.
type RetryPolicy interface {
	// NextDelay returns the delay before the next retry attempt and whether to retry.
	// If ok is false, no more retries should be attempted.
	// attempt starts at 0 for the first retry after the initial failure.
	NextDelay(attempt int, err error) (delay time.Duration, ok bool)
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries int           // Maximum number of retry attempts (default: 3)
	BaseDelay  time.Duration // Initial delay before first retry (default: 1s)
	MaxDelay   time.Duration // Maximum delay cap (default: 30s)
	Jitter     float64       // Jitter factor 0.0-1.0 (default: 0.2)
}

// DefaultRetryPolicy returns a retry policy with sensible defaults.
// Uses exponential backoff with jitter, max 3 retries, 30s max delay.
func DefaultRetryPolicy() RetryPolicy {
	return NewRetryPolicy(RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Jitter:     0.2,
	})
}

// NewRetryPolicy creates a retry policy with the given configuration.
func NewRetryPolicy(cfg RetryConfig) RetryPolicy {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.Jitter < 0 || cfg.Jitter > 1 {
		cfg.Jitter = 0.2
	}
	return &exponentialBackoff{cfg: cfg}
}

type exponentialBackoff struct {
	cfg RetryConfig
}

func (e *exponentialBackoff) NextDelay(attempt int, err error) (time.Duration, bool) {
	// Check if we've exceeded max retries
	if attempt >= e.cfg.MaxRetries {
		return 0, false
	}

	// Check if error is retryable
	if !isRetryable(err) {
		return 0, false
	}

	// Calculate exponential backoff: baseDelay * 2^attempt
	delay := float64(e.cfg.BaseDelay) * math.Pow(2, float64(attempt))

	// Apply jitter: delay * (1 + random(-jitter, +jitter))
	if e.cfg.Jitter > 0 {
		jitterRange := delay * e.cfg.Jitter
		jitter := (rand.Float64()*2 - 1) * jitterRange // random in [-jitterRange, +jitterRange]
		delay += jitter
	}

	// Cap at max delay
	if delay > float64(e.cfg.MaxDelay) {
		delay = float64(e.cfg.MaxDelay)
	}

	// Ensure non-negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay), true
}

// isRetryable determines if an error should trigger a retry.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context cancellation is not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Non-retryable sentinel errors
	if errors.Is(err, ErrUnauthorized) {
		return false
	}
	if errors.Is(err, ErrBadRequest) {
		return false
	}
	if errors.Is(err, ErrDecode) {
		return false
	}

	// Retryable sentinel errors
	if errors.Is(err, ErrNetwork) {
		return true
	}
	if errors.Is(err, ErrRateLimited) {
		return true
	}
	if errors.Is(err, ErrServer) {
		return true
	}

	// Check ProviderError for status codes
	var pe *ProviderError
	if errors.As(err, &pe) {
		return isRetryableStatus(pe.Status)
	}

	// Unknown errors are not retried by default
	return false
}

// isRetryableStatus checks if an HTTP status code indicates a retryable error.
func isRetryableStatus(status int) bool {
	// Rate limited
	if status == 429 {
		return true
	}
	// Server errors (5xx)
	if status >= 500 && status < 600 {
		return true
	}
	return false
}
