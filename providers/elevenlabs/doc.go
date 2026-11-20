// Package elevenlabs provides an ElevenLabs speech API provider for Iris.
//
// ElevenLabs is a speech-focused provider (text-to-speech synthesis and
// speech-to-text transcription). It implements [core.Provider] so it can be
// registered with [github.com/petal-labs/iris/providers] and wrapped by
// [core.Client], while chat methods ([ElevenLabs.Chat] and
// [ElevenLabs.StreamChat]) return [core.ErrNotSupported].
//
// # Construction and Authentication
//
// Create a provider from an explicit API key or from the ELEVENLABS_API_KEY
// environment variable:
//
//	p, err := elevenlabs.NewFromEnv(
//	    elevenlabs.WithTimeout(30 * time.Second),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(p)
//
// The API key is stored as a [core.Secret] and transmitted in the xi-api-key
// HTTP header. Formatting [Config] or [ElevenLabs] with fmt (String/GoString)
// or marshaling [Config] to JSON redacts the API key and omits custom header
// values. Normalized [core.ProviderError] messages and retained error bodies
// also scrub the configured API key if reflected by an upstream server or proxy.
//
// # Concurrency and Timeouts
//
// [ElevenLabs] is immutable after construction and safe for concurrent use
// across goroutines. Per-provider timeouts configured via [WithTimeout] apply
// to unary operations unless the caller's context already carries a deadline
// or [WithTimeout](0) disables the default timeout.
//
// # Text-to-Speech (Synchronous and HTTP Streaming)
//
// [ElevenLabs] implements [core.SpeechSynthesizer] for both synchronous
// synthesis ([ElevenLabs.Synthesize]) and incremental HTTP streaming
// ([ElevenLabs.StreamSynthesize]):
//
//	resp, err := client.Synthesize(ctx, &core.SpeechRequest{
//	    Model:   elevenlabs.ModelElevenFlashV25,
//	    VoiceID: "JBFqnCBsd6RMkjVDRZzb",
//	    Text:    "Hello from Iris.",
//	    Format:  core.AudioFormatMP3_44100_128,
//	})
//
//	stream, err := client.StreamSynthesize(ctx, &core.SpeechRequest{
//	    Model:   elevenlabs.ModelElevenFlashV25,
//	    VoiceID: "JBFqnCBsd6RMkjVDRZzb",
//	    Text:    "Streaming audio from Iris.",
//	    Format:  core.AudioFormatPCM16000,
//	})
//
// Supported output formats include MP3 (22050Hz/32kbps; 44100Hz at 32/64/96/
// 128/192kbps), raw PCM (8000/16000/22050/24000/44100/48000Hz), G.711 mu-law
// and A-law (8000Hz), and Opus (48000Hz at 32/64/96/128/192kbps), mono only.
// Streaming reads use bounded chunk sizes ([WithStreamChunkSize], default
// 16 KiB) and bounded channel capacity ([WithStreamBufferSize], default 16
// chunks) so memory remains bounded regardless of audio length. Cancelling ctx
// promptly closes the HTTP body and closes Ch, Err, and Final without leaking
// goroutines.
//
// # Speech-to-Text (File and Reader Scribe Transcription)
//
// [ElevenLabs] implements [core.SpeechRecognizer] via [ElevenLabs.Transcribe]
// (and [core.Client.Transcribe]) targeting POST /v1/speech-to-text:
//
//	resp, err := client.Transcribe(ctx, &core.TranscriptionRequest{
//	    Model:          elevenlabs.ModelScribeV2,
//	    Reader:         audioFile,
//	    Filename:       "meeting.mp3",
//	    Diarize:        true,
//	    Timestamps:     core.TimestampGranularityWord,
//	    TagAudioEvents: true,
//	})
//
// Both in-memory [core.AudioData] and streaming io.Reader inputs are supported
// via multipart/form-data upload with bounded size enforcement (req.MaxBytes or
// [DefaultMaxTranscriptionBytes] = 100 MiB). Returned responses set
// IsFinal = true. Scribe limits validated before HTTP include keyterms <= 1000
// (scribe_v2 only), num_speakers <= 32, temperature <= 2.0, timestamp
// granularity (none/word/character; segment is not supported), and raw PCM
// (16000Hz mono pcm_s16le_16 only).
//
// # Real-Time Speech-to-Text (Scribe WebSocket)
//
// [ElevenLabs] implements [core.RealtimeSpeechRecognizer] via
// [ElevenLabs.StartRealtimeTranscription] (and
// [core.Client.StartRealtimeTranscription]) targeting
// wss://api.elevenlabs.io/v1/speech-to-text/realtime with
// [ModelScribeV2Realtime]:
//
//	sess, err := client.StartRealtimeTranscription(ctx, core.RealtimeTranscriptionConfig{
//	    Model:          elevenlabs.ModelScribeV2Realtime,
//	    Format:         core.AudioFormatPCM16000,
//	    CommitStrategy: core.CommitStrategyManual,
//	})
//
// Supported real-time formats are mono PCM (8000/16000/22050/24000/44100/
// 48000Hz) and G.711 mu-law (8000Hz). Chunks are validated before transmission
// (max chunk size, even byte length for 16-bit PCM, format matching) and
// partial vs. committed events are delivered over a bounded Events channel.
//
// # Voice Discovery and Settings (Read-Only)
//
// Read-only voice listing, cursor pagination, metadata retrieval, and voice
// settings inspection are available directly on [ElevenLabs]:
//
//   - [ElevenLabs.ListVoices] / [ElevenLabs.ListAllVoices] (GET /v2/voices)
//   - [ElevenLabs.GetVoice] (GET /v1/voices/{voice_id})
//   - [ElevenLabs.GetVoiceSettings] (GET /v1/voices/{voice_id}/settings)
//   - [ElevenLabs.GetDefaultVoiceSettings] (GET /v1/voices/settings/default)
//
// Returned [core.VoiceSettings] values can be passed directly into
// [core.SpeechRequest.VoiceSettings]. Voice creation, editing, deletion, and
// voice cloning are intentionally not exposed by this read-only API.
//
// # Current Limitations
//
// Synchronous TTS, HTTP streaming TTS, file-based Scribe transcription,
// real-time Scribe WebSocket STT, and read-only voice discovery/settings are
// implemented. WebSocket incremental-input TTS, full-duplex voice sessions,
// and consent-gated voice management are implemented in subsequent issues.
package elevenlabs
