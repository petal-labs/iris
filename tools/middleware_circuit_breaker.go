package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation.
	CircuitOpen                         // Failing, reject calls.
	CircuitHalfOpen                     // Testing if recovered.
)

// String returns the string representation of a CircuitState.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Failures before opening.
	SuccessThreshold int           // Successes in half-open to close.
	OpenDuration     time.Duration // How long to stay open.
}

// DefaultCircuitBreakerConfig returns sensible circuit breaker defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		OpenDuration:     30 * time.Second,
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open: too many failures")

// WithCircuitBreaker creates middleware that implements the circuit breaker pattern.
func WithCircuitBreaker(config CircuitBreakerConfig) Middleware {
	var (
		mu          sync.Mutex
		state       CircuitState
		failures    int
		successes   int
		lastFailure time.Time
	)

	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			mu.Lock()

			// Check if circuit should transition from open to half-open.
			if state == CircuitOpen && time.Since(lastFailure) > config.OpenDuration {
				state = CircuitHalfOpen
				successes = 0
			}

			// Reject if circuit is open.
			if state == CircuitOpen {
				mu.Unlock()
				return nil, ErrCircuitOpen
			}

			mu.Unlock()

			// Execute tool.
			result, err := next(ctx, args)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failures++
				lastFailure = time.Now()

				if state == CircuitHalfOpen {
					// Failure in half-open returns to open.
					state = CircuitOpen
				} else if failures >= config.FailureThreshold {
					// Too many failures, open circuit.
					state = CircuitOpen
				}

				return nil, err
			}

			// Success.
			if state == CircuitHalfOpen {
				successes++
				if successes >= config.SuccessThreshold {
					// Enough successes, close circuit.
					state = CircuitClosed
					failures = 0
				}
			} else {
				// Reset failure count on success in closed state.
				failures = 0
			}

			return result, nil
		}
	}
}
