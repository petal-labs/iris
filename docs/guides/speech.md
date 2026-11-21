# Speech and Audio (ElevenLabs)

Iris exposes provider-agnostic audio types and optional speech interfaces in `core` so callers can synthesize and transcribe speech without coupling to provider wire protocols. ElevenLabs is the first speech provider implementation (`providers/elevenlabs`).

Audio capture and playback remain application-side responsibilities: Iris accepts and produces encoded/raw audio bytes and incremental chunks, not microphone or speaker drivers.

## Environment Configuration

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `ELEVENLABS_API_KEY` | Yes (for `NewFromEnv`) | — | API key sent in the `xi-api-key` HTTP header (stored as `core.Secret`) |
| `ELEVENLABS_VOICE_ID` | No (examples & live tests) | `JBFqnCBsd6RMkjVDRZzb` | Default voice ID used by examples and `//go:build integration` tests |

Create a provider from the environment or with explicit options:

```go
provider, err := elevenlabs.NewFromEnv(
	elevenlabs.WithTimeout(30*time.Second),
	elevenlabs.WithStreamChunkSize(16*1024),
	elevenlabs.WithStreamBufferSize(16),
)
if err != nil {
	log.Fatal(err)
}
client := core.NewClient(provider)
```

Available options: `WithAPIKey`, `WithBaseURL`, `WithHTTPClient`, `WithHeader`, `WithTimeout`, `WithStreamChunkSize`, and `WithStreamBufferSize`.

## Capability Discovery

Existing `core.Provider` implementations do not need to implement speech interfaces. Discover speech capabilities at runtime (walking `core.ProviderUnwrapper` chains):

```go
if _, ok := core.AsSpeechSynthesizer(provider); !ok {
	return core.ErrNotSupported
}
if !core.ProviderSupportsModelFeature(provider, elevenlabs.ModelElevenFlashV25, core.FeatureSpeechSynthesisStreaming) {
	return core.ErrNotSupported
}
```

Speech `Feature` constants:
- `core.FeatureSpeechSynthesis` — synchronous text-to-speech (`Synthesize`)
- `core.FeatureSpeechSynthesisStreaming` — streaming text-to-speech (`StreamSynthesize`)
- `core.FeatureSpeechSynthesisRealtime` — incremental-input WebSocket text-to-speech (`StartRealtimeSynthesis`)
- `core.FeatureSpeechRecognition` — file/reader speech-to-text (`Transcribe`)
- `core.FeatureSpeechRecognitionRealtime` — real-time streaming STT (`StartRealtimeTranscription`)
- `core.FeatureVoiceSession` — full-duplex real-time voice sessions (`StartVoiceSession`)

## Verified Models and Capabilities

| Model Constant | Model ID | Sync TTS | Streaming TTS | Realtime WS TTS | File/Reader STT | Realtime STT | Notes |
|---|---|---|---|---|---|---|---|
| `elevenlabs.ModelElevenMultilingualV2` | `eleven_multilingual_v2` | Yes | Yes | Yes | No | No | Default sync/HTTP TTS model; 29 languages |
| `elevenlabs.ModelElevenFlashV25` | `eleven_flash_v2_5` | Yes | Yes | Yes | No | No | Default real-time WS TTS model; low-latency multilingual TTS |
| `elevenlabs.ModelElevenV3` | `eleven_v3` | Yes | Yes | No | No | No | Expressive multi-speaker TTS (incompatible with `/stream-input` WebSocket) |
| `elevenlabs.ModelScribeV1` | `scribe_v1` | No | No | No | Yes | No | Batch STT (`keyterms` unsupported) |
| `elevenlabs.ModelScribeV2` | `scribe_v2` | No | No | No | Yes | No | Default batch STT model; diarization, timestamps, `keyterms`, audio events |
| `elevenlabs.ModelScribeV2Realtime` | `scribe_v2_realtime` | No | No | No | No | Yes | Default real-time WebSocket STT (partial + committed transcripts) |

