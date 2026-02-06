package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockTool is a test implementation of Tool.
type mockTool struct {
	name        string
	description string
	callFn      func(ctx context.Context, args json.RawMessage) (any, error)
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }
func (t *mockTool) Schema() ToolSchema  { return ToolSchema{JSONSchema: json.RawMessage(`{}`)} }
func (t *mockTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	if t.callFn != nil {
		return t.callFn(ctx, args)
	}
	return "result", nil
}

// -----------------------------------------------------------------------------
// Core Types Tests
// -----------------------------------------------------------------------------

func TestToolContextFromContext(t *testing.T) {
	// Context without ToolContext
	ctx := context.Background()
	tc := ToolContextFromContext(ctx)
	if tc != nil {
		t.Error("expected nil for context without ToolContext")
	}

	// Context with ToolContext
	expected := &ToolContext{
		ToolName:  "test_tool",
		CallID:    "call-123",
		Iteration: 5,
		Metadata:  map[string]any{"key": "value"},
	}
	ctx = ContextWithToolContext(ctx, expected)
	tc = ToolContextFromContext(ctx)
	if tc == nil {
		t.Fatal("expected ToolContext, got nil")
	}
	if tc.ToolName != expected.ToolName {
		t.Errorf("ToolName = %q, want %q", tc.ToolName, expected.ToolName)
	}
	if tc.CallID != expected.CallID {
		t.Errorf("CallID = %q, want %q", tc.CallID, expected.CallID)
	}
	if tc.Iteration != expected.Iteration {
		t.Errorf("Iteration = %d, want %d", tc.Iteration, expected.Iteration)
	}
	if tc.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want %q", tc.Metadata["key"], "value")
	}
}

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			order = append(order, "m1-before")
			result, err := next(ctx, args)
			order = append(order, "m1-after")
			return result, err
		}
	}

	m2 := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			order = append(order, "m2-before")
			result, err := next(ctx, args)
			order = append(order, "m2-after")
			return result, err
		}
	}

	tool := func(ctx context.Context, args json.RawMessage) (any, error) {
		order = append(order, "tool")
		return "done", nil
	}

	chain := Chain(m1, m2)
	wrapped := chain(tool)

	_, err := wrapped(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"m1-before", "m2-before", "tool", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order length = %d, want %d", len(order), len(expected))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestApplyMiddleware(t *testing.T) {
	callCount := 0

	middleware := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			return next(ctx, args)
		}
	}

	tool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
	}

	wrapped := ApplyMiddleware(tool, middleware)

	// Verify tool metadata is preserved
	if wrapped.Name() != "test_tool" {
		t.Errorf("Name = %q, want %q", wrapped.Name(), "test_tool")
	}
	if wrapped.Description() != "A test tool" {
		t.Errorf("Description = %q, want %q", wrapped.Description(), "A test tool")
	}

	// Call the wrapped tool
	_, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestApplyMiddlewareNoMiddleware(t *testing.T) {
	tool := &mockTool{name: "test"}
	wrapped := ApplyMiddleware(tool)

	// Should return the same tool
	if wrapped != tool {
		t.Error("expected same tool when no middleware provided")
	}
}

func TestWrappedToolSetsToolContext(t *testing.T) {
	var capturedTC *ToolContext

	middleware := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			capturedTC = ToolContextFromContext(ctx)
			return next(ctx, args)
		}
	}

	tool := &mockTool{name: "my_tool"}
	wrapped := ApplyMiddleware(tool, middleware)

	_, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedTC == nil {
		t.Fatal("expected ToolContext to be set")
	}
	if capturedTC.ToolName != "my_tool" {
		t.Errorf("ToolName = %q, want %q", capturedTC.ToolName, "my_tool")
	}
}

// -----------------------------------------------------------------------------
// Logging Middleware Tests
// -----------------------------------------------------------------------------

func TestWithLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	tool := &mockTool{name: "test_tool"}
	wrapped := ApplyMiddleware(tool, WithLogging(logger))

	_, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "tool call start: test_tool") {
		t.Errorf("expected start log, got: %s", output)
	}
	if !strings.Contains(output, "tool call success: test_tool") {
		t.Errorf("expected success log, got: %s", output)
	}
}

