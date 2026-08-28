package core

import (
	"context"
	"sync"
	"time"
)

// RateLimiter controls when the client may start an outgoing provider attempt.
// Wait must return context cancellation or deadline errors promptly when ctx is
// done. The client calls Wait for the initial request and every retry attempt.
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// RateLimiterFunc adapts a function into a RateLimiter.
type RateLimiterFunc func(context.Context) error

// Wait implements RateLimiter.
func (f RateLimiterFunc) Wait(ctx context.Context) error {
	if f == nil {
		return contextError(ctx)
	}
	return f(ctx)
}

// IntervalRateLimiter spaces request starts by a fixed interval. It is safe
// for concurrent use and does not queue requests after their contexts expire.
type IntervalRateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

// NewIntervalRateLimiter creates a limiter that permits one request every
// interval. A non-positive interval disables waiting.
func NewIntervalRateLimiter(interval time.Duration) *IntervalRateLimiter {
	if interval < 0 {
		interval = 0
	}
	return &IntervalRateLimiter{interval: interval}
}

// Wait blocks until the next request slot is available or ctx is done.
func (l *IntervalRateLimiter) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil || l.interval <= 0 {
		return ctx.Err()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := contextError(ctx); err != nil {
		return err
	}

	now := time.Now()
	readyAt := l.next
	if readyAt.Before(now) {
		readyAt = now
	}
	if wait := time.Until(readyAt); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	l.next = time.Now().Add(l.interval)
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
