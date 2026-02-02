package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	if policy == nil {
		t.Fatal("DefaultRetryPolicy() returned nil")
	}
}

func TestRetryPolicyRetryableErrors(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{"ErrNetwork", ErrNetwork, true},
		{"ErrRateLimited", ErrRateLimited, true},
		{"ErrServer", ErrServer, true},
		{"wrapped ErrNetwork", &ProviderError{Provider: "test", Err: ErrNetwork}, true},
		{"wrapped ErrRateLimited", &ProviderError{Provider: "test", Err: ErrRateLimited}, true},
		{"wrapped ErrServer", &ProviderError{Provider: "test", Err: ErrServer}, true},
		{"ProviderError 429", &ProviderError{Provider: "test", Status: 429}, true},
		{"ProviderError 500", &ProviderError{Provider: "test", Status: 500}, true},
		{"ProviderError 502", &ProviderError{Provider: "test", Status: 502}, true},
		{"ProviderError 503", &ProviderError{Provider: "test", Status: 503}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := policy.NextDelay(0, tt.err)
			if ok != tt.wantRetry {
				t.Errorf("NextDelay(0, %v) retry = %v, want %v", tt.err, ok, tt.wantRetry)
			}
		})
	}
}

func TestRetryPolicyNonRetryableErrors(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrUnauthorized", ErrUnauthorized},
		{"ErrBadRequest", ErrBadRequest},
		{"ErrDecode", ErrDecode},
		{"context.Canceled", context.Canceled},
		{"context.DeadlineExceeded", context.DeadlineExceeded},
		{"wrapped ErrUnauthorized", &ProviderError{Provider: "test", Err: ErrUnauthorized}},
		{"wrapped ErrBadRequest", &ProviderError{Provider: "test", Err: ErrBadRequest}},
		{"ProviderError 400", &ProviderError{Provider: "test", Status: 400}},
		{"ProviderError 401", &ProviderError{Provider: "test", Status: 401}},
		{"ProviderError 403", &ProviderError{Provider: "test", Status: 403}},
		{"ProviderError 404", &ProviderError{Provider: "test", Status: 404}},
		{"nil error", nil},
		{"unknown error", errors.New("unknown error")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := policy.NextDelay(0, tt.err)
			if ok {
				t.Errorf("NextDelay(0, %v) should not retry", tt.err)
			}
		})
	}
}

func TestRetryPolicyMaxRetries(t *testing.T) {
	policy := NewRetryPolicy(RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Jitter:     0, // disable jitter for predictable testing
	})

	err := ErrNetwork

	// Should allow retries for attempts 0, 1, 2
	for attempt := 0; attempt < 3; attempt++ {
		_, ok := policy.NextDelay(attempt, err)
		if !ok {
			t.Errorf("NextDelay(%d, err) should allow retry", attempt)
		}
	}

	// Should not allow retry for attempt 3 (4th retry = exceeds max)
	_, ok := policy.NextDelay(3, err)
	if ok {
		t.Error("NextDelay(3, err) should not allow retry (exceeds max)")
	}
}

func TestRetryPolicyExponentialBackoff(t *testing.T) {
	policy := NewRetryPolicy(RetryConfig{
		MaxRetries: 5,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Jitter:     0, // disable jitter for predictable testing
	})

	err := ErrNetwork
	var lastDelay time.Duration

	for attempt := 0; attempt < 4; attempt++ {
		delay, ok := policy.NextDelay(attempt, err)
		if !ok {
			t.Fatalf("NextDelay(%d, err) should allow retry", attempt)
		}

		// Expected delays: 100ms, 200ms, 400ms, 800ms (2^attempt * base)
		expectedDelay := time.Duration(100*time.Millisecond) * time.Duration(1<<attempt)
		if delay != expectedDelay {
			t.Errorf("attempt %d: delay = %v, want %v", attempt, delay, expectedDelay)
		}

		// Verify exponential growth
		if attempt > 0 && delay <= lastDelay {
			t.Errorf("attempt %d: delay %v should be greater than previous %v", attempt, delay, lastDelay)
		}
		lastDelay = delay
	}
}

func TestRetryPolicyMaxDelayCap(t *testing.T) {
	policy := NewRetryPolicy(RetryConfig{
		MaxRetries: 10,
		BaseDelay:  time.Second,
		MaxDelay:   5 * time.Second,
		Jitter:     0,
	})

	err := ErrNetwork

	// At attempt 5, exponential would be 32s, but should be capped at 5s
	delay, ok := policy.NextDelay(5, err)
	if !ok {
		t.Fatal("should allow retry")
	}
	if delay > 5*time.Second {
		t.Errorf("delay = %v, should be capped at 5s", delay)
	}
	if delay != 5*time.Second {
		t.Errorf("delay = %v, want 5s (max cap)", delay)
	}
}

func TestRetryPolicyJitter(t *testing.T) {
	policy := NewRetryPolicy(RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Jitter:     0.5, // 50% jitter
	})

	err := ErrNetwork
	delays := make(map[time.Duration]bool)

	// Run multiple times to verify jitter produces variance
	for i := 0; i < 100; i++ {
		delay, ok := policy.NextDelay(0, err)
		if !ok {
			t.Fatal("should allow retry")
		}
		delays[delay] = true

		// With 50% jitter on 1s base, delay should be in [0.5s, 1.5s]
		if delay < 500*time.Millisecond || delay > 1500*time.Millisecond {
			t.Errorf("delay %v outside expected jitter range [0.5s, 1.5s]", delay)
		}
	}

	// Should have multiple different delays due to jitter
	if len(delays) < 2 {
		t.Error("jitter should produce varying delays")
	}
}

func TestRetryPolicyConfigDefaults(t *testing.T) {
	// Zero/invalid config should use defaults
	policy := NewRetryPolicy(RetryConfig{
		MaxRetries: 0,  // should default to 3
		BaseDelay:  0,  // should default to 1s
		MaxDelay:   0,  // should default to 30s
		Jitter:     -1, // should default to 0.2
	})

	err := ErrNetwork

	// Should still work with defaults
	_, ok := policy.NextDelay(0, err)
	if !ok {
		t.Error("policy with default config should allow retry")
	}

	// Should stop after 3 retries (default)
	_, ok = policy.NextDelay(3, err)
	if ok {
		t.Error("policy should respect default max retries of 3")
	}
}

func TestRetryPolicyProviderErrorStatusCodes(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		status    int
		wantRetry bool
	}{
		{200, false}, // success
		{400, false}, // bad request
		{401, false}, // unauthorized
		{403, false}, // forbidden
		{404, false}, // not found
		{422, false}, // unprocessable
		{429, true},  // rate limited
		{500, true},  // internal server error
		{502, true},  // bad gateway
		{503, true},  // service unavailable
		{504, true},  // gateway timeout
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.status)), func(t *testing.T) {
			err := &ProviderError{Provider: "test", Status: tt.status, Message: "test"}
			_, ok := policy.NextDelay(0, err)
			if ok != tt.wantRetry {
				t.Errorf("status %d: retry = %v, want %v", tt.status, ok, tt.wantRetry)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ErrNetwork", ErrNetwork, true},
		{"ErrRateLimited", ErrRateLimited, true},
		{"ErrServer", ErrServer, true},
		{"ErrUnauthorized", ErrUnauthorized, false},
		{"ErrBadRequest", ErrBadRequest, false},
		{"ErrDecode", ErrDecode, false},
		{"ErrModelRequired", ErrModelRequired, false},
		{"ErrNoMessages", ErrNoMessages, false},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"unknown error", errors.New("random"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