func TestWithLoggingError(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	tool := &mockTool{
		name: "failing_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			return nil, errors.New("tool failed")
		},
	}
	wrapped := ApplyMiddleware(tool, WithLogging(logger))

	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}

	output := buf.String()
	if !strings.Contains(output, "tool call error: failing_tool") {
		t.Errorf("expected error log, got: %s", output)
	}
}

func TestWithDetailedLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	tool := &mockTool{name: "test_tool"}
	wrapped := ApplyMiddleware(tool, WithDetailedLogging(logger))

	args := json.RawMessage(`{"key":"value"}`)
	_, err := wrapped.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `{"key":"value"}`) {
		t.Errorf("expected args in log, got: %s", output)
	}
	if !strings.Contains(output, "tool result: test_tool") {
		t.Errorf("expected result log, got: %s", output)
	}
}

// -----------------------------------------------------------------------------
// Timeout Middleware Tests
// -----------------------------------------------------------------------------

func TestWithTimeout(t *testing.T) {
	tool := &mockTool{
		name: "slow_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	// Test with sufficient timeout
	wrapped := ApplyMiddleware(tool, WithTimeout(200*time.Millisecond))
	result, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error with sufficient timeout: %v", err)
	}
	if result != "done" {
		t.Errorf("result = %v, want %q", result, "done")
	}

	// Test with insufficient timeout
	wrapped = ApplyMiddleware(tool, WithTimeout(50*time.Millisecond))
	_, err = wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Rate Limiting Tests
// -----------------------------------------------------------------------------

func TestWithRateLimit(t *testing.T) {
	tool := &mockTool{name: "rate_limited_tool"}
	// 10 calls per second
	wrapped := ApplyMiddleware(tool, WithRateLimit(10.0))

	// Should allow burst of up to 20 (2x rate)
	for i := 0; i < 10; i++ {
		_, err := wrapped.Call(context.Background(), nil)
		if err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}
}

func TestWithRateLimitContextCancellation(t *testing.T) {
	tool := &mockTool{name: "rate_limited_tool"}
	// Rate of 5 calls per second, max burst of 10
	wrapped := ApplyMiddleware(tool, WithRateLimit(5.0))

	// Exhaust all tokens (burst of 10)
	for i := 0; i < 10; i++ {
		wrapped.Call(context.Background(), nil)
	}

	// Try with already-canceled context - should fail immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := wrapped.Call(ctx, nil)
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}

func TestTokenBucket(t *testing.T) {
	tb := newTokenBucket(10.0) // 10 tokens/second, max 20

	// Should allow initial burst
	allowed := 0
	for i := 0; i < 25; i++ {
		if tb.Allow() {
			allowed++
		}
	}

	// Should have allowed up to maxTokens (20)
	if allowed < 10 || allowed > 20 {
		t.Errorf("allowed = %d, expected 10-20", allowed)
	}
}

// -----------------------------------------------------------------------------
// Cache Middleware Tests
// -----------------------------------------------------------------------------

func TestWithCache(t *testing.T) {
	callCount := 0
	tool := &mockTool{
		name: "cacheable_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			return "computed_result", nil
		},
	}

	cache := NewMemoryCache()
	wrapped := ApplyMiddleware(tool, WithCache(cache, time.Hour))

	args := json.RawMessage(`{"key":"value"}`)

	// First call - should compute
	result1, err := wrapped.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if result1 != "computed_result" {
		t.Errorf("result1 = %v, want %q", result1, "computed_result")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call with same args - should use cache
	result2, err := wrapped.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if result2 != "computed_result" {
		t.Errorf("result2 = %v, want %q", result2, "computed_result")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (cached)", callCount)
	}

	// Third call with different args - should compute
	args2 := json.RawMessage(`{"key":"other"}`)
	_, err = wrapped.Call(context.Background(), args2)
	if err != nil {
		t.Fatalf("third call error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestWithCacheDoesNotCacheErrors(t *testing.T) {
	callCount := 0
	tool := &mockTool{
		name: "failing_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			return nil, errors.New("tool error")
		},
	}

	cache := NewMemoryCache()
	wrapped := ApplyMiddleware(tool, WithCache(cache, time.Hour))

	// First call - should fail
	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}

	// Second call - should still fail (not cached)
	_, err = wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (errors not cached)", callCount)
	}
}

