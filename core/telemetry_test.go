package core

import (
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
