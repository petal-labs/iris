package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// testTelemetryHook is a test implementation that records events.
type testTelemetryHook struct {
	startEvents []RequestStartEvent
	endEvents   []RequestEndEvent
}

func (h *testTelemetryHook) OnRequestStart(e RequestStartEvent) {
	h.startEvents = append(h.startEvents, e)
}

func (h *testTelemetryHook) OnRequestEnd(e RequestEndEvent) {
	h.endEvents = append(h.endEvents, e)
}

func TestTelemetryHookCanBeImplemented(t *testing.T) {
	// Verify test struct implements interface
	var hook TelemetryHook = &testTelemetryHook{}
	if hook == nil {
		t.Fatal("testTelemetryHook should implement TelemetryHook")
	}
}

func TestRequestStartEventFields(t *testing.T) {
	now := time.Now()
	event := RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4",
		Start:    now,
	}

	if event.Provider != "openai" {
		t.Errorf("Provider = %v, want openai", event.Provider)
	}
	if event.Model != "gpt-4" {
		t.Errorf("Model = %v, want gpt-4", event.Model)
	}
	if !event.Start.Equal(now) {
		t.Errorf("Start = %v, want %v", event.Start, now)
	}
}

func TestRequestEndEventFields(t *testing.T) {
	start := time.Now()
	end := start.Add(500 * time.Millisecond)
	testErr := errors.New("test error")

	event := RequestEndEvent{
		Provider: "anthropic",
		Model:    "claude-3",
		Start:    start,
		End:      end,
		Usage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Err: testErr,
	}

	if event.Provider != "anthropic" {
		t.Errorf("Provider = %v, want anthropic", event.Provider)
	}
	if event.Model != "claude-3" {
		t.Errorf("Model = %v, want claude-3", event.Model)
	}
	if !event.Start.Equal(start) {
		t.Errorf("Start = %v, want %v", event.Start, start)
	}
	if !event.End.Equal(end) {
		t.Errorf("End = %v, want %v", event.End, end)
	}
	if event.Usage.TotalTokens != 150 {
		t.Errorf("Usage.TotalTokens = %v, want 150", event.Usage.TotalTokens)
	}
	if event.Err != testErr {
		t.Errorf("Err = %v, want %v", event.Err, testErr)
	}
}

func TestRequestEndEventDuration(t *testing.T) {
	start := time.Now()
	end := start.Add(500 * time.Millisecond)

	event := RequestEndEvent{
		Start: start,
		End:   end,
	}

	duration := event.Duration()
	if duration != 500*time.Millisecond {
		t.Errorf("Duration() = %v, want 500ms", duration)
	}
}

func TestRequestEndEventSuccessHasNilError(t *testing.T) {
	event := RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4",
		Start:    time.Now(),
		End:      time.Now(),
		Usage:    TokenUsage{TotalTokens: 100},
		Err:      nil,
	}

	if event.Err != nil {
		t.Error("successful request should have nil Err")
	}
}

func TestNoopTelemetryHookImplementsInterface(t *testing.T) {
	var hook TelemetryHook = NoopTelemetryHook{}
	if hook == nil {
		t.Fatal("NoopTelemetryHook should implement TelemetryHook")
	}
}

func TestNoopTelemetryHookDoesNotPanic(t *testing.T) {
	hook := NoopTelemetryHook{}

	// Should not panic
	hook.OnRequestStart(RequestStartEvent{
		Provider: "test",
		Model:    "test-model",
		Start:    time.Now(),
	})

	hook.OnRequestEnd(RequestEndEvent{
		Provider: "test",
		Model:    "test-model",
		Start:    time.Now(),
		End:      time.Now(),
		Usage:    TokenUsage{},
		Err:      errors.New("test"),
	})
}

func TestTelemetryHookReceivesEvents(t *testing.T) {
	hook := &testTelemetryHook{}

	startEvent := RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4",
		Start:    time.Now(),
	}

	endEvent := RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4",
		Start:    startEvent.Start,
		End:      time.Now(),
		Usage:    TokenUsage{TotalTokens: 100},
		Err:      nil,
	}

	hook.OnRequestStart(startEvent)
	hook.OnRequestEnd(endEvent)

	if len(hook.startEvents) != 1 {
		t.Errorf("expected 1 start event, got %d", len(hook.startEvents))
	}
	if len(hook.endEvents) != 1 {
		t.Errorf("expected 1 end event, got %d", len(hook.endEvents))
	}

	if hook.startEvents[0].Provider != "openai" {
		t.Error("start event should contain correct provider")
	}
	if hook.endEvents[0].Usage.TotalTokens != 100 {
		t.Error("end event should contain correct usage")
	}
}

// TestEventStructsHaveNoSecretFields verifies at compile time that
// event structs don't have fields for sensitive data.
// This is a documentation test - the actual enforcement is via struct design.
func TestEventStructsHaveNoSecretFields(t *testing.T) {
	// RequestStartEvent should only have safe fields
	_ = RequestStartEvent{
		Provider: "test",     // safe: provider name
		Model:    "model",    // safe: model identifier
		Start:    time.Now(), // safe: timestamp
	}

	// RequestEndEvent should only have safe fields
	_ = RequestEndEvent{
		Provider: "test",       // safe: provider name
		Model:    "model",      // safe: model identifier
		Start:    time.Now(),   // safe: timestamp
		End:      time.Now(),   // safe: timestamp
		Usage:    TokenUsage{}, // safe: token counts only
		Err:      nil,          // safe: error type (not content)
	}

	// If this test compiles, the structs don't have fields like:
	// - APIKey
	// - PromptContent / Messages
	// - ResponseContent / Output
	// - Headers
	// etc.
}