func TestMemoryCacheExpiration(t *testing.T) {
	cache := NewMemoryCache()

	cache.Set("key", "value", 50*time.Millisecond)

	// Should be present
	val, ok := cache.Get("key")
	if !ok || val != "value" {
		t.Error("expected value to be present")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get("key")
	if ok {
		t.Error("expected value to be expired")
	}
}

func TestDefaultCacheKey(t *testing.T) {
	key1 := DefaultCacheKey("tool1", json.RawMessage(`{"a":1}`))
	key2 := DefaultCacheKey("tool1", json.RawMessage(`{"a":1}`))
	key3 := DefaultCacheKey("tool1", json.RawMessage(`{"a":2}`))
	key4 := DefaultCacheKey("tool2", json.RawMessage(`{"a":1}`))

	if key1 != key2 {
		t.Error("same inputs should produce same key")
	}
	if key1 == key3 {
		t.Error("different args should produce different key")
	}
	if key1 == key4 {
		t.Error("different tool names should produce different key")
	}
}

// -----------------------------------------------------------------------------
// Validation Middleware Tests
// -----------------------------------------------------------------------------

func TestWithBasicValidation(t *testing.T) {
	tool := &mockTool{name: "validated_tool"}
	wrapped := ApplyMiddleware(tool, WithBasicValidation())

	// Valid JSON
	_, err := wrapped.Call(context.Background(), json.RawMessage(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("valid JSON should pass: %v", err)
	}

	// Invalid JSON
	_, err = wrapped.Call(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("invalid JSON should fail")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected invalid JSON error, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Metrics Middleware Tests
// -----------------------------------------------------------------------------

type mockMetricsCollector struct {
	mu    sync.Mutex
	calls []struct {
		toolName string
		duration time.Duration
		err      error
	}
}

func (m *mockMetricsCollector) RecordCall(toolName string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, struct {
		toolName string
		duration time.Duration
		err      error
	}{toolName, duration, err})
}

func TestWithMetrics(t *testing.T) {
	collector := &mockMetricsCollector{}

	tool := &mockTool{
		name: "metrics_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			time.Sleep(10 * time.Millisecond)
			return "done", nil
		},
	}
	wrapped := ApplyMiddleware(tool, WithMetrics(collector))

	_, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()

	if len(collector.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(collector.calls))
	}
	call := collector.calls[0]
	if call.toolName != "metrics_tool" {
		t.Errorf("toolName = %q, want %q", call.toolName, "metrics_tool")
	}
	if call.duration < 10*time.Millisecond {
		t.Errorf("duration = %v, expected >= 10ms", call.duration)
	}
	if call.err != nil {
		t.Errorf("err = %v, want nil", call.err)
	}
}

func TestWithMetricsRecordsErrors(t *testing.T) {
	collector := &mockMetricsCollector{}

	expectedErr := errors.New("tool failed")
	tool := &mockTool{
		name: "failing_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			return nil, expectedErr
		},
	}
	wrapped := ApplyMiddleware(tool, WithMetrics(collector))

	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()

	if len(collector.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(collector.calls))
	}
	if collector.calls[0].err != expectedErr {
		t.Errorf("err = %v, want %v", collector.calls[0].err, expectedErr)
	}
}

// -----------------------------------------------------------------------------
// Retry Middleware Tests
// -----------------------------------------------------------------------------

