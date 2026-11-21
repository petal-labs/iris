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

	// Voice and speech operational attribute keys (never carry audio bytes,
	// transcripts, prompts, or credentials).
	IrisVoiceOperation          = attribute.Key("iris.voice.operation")
	IrisVoiceSessionID          = attribute.Key("iris.voice.session_id")
	IrisVoiceTurnID             = attribute.Key("iris.voice.turn_id")
	IrisVoiceMode               = attribute.Key("iris.voice.mode")
	IrisVoiceAudioEncoding      = attribute.Key("iris.voice.audio.encoding")
	IrisVoiceAudioSampleRate    = attribute.Key("iris.voice.audio.sample_rate")
	IrisVoiceAudioDirection     = attribute.Key("iris.voice.audio.direction")
	IrisVoiceInputBytes         = attribute.Key("iris.voice.input_bytes")
	IrisVoiceOutputBytes        = attribute.Key("iris.voice.output_bytes")
	IrisVoiceInputCharacters    = attribute.Key("iris.voice.input_characters")
	IrisVoiceAudioChunks        = attribute.Key("iris.voice.audio_chunks")
	IrisVoiceFirstChunkMS       = attribute.Key("iris.voice.first_chunk_ms")
	IrisVoiceInterrupted        = attribute.Key("iris.voice.interrupted")
	IrisVoiceInterruptionReason = attribute.Key("iris.voice.interruption_reason")
	IrisVoiceTurns              = attribute.Key("iris.voice.turns")
	IrisVoiceInterruptions      = attribute.Key("iris.voice.interruptions")
	IrisVoiceToolCalls          = attribute.Key("iris.voice.tool_calls")
	IrisVoiceErrorCount         = attribute.Key("iris.voice.error_count")
)
