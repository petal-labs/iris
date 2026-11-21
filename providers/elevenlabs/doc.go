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
// # Incremental-Input Text-to-Speech (WebSocket)
//
// [ElevenLabs] implements [core.RealtimeSpeechSynthesizer] via
// [ElevenLabs.StartRealtimeSynthesis] (and
// [core.Client.StartRealtimeSynthesis]) targeting
// wss://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream-input:
//
//	sess, err := client.StartRealtimeSynthesis(ctx, core.RealtimeSynthesisConfig{
//	    Model:   elevenlabs.ModelElevenFlashV25,
//	    VoiceID: "JBFqnCBsd6RMkjVDRZzb",
//	    Format:  core.AudioFormatPCM16000,
//	})
//	defer sess.Close()
//
//	_ = sess.SendText(ctx, "Hello from ")
//	_ = sess.SendText(ctx, "Iris.")
//	_ = sess.Flush(ctx)  // force audio generation without closing the socket
//	_ = sess.Finish(ctx) // signal end-of-input so remaining audio drains cleanly
//
// Model compatibility is enforced before dialing: [ModelElevenFlashV25] and
// [ModelElevenMultilingualV2] advertise [core.FeatureSpeechSynthesisRealtime],
// whereas [ModelElevenV3] (which requires ElevenLabs' separate Text-to-Dialogue
// API) and Scribe STT models are rejected with [core.ErrNotSupported].
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
// # Speech Engine Upstream Server Adapter
//
// ElevenLabs Speech Engine uses an inverted WebSocket protocol where
// ElevenLabs connects outbound to your server's publicly reachable wsUrl,
// delivers transcribed conversation history, and synthesizes streamed text
// replies from your chosen Iris LLM provider. Mount [SpeechEngineHandler]
// created via [ElevenLabs.NewSpeechEngineHandler] on your HTTP server:
//
//	handler, err := provider.NewSpeechEngineHandler(elevenlabs.SpeechEngineConfig{
//	    LLM:          openAIProvider,
//	    Model:        "gpt-5",
//	    SystemPrompt: "Be concise and conversational.",
//	    Tools:        []core.Tool{weatherTool},
//	})
//	http.Handle("/speech-engine", handler)
//
// Inbound upgrades are authenticated via [VerifySpeechEngineJWT] against the
// X-Elevenlabs-Speech-Engine-Authorization HS256 JWT header (signed with the
// SHA-256 hash of your ElevenLabs API key, iss/sub claims, and 60s clock-skew
// leeway). Higher event_id transcripts immediately cancel in-flight LLM turns.
// Speech Engine is distinct from hosted ElevenAgents and from client-initiated
// raw-audio streaming; use package voice ([github.com/petal-labs/iris/voice])
// when your application needs client-initiated audio streaming.
//
// # Hosted ElevenAgents Sessions (Conversational AI)
//
// [ElevenLabs] implements [core.VoiceSessionProvider] via
// [ElevenLabs.StartVoiceSession] (and [core.Client.StartVoiceSession]) plus the
// provider-specific [ElevenLabs.StartAgentSession] and
// [ElevenLabs.GetAgentSignedURL] helpers targeting
// wss://api.elevenlabs.io/v1/convai/conversation and
// GET /v1/convai/conversation/get-signed-url:
//
//	sess, err := client.StartVoiceSession(ctx, core.VoiceSessionConfig{
//	    AgentID: "agent_...",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sess.Close()
//
// For private agents, StartVoiceSession automatically mints a short-lived
// signed URL (stored as [core.Secret] and scrubbed from logs and errors) before
// dialing the WebSocket. Untrusted/client callers can also pass a pre-minted
// SignedURL into [ElevenLabs.StartAgentSession] without holding an API key.
// Hosted ElevenAgents executes its LLM and prompt on ElevenLabs' platform; use
// package voice or [SpeechEngineHandler] when Iris should control LLM reasoning.
//
// # Current Limitations
//
// Synchronous TTS, HTTP streaming TTS, incremental-input WebSocket TTS,
// file-based Scribe transcription, real-time Scribe WebSocket STT, Speech
// Engine upstream server handling, hosted ElevenAgents sessions, and read-only
// voice discovery/settings are implemented. Consent-gated voice management and
// cloning are implemented in subsequent issues.
package elevenlabs
