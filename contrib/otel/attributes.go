package otel

import "go.opentelemetry.io/otel/attribute"

// GenAI semantic convention attribute keys.
// These follow the OpenTelemetry Semantic Conventions for GenAI.
// See: https://opentelemetry.io/docs/specs/semconv/gen-ai/
var (
	// GenAISystem identifies the GenAI provider system.
	// Examples: "openai", "anthropic", "google", "ollama"
	GenAISystem = attribute.Key("gen_ai.system")

	// GenAIRequestModel identifies the model requested.
	// Examples: "gpt-4o", "claude-sonnet-4-20250514", "gemini-2.0-flash"
	GenAIRequestModel = attribute.Key("gen_ai.request.model")

	// GenAIUsageInputTokens is the number of input tokens consumed.
	GenAIUsageInputTokens = attribute.Key("gen_ai.usage.input_tokens")

	// GenAIUsageOutputTokens is the number of output tokens generated.
	GenAIUsageOutputTokens = attribute.Key("gen_ai.usage.output_tokens")

	// GenAIResponseFinishReason is the reason the model stopped generating.
	// Examples: "stop", "length", "tool_calls", "content_filter"
	GenAIResponseFinishReason = attribute.Key("gen_ai.response.finish_reason")
)

// Iris-specific attribute keys for additional telemetry.
var (
	// IrisStreamMode indicates whether the request was streaming.
	IrisStreamMode = attribute.Key("iris.stream_mode")
)
