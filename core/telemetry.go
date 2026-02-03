package core

import "time"

// TelemetryHook receives notifications about request lifecycle events.
// Implementations can use this for logging, metrics, tracing, etc.
//
// # Security Considerations
//
// Event types are designed to NEVER include sensitive data:
//   - API keys are NEVER included (stored separately as core.Secret)
//   - Prompt content (user messages) is NEVER included
//   - Response content (model outputs) is NEVER included
//   - Only operational metadata is exposed (provider, model, timing, token counts)
//
// This design ensures that telemetry data can be safely:
//   - Logged to disk without risk of credential exposure
//   - Sent to external monitoring systems
//   - Aggregated for analytics
//   - Stored long-term for debugging
//
// If extending this interface, maintain these security properties.
// New event types must undergo security review before merge.
// Never add fields that could contain: API keys, user prompts, model responses,
// or any other potentially sensitive content.
type TelemetryHook interface {
	// OnRequestStart is called when a request to a provider begins.
	OnRequestStart(e RequestStartEvent)

	// OnRequestEnd is called when a request to a provider completes.
	OnRequestEnd(e RequestEndEvent)
}

// RequestStartEvent contains metadata about a starting request.
//
// # Security
//
// This struct intentionally excludes:
//   - API keys (never logged)
//   - Prompt/message content (privacy sensitive)
//   - Request headers (may contain auth tokens)
//
// Only operational metadata suitable for logging is included.
type RequestStartEvent struct {
	Provider string    // Provider identifier (e.g., "openai", "anthropic")
	Model    ModelID   // Model being called
	Start    time.Time // When the request started
}

// RequestEndEvent contains metadata about a completed request.
//
// # Security
//
// This struct intentionally excludes:
//   - API keys (never logged)
//   - Response content (may be sensitive)
//   - Raw HTTP response data
//
// The Err field contains error types, not raw error messages from providers
// which might inadvertently include sensitive data.
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