func TestWithRetry(t *testing.T) {
	attempts := 0
	tool := &mockTool{
		name: "flaky_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary error")
			}
			return "success", nil
		},
	}

	config := RetryConfig{
		MaxAttempts: 5,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2.0,
		Retryable: func(err error) bool {
			return strings.Contains(err.Error(), "temporary")
		},
	}

	wrapped := ApplyMiddleware(tool, WithRetry(config))

	result, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("result = %v, want %q", result, "success")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWithRetryMaxAttemptsExceeded(t *testing.T) {
	attempts := 0
	tool := &mockTool{
		name: "always_fail",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			attempts++
			return nil, errors.New("temporary error")
		},
	}

	config := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2.0,
		Retryable:   func(err error) bool { return true },
	}

	wrapped := ApplyMiddleware(tool, WithRetry(config))

	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("expected max attempts error, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestWithRetryNonRetryableError(t *testing.T) {
	attempts := 0
	tool := &mockTool{
		name: "non_retryable",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			attempts++
			return nil, errors.New("permanent error")
		},
	}

	config := RetryConfig{
		MaxAttempts: 5,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2.0,
		Retryable: func(err error) bool {
			return strings.Contains(err.Error(), "temporary")
		},
	}

	wrapped := ApplyMiddleware(tool, WithRetry(config))

	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should fail immediately without retry
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry for non-retryable error)", attempts)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", config.MaxAttempts)
	}
	if config.InitialWait != 100*time.Millisecond {
		t.Errorf("InitialWait = %v, want 100ms", config.InitialWait)
	}

	// Test default retryable function
	if !config.Retryable(context.DeadlineExceeded) {
		t.Error("should retry DeadlineExceeded")
	}
	if !config.Retryable(errors.New("temporary failure")) {
		t.Error("should retry temporary errors")
	}
	if config.Retryable(errors.New("permanent failure")) {
		t.Error("should not retry permanent errors")
	}
}

// -----------------------------------------------------------------------------
// Circuit Breaker Tests
// -----------------------------------------------------------------------------

func TestWithCircuitBreaker(t *testing.T) {
	var callCount int32

	tool := &mockTool{
		name: "breaker_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			atomic.AddInt32(&callCount, 1)
			return nil, errors.New("service unavailable")
		},
	}

	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenDuration:     50 * time.Millisecond,
	}

	wrapped := ApplyMiddleware(tool, WithCircuitBreaker(config))

	// First 3 calls should fail and open circuit
	for i := 0; i < 3; i++ {
		_, err := wrapped.Call(context.Background(), nil)
		if err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}

	// Circuit should now be open - calls should fail immediately
	_, err := wrapped.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected circuit open error")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}

	// Call count should not increase (circuit open)
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("callCount = %d, want 3 (circuit open)", callCount)
	}
}

func TestWithCircuitBreakerHalfOpen(t *testing.T) {
	callCount := 0
	tool := &mockTool{
		name: "recovering_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			// First 3 calls fail, then succeed
			if callCount <= 3 {
				return nil, errors.New("service unavailable")
			}
			return "success", nil
		},
	}

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		OpenDuration:     20 * time.Millisecond,
	}

	wrapped := ApplyMiddleware(tool, WithCircuitBreaker(config))

	// Trigger circuit open (2 failures)
	wrapped.Call(context.Background(), nil) // call 1: fail, failures=1
	wrapped.Call(context.Background(), nil) // call 2: fail, failures=2, opens circuit

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}

	// Circuit is now open - calls should fail immediately
	_, err := wrapped.Call(context.Background(), nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen while open, got: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (open circuit shouldn't call tool)", callCount)
	}

	// Wait for circuit to transition to half-open
	time.Sleep(30 * time.Millisecond)

	// Next call should go through (half-open) but fail, reopening circuit
	_, err = wrapped.Call(context.Background(), nil) // call 3: fail
	if err == nil || errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected tool error in half-open, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}

	// Circuit should be open again - immediate failure
	_, err = wrapped.Call(context.Background(), nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen after half-open failure, got: %v", err)
	}

	// Wait for half-open again
	time.Sleep(30 * time.Millisecond)

	// This time the tool succeeds (call 4)
	result, err := wrapped.Call(context.Background(), nil)
	if err != nil {
		t.Errorf("expected success in half-open recovery, got: %v", err)
	}
	if result != "success" {
		t.Errorf("result = %v, want %q", result, "success")
	}
}

func TestCircuitStateString(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tc := range tests {
		got := tc.state.String()
		if got != tc.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	if config.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", config.FailureThreshold)
	}
	if config.SuccessThreshold != 2 {
		t.Errorf("SuccessThreshold = %d, want 2", config.SuccessThreshold)
	}
	if config.OpenDuration != 30*time.Second {
		t.Errorf("OpenDuration = %v, want 30s", config.OpenDuration)
	}
}

// -----------------------------------------------------------------------------
// Conditional Middleware Tests
// -----------------------------------------------------------------------------

