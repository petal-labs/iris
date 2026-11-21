// Package voice provides a provider-independent, Iris-managed full-duplex
// real-time voice pipeline.
//
// A [Pipeline] composes three interchangeable Iris components into a single
// [core.VoiceSession]:
//
//  1. Real-time streaming speech-to-text ([core.RealtimeSpeechRecognizer], such
//     as ElevenLabs Scribe v2 Realtime)
//  2. Any Iris chat provider ([core.Provider] — OpenAI, Anthropic, Gemini,
//     Ollama, xAI, Perplexity, Azure AI Foundry, etc.) with optional tool calling
//  3. Streaming text-to-speech — preferring incremental-input WebSocket TTS
//     ([core.RealtimeSpeechSynthesizer]) so LLM text deltas begin synthesizing
//     as tokens arrive, with automatic fallback to HTTP streaming TTS
//     ([core.SpeechSynthesizer.StreamSynthesize])
//
// # Architecture and Ownership
//
// Following Iris's architectural boundaries:
//   - Audio capture and speaker playback remain application-side responsibilities;
//     [Session] accepts encoded/PCM audio chunks via SendAudio and emits
//     synthesized audio chunks via Events().
//   - External LLM reasoning stays completely under Iris/application control,
//     distinct from hosted conversational agent platforms (such as ElevenAgents).
//   - Partial user transcripts emit [core.VoiceEventUserTranscriptPartial] and
//     may trigger barge-in interruption, whereas only committed user utterances
//     ([core.VoiceEventUserTranscriptCommitted]) or explicit SendText calls enter
//     conversation history and trigger LLM reasoning turns.
//   - Every conversational turn receives a unique TurnID via [core.VoiceTurnTracker];
//     calling Interrupt (or user barge-in) cancels in-flight LLM, tool, and TTS
//     work immediately and suppresses stale audio chunks from cancelled turns.
//
// # Usage Example
//
//	pipeline, err := voice.NewPipeline(voice.PipelineConfig{
//	    STT:         elevenLabsProvider,
//	    LLM:         openAIProvider,
//	    LLMModel:    "gpt-5",
//	    RealtimeTTS: elevenLabsProvider,
//	    VoiceID:     "JBFqnCBsd6RMkjVDRZzb",
//	    Tools:       []core.Tool{weatherTool},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	sess, err := pipeline.StartVoiceSession(ctx, core.VoiceSessionConfig{
//	    InputFormat:  core.AudioFormatPCM16000,
//	    OutputFormat: core.AudioFormatPCM16000,
//	    SystemPrompt: "Be concise and conversational.",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sess.Close()
//
// # Error Semantics and Limitations
//
//   - Session-level transport failures (such as a dropped STT WebSocket or
//     parent context cancellation) terminate the session and report the error
//     on Err().
//   - Turn-level LLM, tool, or TTS failures emit [core.VoiceEventError] tagged
//     with the turn's TurnID while keeping the session open for subsequent turns.
//   - When RealtimeTTS is unavailable and only [core.SpeechSynthesizer] is
//     configured, the pipeline buffers the LLM round's text before calling
//     StreamSynthesize, which increases first-audio latency relative to
//     incremental WebSocket TTS.
//   - Conversation history is bounded in memory by MaxHistoryMessages (default
//     64) and is not persisted automatically; callers needing durable storage
//     should persist committed transcript events explicitly.
package voice
