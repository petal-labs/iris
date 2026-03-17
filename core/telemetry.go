package core

import (
	"context"
	"time"
)

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

// ContextualTelemetryHook extends TelemetryHook with context support.
// Implementations that need access to context.Context (e.g., OpenTelemetry
// for span creation and propagation) should implement this interface.
//
// The Client checks for this interface at runtime and uses it when available,
// falling back to the base TelemetryHook methods otherwise. This ensures
// backward compatibility with existing TelemetryHook implementations.
//
// # Context Propagation
//
// OnRequestStartWithContext returns a new context that may contain span
// information or other telemetry-related data. The returned context is
// passed through the request lifecycle and provided to OnRequestEndWithContext.
//
// # Security
//
// Like TelemetryHook, implementations must NOT capture sensitive data
// (API keys, prompts, responses) in spans or other telemetry output.
type ContextualTelemetryHook interface {
	TelemetryHook

	// OnRequestStartWithContext is called when a request to a provider begins.
	// Returns a new context that should be used for the request.
	// The returned context may contain span information for propagation.
	OnRequestStartWithContext(ctx context.Context, e RequestStartEvent) context.Context

	// OnRequestEndWithContext is called when a request to a provider completes.
	OnRequestEndWithContext(ctx context.Context, e RequestEndEvent)
}
