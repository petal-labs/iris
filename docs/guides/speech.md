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
- `core.FeatureSpeechRecognition` — file/reader speech-to-text (`Transcribe`)
- `core.FeatureSpeechRecognitionRealtime` — real-time streaming STT (`StartRealtimeTranscription`)
- `core.FeatureVoiceSession` — full-duplex voice sessions *(planned)*

## Verified Models and Capabilities

| Model Constant | Model ID | Sync TTS | Streaming TTS | File/Reader STT | Realtime STT | Notes |
|---|---|---|---|---|---|---|
| `elevenlabs.ModelElevenMultilingualV2` | `eleven_multilingual_v2` | Yes | Yes | No | No | Default TTS model; 29 languages |
| `elevenlabs.ModelElevenFlashV25` | `eleven_flash_v2_5` | Yes | Yes | No | No | Low-latency multilingual TTS |
| `elevenlabs.ModelElevenV3` | `eleven_v3` | Yes | Yes | No | No | Expressive multi-speaker TTS |
| `elevenlabs.ModelScribeV1` | `scribe_v1` | No | No | Yes | No | Batch STT (`keyterms` unsupported) |
| `elevenlabs.ModelScribeV2` | `scribe_v2` | No | No | Yes | No | Default batch STT model; diarization, timestamps, `keyterms`, audio events |
| `elevenlabs.ModelScribeV2Realtime` | `scribe_v2_realtime` | No | No | No | Yes | Default real-time WebSocket STT (partial + committed transcripts) |

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

Voice discovery is strictly read-only: voice creation, editing, deletion, and voice cloning are not exposed.

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
- **Planned (not yet implemented)**: Incremental text-input WebSocket TTS, Iris-managed voice pipelines, hosted ElevenAgents sessions, and consent-gated voice management/cloning.

## Privacy and Reliability Defaults

- **No secrets, audio, or transcripts in default logs**: `core.Secret` redacts API keys in `String`, `GoString`, and JSON; `ElevenLabs`, `Config`, `AudioData`, `AudioChunk`, `SpeechRequest`, `SpeechResponse`, `TranscriptionRequest`, and `TranscriptionResponse` emit metadata summaries only in `String()`/`GoString()`.
- **Error scrubbing**: `ProviderError.Message` and retained error `Body` scrub the configured API key if an upstream server or proxy reflects it.
- **Telemetry**: `core.TelemetryHook` events carry provider, model, timing, and error metadata only—never raw audio, prompts, or transcripts.
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
