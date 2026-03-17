package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/petal-labs/iris/core"
)

// Compile-time interface checks.
var (
	_ core.TelemetryHook           = (*Hook)(nil)
	_ core.ContextualTelemetryHook = (*Hook)(nil)
)

// Hook implements core.ContextualTelemetryHook for OpenTelemetry integration.
// It creates spans for LLM requests following GenAI semantic conventions.
type Hook struct {
	tracer trace.Tracer
	config Config
}

// Config configures the Hook behavior.
type Config struct {
	// TracerName is the name used for the tracer.
	// Defaults to "github.com/petal-labs/iris".
	TracerName string

	// TracerProvider is the OTel tracer provider to use.
	// If nil, uses the global provider via otel.GetTracerProvider().
	TracerProvider trace.TracerProvider

	// SpanNameFunc customizes span names.
	// If nil, defaults to "chat {model}".
	SpanNameFunc func(e core.RequestStartEvent) string

	// RecordError controls whether errors are recorded on spans.
	// Defaults to true.
	RecordError bool

	// AdditionalAttributes adds custom attributes to every span.
	AdditionalAttributes []attribute.KeyValue
}

// Option configures a Hook.
type Option func(*Config)

// New creates a new Hook with the given options.
func New(opts ...Option) *Hook {
	cfg := Config{
		TracerName:  "github.com/petal-labs/iris",
		RecordError: true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	tp := cfg.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}

	return &Hook{
		tracer: tp.Tracer(cfg.TracerName),
		config: cfg,
	}
}

// WithTracerProvider sets a custom tracer provider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *Config) {
		c.TracerProvider = tp
	}
}

// WithTracerName sets the tracer name.
func WithTracerName(name string) Option {
	return func(c *Config) {
		c.TracerName = name
	}
}

// WithSpanNameFunc sets a custom span name function.
func WithSpanNameFunc(fn func(core.RequestStartEvent) string) Option {
	return func(c *Config) {
		c.SpanNameFunc = fn
	}
}

// WithAttributes adds additional attributes to all spans.
func WithAttributes(attrs ...attribute.KeyValue) Option {
	return func(c *Config) {
		c.AdditionalAttributes = append(c.AdditionalAttributes, attrs...)
	}
}

// WithRecordError controls whether errors are recorded on spans.
func WithRecordError(record bool) Option {
	return func(c *Config) {
		c.RecordError = record
	}
}

// OnRequestStart implements core.TelemetryHook.
// This is a no-op because without context, proper spans cannot be created.
// Use OnRequestStartWithContext instead.
func (h *Hook) OnRequestStart(e core.RequestStartEvent) {
	// No-op: without context, we cannot create proper spans.
	// The Client will detect ContextualTelemetryHook and use the context methods.
}

// OnRequestEnd implements core.TelemetryHook.
// This is a no-op because without context, the span cannot be accessed.
// Use OnRequestEndWithContext instead.
func (h *Hook) OnRequestEnd(e core.RequestEndEvent) {
	// No-op: without context, we cannot access the span.
}

// OnRequestStartWithContext implements core.ContextualTelemetryHook.
// It creates a new span and returns a context containing the span.
func (h *Hook) OnRequestStartWithContext(ctx context.Context, e core.RequestStartEvent) context.Context {
	spanName := h.spanName(e)

	attrs := []attribute.KeyValue{
		GenAISystem.String(e.Provider),
		GenAIRequestModel.String(string(e.Model)),
	}
	attrs = append(attrs, h.config.AdditionalAttributes...)

	ctx, _ = h.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)

	return ctx
}

// OnRequestEndWithContext implements core.ContextualTelemetryHook.
// It ends the span from the context and adds final attributes.
func (h *Hook) OnRequestEndWithContext(ctx context.Context, e core.RequestEndEvent) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	// Add usage attributes
	span.SetAttributes(
		GenAIUsageInputTokens.Int(e.Usage.PromptTokens),
		GenAIUsageOutputTokens.Int(e.Usage.CompletionTokens),
	)

	// Handle errors
	if e.Err != nil {
		if h.config.RecordError {
			span.RecordError(e.Err)
		}
		span.SetStatus(codes.Error, e.Err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.End()
}

func (h *Hook) spanName(e core.RequestStartEvent) string {
	if h.config.SpanNameFunc != nil {
		return h.config.SpanNameFunc(e)
	}
	return "chat " + string(e.Model)
}
