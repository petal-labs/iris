package otel

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/petal-labs/iris/core"
)

func TestHookImplementsInterfaces(t *testing.T) {
	hook := New()

	// Should implement TelemetryHook
	var _ core.TelemetryHook = hook

	// Should implement ContextualTelemetryHook
	var _ core.ContextualTelemetryHook = hook
}

func TestHookCreatesSpans(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(WithTracerProvider(tp))

	ctx := context.Background()
	startEvent := core.RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    time.Now(),
	}

	ctx = hook.OnRequestStartWithContext(ctx, startEvent)

	endEvent := core.RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    startEvent.Start,
		End:      time.Now(),
		Usage: core.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	hook.OnRequestEndWithContext(ctx, endEvent)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]

	// Verify span name
	if span.Name() != "chat gpt-4o" {
		t.Errorf("span name = %q, want %q", span.Name(), "chat gpt-4o")
	}

	// Verify span kind
	if span.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind = %v, want %v", span.SpanKind(), trace.SpanKindClient)
	}

	// Verify status
	if span.Status().Code != codes.Ok {
		t.Errorf("span status = %v, want %v", span.Status().Code, codes.Ok)
	}

	// Verify attributes
	attrs := span.Attributes()
	assertAttribute(t, attrs, "gen_ai.system", "openai")
	assertAttribute(t, attrs, "gen_ai.request.model", "gpt-4o")
	assertAttributeInt(t, attrs, "gen_ai.usage.input_tokens", 100)
	assertAttributeInt(t, attrs, "gen_ai.usage.output_tokens", 50)
}

func TestHookRecordsErrors(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(WithTracerProvider(tp))

	ctx := context.Background()
	startEvent := core.RequestStartEvent{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
		Start:    time.Now(),
	}

	ctx = hook.OnRequestStartWithContext(ctx, startEvent)

	testErr := errors.New("rate limited")
	endEvent := core.RequestEndEvent{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
		Start:    startEvent.Start,
		End:      time.Now(),
		Err:      testErr,
	}

	hook.OnRequestEndWithContext(ctx, endEvent)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]

	// Verify error status
	if span.Status().Code != codes.Error {
		t.Errorf("span status = %v, want %v", span.Status().Code, codes.Error)
	}

	// Verify error was recorded
	events := span.Events()
	foundError := false
	for _, e := range events {
		if e.Name == "exception" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error event to be recorded")
	}
}

func TestHookWithRecordErrorFalse(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(WithTracerProvider(tp), WithRecordError(false))

	ctx := context.Background()
	startEvent := core.RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    time.Now(),
	}

	ctx = hook.OnRequestStartWithContext(ctx, startEvent)

	endEvent := core.RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    startEvent.Start,
		End:      time.Now(),
		Err:      errors.New("some error"),
	}

	hook.OnRequestEndWithContext(ctx, endEvent)

	spans := sr.Ended()
	span := spans[0]

	// Status should still be error
	if span.Status().Code != codes.Error {
		t.Errorf("span status = %v, want %v", span.Status().Code, codes.Error)
	}

	// But no error event should be recorded
	for _, e := range span.Events() {
		if e.Name == "exception" {
			t.Error("error event should not be recorded when WithRecordError(false)")
		}
	}
}

func TestHookWithCustomSpanName(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(
		WithTracerProvider(tp),
		WithSpanNameFunc(func(e core.RequestStartEvent) string {
			return "llm/" + e.Provider + "/" + string(e.Model)
		}),
	)

	ctx := context.Background()
	startEvent := core.RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    time.Now(),
	}

	ctx = hook.OnRequestStartWithContext(ctx, startEvent)
	hook.OnRequestEndWithContext(ctx, core.RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    startEvent.Start,
		End:      time.Now(),
	})

	spans := sr.Ended()
	span := spans[0]

	if span.Name() != "llm/openai/gpt-4o" {
		t.Errorf("span name = %q, want %q", span.Name(), "llm/openai/gpt-4o")
	}
}

func TestHookWithAdditionalAttributes(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(
		WithTracerProvider(tp),
		WithAttributes(
			attribute.String("service.name", "my-service"),
			attribute.String("deployment.environment", "production"),
		),
	)

	ctx := context.Background()
	startEvent := core.RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    time.Now(),
	}

	ctx = hook.OnRequestStartWithContext(ctx, startEvent)
	hook.OnRequestEndWithContext(ctx, core.RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    startEvent.Start,
		End:      time.Now(),
	})

	spans := sr.Ended()
	span := spans[0]

	attrs := span.Attributes()
	assertAttribute(t, attrs, "service.name", "my-service")
	assertAttribute(t, attrs, "deployment.environment", "production")
}

func TestHookWithCustomTracerName(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(
		WithTracerProvider(tp),
		WithTracerName("my-app/llm-client"),
	)

	ctx := context.Background()
	startEvent := core.RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    time.Now(),
	}

	ctx = hook.OnRequestStartWithContext(ctx, startEvent)
	hook.OnRequestEndWithContext(ctx, core.RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    startEvent.Start,
		End:      time.Now(),
	})

	spans := sr.Ended()
	span := spans[0]

	// The instrumentation scope name should be our custom tracer name
	if span.InstrumentationScope().Name != "my-app/llm-client" {
		t.Errorf("tracer name = %q, want %q", span.InstrumentationScope().Name, "my-app/llm-client")
	}
}

func TestHookNoopMethodsDoNotPanic(t *testing.T) {
	hook := New()

	// These should be no-ops but should not panic
	hook.OnRequestStart(core.RequestStartEvent{
		Provider: "test",
		Model:    "test-model",
		Start:    time.Now(),
	})

	hook.OnRequestEnd(core.RequestEndEvent{
		Provider: "test",
		Model:    "test-model",
		Start:    time.Now(),
		End:      time.Now(),
	})
}

func TestHookContextPropagation(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	hook := New(WithTracerProvider(tp))

	// Create a parent context with a span
	tracer := tp.Tracer("test")
	parentCtx, parentSpan := tracer.Start(context.Background(), "parent-operation")

	// Start an LLM request - it should be a child of the parent span
	startEvent := core.RequestStartEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    time.Now(),
	}

	childCtx := hook.OnRequestStartWithContext(parentCtx, startEvent)
	hook.OnRequestEndWithContext(childCtx, core.RequestEndEvent{
		Provider: "openai",
		Model:    "gpt-4o",
		Start:    startEvent.Start,
		End:      time.Now(),
	})

	parentSpan.End()

	spans := sr.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Find the child span (the LLM call)
	var childSpanData sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == "chat gpt-4o" {
			childSpanData = s
			break
		}
	}

	// Verify parent-child relationship
	if childSpanData.Parent().SpanID() != parentSpan.SpanContext().SpanID() {
		t.Error("LLM span should be a child of the parent span")
	}
}

// Helper functions

func assertAttribute(t *testing.T, attrs []attribute.KeyValue, key, expected string) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			if attr.Value.AsString() != expected {
				t.Errorf("attribute %q = %q, want %q", key, attr.Value.AsString(), expected)
			}
			return
		}
	}
	t.Errorf("attribute %q not found", key)
}

func assertAttributeInt(t *testing.T, attrs []attribute.KeyValue, key string, expected int64) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			if attr.Value.AsInt64() != expected {
				t.Errorf("attribute %q = %d, want %d", key, attr.Value.AsInt64(), expected)
			}
			return
		}
	}
	t.Errorf("attribute %q not found", key)
}
