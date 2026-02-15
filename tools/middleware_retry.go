package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
	Retryable   func(error) bool // Returns true if error is retryable.
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
		Retryable: func(err error) bool {
			// Retry on common transient errors.
			return errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(err.Error(), "temporary") ||
				strings.Contains(err.Error(), "timeout")
		},
	}
}

// WithRetry creates middleware that retries failed tool calls.
func WithRetry(config RetryConfig) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			var lastErr error
			wait := config.InitialWait

			for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
				result, err := next(ctx, args)
				if err == nil {
					return result, nil
				}

				lastErr = err

				// Check if error is retryable.
				if config.Retryable != nil && !config.Retryable(err) {
					return nil, err
				}

				// Don't wait after last attempt.
				if attempt == config.MaxAttempts {
					break
				}

				// Wait with exponential backoff.
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}

				wait = time.Duration(float64(wait) * config.Multiplier)
				if wait > config.MaxWait {
					wait = config.MaxWait
				}
			}

			return nil, fmt.Errorf("tool call failed after %d attempts: %w", config.MaxAttempts, lastErr)
		}
	}
}
