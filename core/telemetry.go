package core

import "time"

// TelemetryHook receives notifications about request lifecycle events.
// Implementations can use this for logging, metrics, tracing, etc.
//
// Security: Event types intentionally exclude sensitive data.
// They NEVER include API keys, prompt content, or response content.
type TelemetryHook interface {
	// OnRequestStart is called when a request to a provider begins.
	OnRequestStart(e RequestStartEvent)

	// OnRequestEnd is called when a request to a provider completes.
	OnRequestEnd(e RequestEndEvent)
}

// RequestStartEvent contains metadata about a starting request.
// Security: This struct intentionally excludes prompt content and API keys.
type RequestStartEvent struct {
	Provider string    // Provider identifier (e.g., "openai", "anthropic")
	Model    ModelID   // Model being called
	Start    time.Time // When the request started
}

// RequestEndEvent contains metadata about a completed request.
// Security: This struct intentionally excludes response content and API keys.
type RequestEndEvent struct {
	Provider string     // Provider identifier
	Model    ModelID    // Model that was called
	Start    time.Time  // When the request started
	End      time.Time  // When the request completed
	Usage    TokenUsage // Token consumption
	Err      error      // Error if request failed, nil on success
}

// Duration returns the elapsed time for the request.
func (e RequestEndEvent) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// NoopTelemetryHook is a no-op implementation of TelemetryHook.
// Use this as a default when no telemetry is configured.
type NoopTelemetryHook struct{}

// OnRequestStart does nothing.
func (NoopTelemetryHook) OnRequestStart(RequestStartEvent) {}

// OnRequestEnd does nothing.
func (NoopTelemetryHook) OnRequestEnd(RequestEndEvent) {}

// Compile-time check that NoopTelemetryHook implements TelemetryHook.
var _ TelemetryHook = NoopTelemetryHook{}