func TestForTools(t *testing.T) {
	callCount := 0
	middleware := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			return next(ctx, args)
		}
	}

	conditionalMw := ForTools([]string{"allowed_tool"}, middleware)

	// Tool that should have middleware applied
	tool1 := &mockTool{name: "allowed_tool"}
	wrapped1 := ApplyMiddleware(tool1, conditionalMw)
	_, _ = wrapped1.Call(context.Background(), nil)
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (middleware should run)", callCount)
	}

	// Tool that should NOT have middleware applied
	tool2 := &mockTool{name: "other_tool"}
	wrapped2 := ApplyMiddleware(tool2, conditionalMw)
	_, _ = wrapped2.Call(context.Background(), nil)
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (middleware should be skipped)", callCount)
	}
}

func TestExceptTools(t *testing.T) {
	callCount := 0
	middleware := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			return next(ctx, args)
		}
	}

	conditionalMw := ExceptTools([]string{"excluded_tool"}, middleware)

	// Tool that should have middleware applied
	tool1 := &mockTool{name: "normal_tool"}
	wrapped1 := ApplyMiddleware(tool1, conditionalMw)
	_, _ = wrapped1.Call(context.Background(), nil)
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (middleware should run)", callCount)
	}

	// Tool that should NOT have middleware applied
	tool2 := &mockTool{name: "excluded_tool"}
	wrapped2 := ApplyMiddleware(tool2, conditionalMw)
	_, _ = wrapped2.Call(context.Background(), nil)
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (middleware should be skipped)", callCount)
	}
}

// -----------------------------------------------------------------------------
// Registry Integration Tests
// -----------------------------------------------------------------------------

func TestRegistryWithMiddleware(t *testing.T) {
	callCount := 0
	middleware := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			callCount++
			return next(ctx, args)
		}
	}

	registry := NewRegistry(WithRegistryMiddleware(middleware))

	tool := &mockTool{name: "test_tool"}
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	_, err = registry.Execute(context.Background(), "test_tool", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestRegistryRegisterWithMiddleware(t *testing.T) {
	globalCount := 0
	perToolCount := 0

	globalMw := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			globalCount++
			return next(ctx, args)
		}
	}

	perToolMw := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			perToolCount++
			return next(ctx, args)
		}
	}

	registry := NewRegistry(WithRegistryMiddleware(globalMw))

	tool := &mockTool{name: "test_tool"}
	err := registry.RegisterWithMiddleware(tool, perToolMw)
	if err != nil {
		t.Fatalf("RegisterWithMiddleware error: %v", err)
	}

	_, err = registry.Execute(context.Background(), "test_tool", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Both should be called
	if globalCount != 1 {
		t.Errorf("globalCount = %d, want 1", globalCount)
	}
	if perToolCount != 1 {
		t.Errorf("perToolCount = %d, want 1", perToolCount)
	}
}

func TestRegistryNoMiddleware(t *testing.T) {
	// Registry without middleware should work as before
	registry := NewRegistry()

	tool := &mockTool{name: "test_tool"}
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	result, err := registry.Execute(context.Background(), "test_tool", nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result != "result" {
		t.Errorf("result = %v, want %q", result, "result")
	}
}

// -----------------------------------------------------------------------------
// Middleware Ordering Tests
// -----------------------------------------------------------------------------

func TestMiddlewareOrdering(t *testing.T) {
	var order []string

	m1 := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			order = append(order, "global-before")
			result, err := next(ctx, args)
			order = append(order, "global-after")
			return result, err
		}
	}

	m2 := func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			order = append(order, "per-tool-before")
			result, err := next(ctx, args)
			order = append(order, "per-tool-after")
			return result, err
		}
	}

	registry := NewRegistry(WithRegistryMiddleware(m1))

	tool := &mockTool{
		name: "test_tool",
		callFn: func(ctx context.Context, args json.RawMessage) (any, error) {
			order = append(order, "tool")
			return nil, nil
		},
	}
	registry.RegisterWithMiddleware(tool, m2)
	registry.Execute(context.Background(), "test_tool", nil)

	// Global wraps per-tool, so order should be:
	// global-before -> per-tool-before -> tool -> per-tool-after -> global-after
	expected := []string{
		"global-before",
		"per-tool-before",
		"tool",
		"per-tool-after",
		"global-after",
	}

	if len(order) != len(expected) {
		t.Fatalf("order length = %d, want %d\norder: %v", len(order), len(expected), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q\nfull order: %v", i, order[i], v, order)
		}
	}
}