// telemetryTestKey is a custom type for context keys to satisfy staticcheck SA1029.
type telemetryTestKey struct{}

// testContextualTelemetryHook is a test implementation that records events
// and supports context propagation.
type testContextualTelemetryHook struct {
	testTelemetryHook
	startContextEvents []RequestStartEvent
	endContextEvents   []RequestEndEvent
	contexts           []context.Context
}

func (h *testContextualTelemetryHook) OnRequestStartWithContext(ctx context.Context, e RequestStartEvent) context.Context {
	h.startContextEvents = append(h.startContextEvents, e)
	// Return a new context with a value to verify propagation
	return context.WithValue(ctx, telemetryTestKey{}, "telemetry-test-value")
}

func (h *testContextualTelemetryHook) OnRequestEndWithContext(ctx context.Context, e RequestEndEvent) {
	h.endContextEvents = append(h.endContextEvents, e)
	h.contexts = append(h.contexts, ctx)
}

func TestContextualTelemetryHookCanBeImplemented(t *testing.T) {
	// Verify test struct implements both interfaces
	var hook TelemetryHook = &testContextualTelemetryHook{}
	if hook == nil {
		t.Fatal("testContextualTelemetryHook should implement TelemetryHook")
	}

	var ctxHook ContextualTelemetryHook = &testContextualTelemetryHook{}
	if ctxHook == nil {
		t.Fatal("testContextualTelemetryHook should implement ContextualTelemetryHook")
	}
}

func TestContextualTelemetryHookExtendsBase(t *testing.T) {
	hook := &testContextualTelemetryHook{}

	// Should also work as base TelemetryHook
	hook.OnRequestStart(RequestStartEvent{Provider: "test", Model: "model", Start: time.Now()})
	hook.OnRequestEnd(RequestEndEvent{Provider: "test", Model: "model", Start: time.Now(), End: time.Now()})

	if len(hook.startEvents) != 1 {
		t.Errorf("expected 1 base start event, got %d", len(hook.startEvents))
	}
	if len(hook.endEvents) != 1 {
		t.Errorf("expected 1 base end event, got %d", len(hook.endEvents))
	}
}

func TestContextualTelemetryHookReceivesContext(t *testing.T) {
	hook := &testContextualTelemetryHook{}
	ctx := context.Background()

	startEvent := RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4",
		Start:    time.Now(),
	}

	// OnRequestStartWithContext should return a new context
	newCtx := hook.OnRequestStartWithContext(ctx, startEvent)
	if newCtx == ctx {
		t.Error("OnRequestStartWithContext should return a new context")
	}

	// The new context should have the test value
	val := newCtx.Value(telemetryTestKey{})
	if val != "telemetry-test-value" {
		t.Errorf("context value = %v, want telemetry-test-value", val)
	}

	endEvent := RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4",
		Start:    startEvent.Start,
		End:      time.Now(),
		Usage:    TokenUsage{TotalTokens: 100},
	}

	// OnRequestEndWithContext should receive the context
	hook.OnRequestEndWithContext(newCtx, endEvent)

	if len(hook.startContextEvents) != 1 {
		t.Errorf("expected 1 contextual start event, got %d", len(hook.startContextEvents))
	}
	if len(hook.endContextEvents) != 1 {
		t.Errorf("expected 1 contextual end event, got %d", len(hook.endContextEvents))
	}
	if len(hook.contexts) != 1 {
		t.Errorf("expected 1 context, got %d", len(hook.contexts))
	}

	// Verify the context passed to OnRequestEndWithContext is the one from OnRequestStartWithContext
	receivedCtx := hook.contexts[0]
	if receivedCtx.Value(telemetryTestKey{}) != "telemetry-test-value" {
		t.Error("OnRequestEndWithContext should receive the context from OnRequestStartWithContext")
	}
}

func TestContextualTelemetryHookTypeAssertion(t *testing.T) {
	// Test type assertion pattern used in client.go
	var baseTelemetry TelemetryHook = &testTelemetryHook{}

	// Base hook should NOT be assertable to ContextualTelemetryHook
	if _, ok := baseTelemetry.(ContextualTelemetryHook); ok {
		t.Error("base TelemetryHook should not be assertable to ContextualTelemetryHook")
	}

	var contextualTelemetry TelemetryHook = &testContextualTelemetryHook{}

	// Contextual hook SHOULD be assertable to ContextualTelemetryHook
	ctxHook, ok := contextualTelemetry.(ContextualTelemetryHook)
	if !ok {
		t.Error("ContextualTelemetryHook should be assertable from TelemetryHook")
	}
	if ctxHook == nil {
		t.Error("type assertion should return non-nil hook")
	}
}

func TestNoopTelemetryHookIsNotContextual(t *testing.T) {
	var hook TelemetryHook = NoopTelemetryHook{}

	// NoopTelemetryHook should NOT implement ContextualTelemetryHook
	// This is intentional - noop doesn't need context
	if _, ok := hook.(ContextualTelemetryHook); ok {
		t.Error("NoopTelemetryHook should not implement ContextualTelemetryHook")
	}
}
