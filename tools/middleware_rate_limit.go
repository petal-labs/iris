package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RateLimiter is the interface for rate limiting.
type RateLimiter interface {
	// Allow returns true if the request should proceed.
	Allow() bool
	// Wait blocks until the request can proceed or context is canceled.
	Wait(ctx context.Context) error
}

// WithRateLimit creates middleware that rate limits tool calls.
// Uses a token bucket algorithm with the specified rate (calls per second).
func WithRateLimit(ratePerSecond float64) Middleware {
	limiter := newTokenBucket(ratePerSecond)
	return WithRateLimiter(limiter)
}

// WithRateLimiter creates middleware using a custom rate limiter.
func WithRateLimiter(limiter RateLimiter) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			if err := limiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("rate limit exceeded: %w", err)
			}
			return next(ctx, args)
		}
	}
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func newTokenBucket(ratePerSecond float64) *tokenBucket {
	return &tokenBucket{
		tokens:     ratePerSecond,
		maxTokens:  ratePerSecond * 2, // Allow burst of 2x rate.
		refillRate: ratePerSecond,
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

func (tb *tokenBucket) Wait(ctx context.Context) error {
	for {
		if tb.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Retry.
		}
	}
}

func (tb *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now
}