Unverified model/feature combinations are never optimistically advertised as supported (`core.ProviderSupportsModelFeature` returns `false` for unknown models).

## Synchronous Text-to-Speech

```go
resp, err := client.Synthesize(ctx, &core.SpeechRequest{
	Model:   elevenlabs.ModelElevenFlashV25,
	VoiceID: "JBFqnCBsd6RMkjVDRZzb",
	Text:    "Hello from Iris.",
	Format:  core.AudioFormatMP3_44100_128,
})
if err != nil {
	return err
}
audioBytes, err := resp.Audio.GetBytes()
```

Supported ElevenLabs TTS output formats (mono / 1 channel only):
- **MP3**: `22050Hz` @ `32kbps`; `44100Hz` @ `32`, `64`, `96`, `128` (default), or `192kbps`
- **Raw PCM**: `8000`, `16000`, `22050`, `24000`, `44100`, or `48000Hz`
- **G.711 telephony**: `ulaw` @ `8000Hz`, `alaw` @ `8000Hz`
- **Opus**: `48000Hz` @ `32`, `64`, `96`, `128`, or `192kbps`

## HTTP Streaming Text-to-Speech

```go
stream, err := client.StreamSynthesize(ctx, &core.SpeechRequest{
	Model:   elevenlabs.ModelElevenFlashV25,
	VoiceID: "JBFqnCBsd6RMkjVDRZzb",
	Text:    "Streaming audio from Iris.",
	Format:  core.AudioFormatPCM16000,
})
if err != nil {
	return err
}

for chunk := range stream.Ch {
	if len(chunk.Data) > 0 {
		// Forward chunk.Data to playback or downstream sink immediately.
	}
}
if err := <-stream.Err; err != nil {
	return err
}
meta := <-stream.Final // metadata + Usage.OutputBytes (Audio.Data omitted to keep memory bounded)
```

If you prefer a single assembled `*core.SpeechResponse`, pass `stream` to `core.DrainSpeechStream(ctx, stream)`. Cancelling `ctx` promptly closes the HTTP response body, unblocks pending reads or channel sends, reports `ctx.Err()` on `stream.Err`, and closes all three channels (`Ch`, `Err`, `Final`).

## Incremental-Input WebSocket Text-to-Speech

For streaming LLM tokens directly into speech synthesis without waiting for the full text response, open a `core.RealtimeSynthesisSession` via `client.StartRealtimeSynthesis` (`wss://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream-input`):

```go
sess, err := client.StartRealtimeSynthesis(ctx, core.RealtimeSynthesisConfig{
	Model:               elevenlabs.ModelElevenFlashV25, // default
	VoiceID:             "JBFqnCBsd6RMkjVDRZzb",
	Format:              core.AudioFormatPCM16000,
	ChunkLengthSchedule: []int{120, 160, 250, 290},
	InactivityTimeout:   30 * time.Second,
})
if err != nil {
	return err
}
defer sess.Close()

// Stream incremental text fragments as tokens arrive.
_ = sess.SendText(ctx, "Hello from ")
_ = sess.SendText(ctx, "Iris. ")
// Flush forces audio generation while keeping the socket open for additional turns.
_ = sess.Flush(ctx)
// Finish signals end-of-input (EOS) so remaining buffered audio drains cleanly.
_ = sess.Finish(ctx)

for chunk := range sess.Audio() {
	if len(chunk.Data) > 0 {
		// Forward chunk.Data to playback immediately.
	}
}
if err := <-sess.Err(); err != nil {
	return err
}
```

Key constraints enforced preflight:
- **Model compatibility**: `eleven_flash_v2_5` and `eleven_multilingual_v2` advertise `core.FeatureSpeechSynthesisRealtime`. `eleven_v3` (and `eleven_v3*` / `eleven_v4*` / Scribe models) are rejected before dialing with `core.ErrNotSupported` because ElevenLabs' `/stream-input` TTS WebSocket does not support v3/v4 dialogue models.
- **Bounds**: `ChunkLengthSchedule` entries must be in `[50, 500]`; `InactivityTimeout` (when set) must be in `[1s, 180s]`.
- **Empty fragments**: `SendText(ctx, "")` is rejected with `core.ErrInvalidSpeechRequest` so callers cannot accidentally emit an EOS frame; use `Flush` or `Finish` explicitly.
- **Cleanup**: Always `defer sess.Close()` (idempotent and concurrency-safe) to release the WebSocket and background read goroutine immediately on error or barge-in interruption.

## File and Reader Scribe Transcription

```go
f, err := os.Open("meeting.mp3")
if err != nil {
	return err
}
defer f.Close()

resp, err := client.Transcribe(ctx, &core.TranscriptionRequest{
	Model:          elevenlabs.ModelScribeV2,
	Reader:         f,
	Filename:       "meeting.mp3",
	Diarize:        true,
	Timestamps:     core.TimestampGranularityWord,
	TagAudioEvents: true,
	Keyterms:       []string{"Iris", "PetalFlow"},
})
if err != nil {
	return err
}
// resp.IsFinal == true; resp.Text, resp.Language, resp.Words, resp.AudioEvents populated
```

## Voice Discovery and Settings (Read-Only)

Discover available voices and retrieve supported synthesis settings directly from `*elevenlabs.ElevenLabs`:

```go
// Single paginated page (GET /v2/voices)
page, err := provider.ListVoices(ctx, &elevenlabs.VoiceListRequest{
	PageSize: 25,
	Search:   "George",
	Category: "premade",
})

// Automatic cursor pagination across all matching voices
voices, err := provider.ListAllVoices(ctx, &elevenlabs.VoiceListRequest{Category: "premade"})

// Single voice metadata and settings
voice, err := provider.GetVoice(ctx, "JBFqnCBsd6RMkjVDRZzb")
settings, err := provider.GetVoiceSettings(ctx, "JBFqnCBsd6RMkjVDRZzb")
defaults, err := provider.GetDefaultVoiceSettings(ctx)
```

Voice discovery methods (`ListVoices`, `ListAllVoices`, `GetVoice`, `GetVoiceSettings`, `GetDefaultVoiceSettings`) are always read-only.

### Opt-In Voice Management and Consent-Gated Cloning

Mutating operations (`CreateVoice`, `EditVoice`, `EditVoiceSettings`, `DeleteVoice`) are **disabled by default** (failing fast with `elevenlabs.ErrVoiceManagementDisabled` / `core.ErrNotSupported`) and must be explicitly enabled with `elevenlabs.WithVoiceManagementEnabled()` (optionally paired with `elevenlabs.WithVoiceAuthorizer(...)` for RBAC/audit enforcement):

```go
provider, err := elevenlabs.NewFromEnv(
	elevenlabs.WithVoiceManagementEnabled(),
	elevenlabs.WithVoiceAuthorizer(myRBACAuthorizer),
)
created, err := provider.CreateVoice(ctx, &elevenlabs.CreateVoiceRequest{
	Name:    "Studio Host",
	Samples: []elevenlabs.VoiceSample{{Filename: "sample1.mp3", Data: sampleBytes}},
	Consent: &elevenlabs.VoiceConsent{
		Confirmed:   true,
		SubjectID:   "user-opaque-id",
		GrantedBy:   "self",
		GrantedAt:   time.Now(),
		EvidenceRef: "audit-consent-123",
	},
})
```

Security & consent safeguards:
- **Consent gate**: `CreateVoice` and `EditVoice` (when adding audio samples) require a confirmed, non-future `VoiceConsent` record (`ErrVoiceConsentRequired`).
- **Filename & size bounds**: Sample filenames reject path traversal (`..`, `/`, `\`, control chars); uploads stream through bounded `io.LimitReader` caps (default `10 MiB` per sample, `50 MiB` total, max `25` samples).
- **Privacy**: `VoiceConsent.String()` and `VoiceSample.String()` omit `SubjectID`, `EvidenceRef`, and raw audio bytes.

## Real-Time Voice Session and Event API

Full-duplex conversational voice sessions share a provider-agnostic contract in `core` (`core/voice_session.go`) designed for both Iris-managed STT→LLM→TTS pipelines and hosted session providers (such as ElevenAgents or Speech Engine):

- **Interfaces & Discovery**: `core.VoiceSession`, `core.VoiceSessionProvider`, `core.AsVoiceSessionProvider`, and `(*core.Client).StartVoiceSession`.
- **Typed Events (`core.VoiceEvent`)**:
  - Lifecycle: `session_started`, `session_state`, `session_ended`
  - Turn & interruption: `turn_started`, `turn_completed`, `turn_interrupted` (with `VoiceInterruption{CancelledTurnID, NextTurnID, Reason}`)
  - Transcripts: `user_transcript_partial` (`IsFinal: false`), `user_transcript_committed` (`IsFinal: true`), `agent_text_delta` (`IsFinal: false`), `agent_text_committed` (`IsFinal: true`)
  - Audio: `audio_output` (tagged with `TurnID`), `audio_output_ended`
  - Tools & errors: `tool_call`, `tool_result`, `error`
- **Stale Audio & Interruption Guard (`core.VoiceTurnTracker`)**: Generates monotonic turn IDs, cancels superseded/interrupted turn contexts immediately, and exposes `ObserveEvent` / `ShouldPlayAudio` so playback consumers drop late audio chunks belonging to interrupted turns.
- **Bounded Delivery (`core.VoiceEventEmitter`)**: Provides race-free, deadlock-free bounded event emission and idempotent `Close()` / `Fail(err)` channel closure.
- **Privacy**: `VoiceSessionConfig.String()` and `VoiceEvent.String()` omit system prompts, raw transcripts, assistant text, audio bytes, and tool argument/result payloads.

## Iris-Managed Voice Pipeline (`voice` package)

The optional `github.com/petal-labs/iris/voice` package composes real-time STT, any Iris LLM provider, and streaming TTS into a full-duplex `core.VoiceSession` while keeping LLM reasoning and tool execution completely under Iris control:

```go
pipeline, err := voice.NewPipeline(voice.PipelineConfig{
	STT:         elevenLabsProvider, // core.RealtimeSpeechRecognizer
	LLM:         openAIProvider,     // any core.Provider
	LLMModel:    "gpt-5",
	RealtimeTTS: elevenLabsProvider, // prefers WebSocket TTS; falls back to TTS (HTTP streaming)
	VoiceID:     "JBFqnCBsd6RMkjVDRZzb",
	Tools:       []core.Tool{weatherTool},
})
if err != nil {
	return err
}

sess, err := pipeline.StartVoiceSession(ctx, core.VoiceSessionConfig{
	InputFormat:  core.AudioFormatPCM16000,
	OutputFormat: core.AudioFormatPCM16000,
	SystemPrompt: "Be concise and conversational.",
})
if err != nil {
	return err
}
defer sess.Close()
```

Pipeline capabilities and guarantees:
- **Streaming token-to-audio**: When `LLM` supports `core.FeatureChatStreaming` and `RealtimeTTS` is configured, LLM text deltas stream directly into `RealtimeTTS.SendText` as tokens arrive; otherwise the pipeline falls back cleanly to `Chat` and/or `SpeechSynthesizer.StreamSynthesize`.
- **Tool handling**: Supports automatic execution via `ToolExecutor` (including `voice.RegistryToolExecutor(reg)` for `*tools.Registry`), direct `tools.Tool` execution, and caller-driven `SubmitToolResult` for schema-only `core.Tool` definitions, bounded by `MaxToolRounds` (default 5) and `ToolTimeout` (default 30s).
- **Interruption & barge-in**: Calling `sess.Interrupt(ctx)` or receiving non-empty partial user speech during an active assistant turn (unless `DisableBargeIn: true`) cancels in-flight LLM, tool, and TTS work immediately, emits `core.VoiceEventTurnInterrupted`, and suppresses stale audio chunks from the cancelled turn.
- **Bounded memory**: Conversation history is capped by `MaxHistoryMessages` (default 64) and event delivery uses a bounded `core.VoiceEventEmitter`.

## ElevenLabs Speech Engine Upstream Adapter

ElevenLabs **Speech Engine** is distinct from both client-initiated `voice.Pipeline` and hosted ElevenAgents. Its officially documented **Speech Engine Upstream** protocol reverses the WebSocket direction: **ElevenLabs connects outbound as a WebSocket client** to your server's publicly reachable `wsUrl`, handles audio capture/STT/turn-taking/TTS on the ElevenLabs side, and streams `user_transcript` frames to your Go server while expecting streamed `agent_response` frames back.

Mount `elevenlabs.SpeechEngineHandler` (`http.Handler`) on your Go HTTP server to connect Speech Engine to any Iris LLM provider and tools:

```go
handler, err := provider.NewSpeechEngineHandler(elevenlabs.SpeechEngineConfig{
	LLM:          openAIProvider, // any core.Provider
	Model:        "gpt-5",
	SystemPrompt: "Be concise and conversational.",
	Tools:        []core.Tool{weatherTool},
})
if err != nil {
	return err
}
http.Handle("/speech-engine", handler)
```

Protocol, security, and fallback notes:
- **Inbound JWT verification**: Every upgrade verifies `X-Elevenlabs-Speech-Engine-Authorization` (HS256 signed with SHA-256 of your ElevenLabs API key, `iss: https://api.elevenlabs.io/convai/speech-engine`, `sub: convai_speech_engine_upstream`, 60s clock-skew leeway) via `elevenlabs.VerifySpeechEngineJWT`. Set `DisableAuth: true` only behind an IP allowlist restricting traffic to ElevenLabs egress ranges.
- **Interruption cancellation**: Receiving a `user_transcript` with a higher `event_id` cancels the in-flight LLM turn context immediately and discards stale responses.
- **Fallback / Limitations**: Speech Engine requires a publicly reachable server endpoint (`wsUrl`), does not expose raw audio bytes to your server, and has no official ElevenLabs Go SDK (protocol schemas follow ElevenLabs' upstream WebSocket documentation and may evolve). Use the Iris-managed `voice.Pipeline` when you want client-initiated raw-audio streaming with zero inbound webhook/WebSocket infrastructure.

## Hosted ElevenAgents Sessions

For hosted ElevenLabs Conversational AI agents (`wss://api.elevenlabs.io/v1/convai/conversation`), `*elevenlabs.ElevenLabs` implements `core.VoiceSessionProvider` (`core.FeatureVoiceSession`):

```go
// Private agent: automatically mints a short-lived signed URL via GET /v1/convai/conversation/get-signed-url
sess, err := client.StartVoiceSession(ctx, core.VoiceSessionConfig{
	AgentID: "agent_...",
})
if err != nil {
	return err
}
defer sess.Close()

// Or mint a signed URL on your backend for a client without an API key:
signed, err := provider.GetAgentSignedURL(ctx, &elevenlabs.AgentSignedURLRequest{
	AgentID: "agent_...",
})
```

Architectural distinction & limitations:
- **Hosted reasoning**: ElevenAgents runs the LLM, prompt, and turn-taking configured in your ElevenLabs agent dashboard. Use `voice.Pipeline` or `SpeechEngineHandler` when you want Iris to control external LLM reasoning directly.
- **Signed URLs & privacy**: `AgentSignedURLResponse.SignedURL` is stored as `core.Secret`, redacted in `String()`/`GoString()`/JSON, and scrubbed from dial/network/server errors.
- **Client tools & heartbeats**: `ping` frames are automatically answered with `pong`; `client_tool_call` events emit `core.VoiceEventToolCall` and accept responses via `sess.SubmitToolResult(ctx, result)`.

## Unsupported Models, Formats, and Current Limitations

- **Chat completions**: `ElevenLabs.Chat` and `ElevenLabs.StreamChat` return `*core.ProviderError` wrapping `core.ErrNotSupported`.
- **Unsupported TTS formats**: `wav`, `flac`, `ogg`, `aac`, `webm`, and multi-channel (`Channels > 1`) synthesis requests fail fast with `core.ErrInvalidAudioFormat` before sending HTTP traffic.
- **Scribe preflight limits**:
  - `Keyterms` is unsupported on `scribe_v1` (use `scribe_v2`) and capped at `1,000` terms (`MaxScribeKeyterms`).
  - `Timestamps: core.TimestampGranularitySegment` is unsupported by ElevenLabs Scribe (use `none`, `word`, or `character`).
  - `NumSpeakers` is capped at `32` (`MaxScribeSpeakers`); `Temperature` is capped at `2.0` (`MaxScribeTemperature`).
  - Raw PCM transcription input must be `16000Hz` mono signed 16-bit little-endian (`pcm_s16le_16`); container formats (`mp3`, `wav`, `flac`, `ogg`, `opus`, `aac`, `webm`) are uploaded directly.
  - Default upload cap is `100 MiB` (`DefaultMaxTranscriptionBytes`), overridable per request via `TranscriptionRequest.MaxBytes`.
- **Retries with `io.Reader`**: `core.Client.Transcribe` automatically retries transient 429/5xx/network errors only for in-memory `AudioData` inputs. Requests backed by `Reader` are attempted once because `io.Reader` cannot be rewound safely.
- **Voice cloning scope**: Instant Voice Cloning (`POST /v1/voices/add`) is opt-in and consent-gated; Professional Voice Cloning (PVC) verification flows remain managed through the ElevenLabs dashboard.

For downstream workflow orchestration contracts (typed Transcription/Synthesis/Session nodes, tool ports, credential hydration, Cortex boundaries, and acceptance gates), see the [PetalFlow Voice Integration Specification](petalflow-voice-integration.md).

## Privacy and Reliability Defaults

- **No secrets, audio, or transcripts in default logs**: `core.Secret` redacts API keys in `String`, `GoString`, and JSON; `ElevenLabs`, `Config`, `AudioData`, `AudioChunk`, `SpeechRequest`, `SpeechResponse`, `TranscriptionRequest`, and `TranscriptionResponse` emit metadata summaries only in `String()`/`GoString()`.
- **Error scrubbing**: `ProviderError.Message` and retained error `Body` scrub the configured API key if an upstream server or proxy reflects it.
- **Telemetry**: `core.TelemetryHook` and `core.VoiceTelemetryHook` (via `core.WithVoiceTelemetry`, `voice.PipelineConfig.Telemetry`, `core.ObserveVoiceSession`, and OpenTelemetry `contrib/otel.NewVoiceHook`) emit provider, model, operation, timing, first-chunk latency, byte/character/turn/interruption/tool counts, and error status only—never raw audio, prompts, transcripts, or credentials.
- **Bounded retries**: `core.Client` honors server `Retry-After` headers on HTTP 429, retries 5xx/network errors up to `MaxRetries`, and never retries 400/401/403/404/422 errors.

## Runnable Examples and Live Tests

```bash
export ELEVENLABS_API_KEY=your-key

# Synchronous TTS
go run ./examples/speech/elevenlabs-synthesize -text "Hello from Iris." -out speech.mp3

# HTTP streaming TTS
go run ./examples/speech/elevenlabs-stream -text "Streaming audio from Iris." -out stream.mp3

# File-based Scribe transcription
go run ./examples/speech/elevenlabs-transcribe -file speech.mp3 -diarize -timestamps word

# Optional live integration tests
go test -count=1 -tags=integration -v ./tests/integration -run TestElevenLabs
```
