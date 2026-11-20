package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrRealtimeSessionClosed indicates an operation was attempted on a closed
// [RealtimeTranscriptionSession] or [RealtimeSynthesisSession].
var ErrRealtimeSessionClosed = errors.New("realtime transcription session closed")

// CommitStrategy controls how a real-time speech recognizer finalizes utterances.
type CommitStrategy string

const (
	// CommitStrategyDefault uses the provider's default commit strategy.
	CommitStrategyDefault CommitStrategy = ""
	// CommitStrategyManual commits segments only when the caller invokes Commit or sets AudioChunk.IsFinal.
	CommitStrategyManual CommitStrategy = "manual"
	// CommitStrategyVAD commits segments automatically using server-side voice activity detection.
	CommitStrategyVAD CommitStrategy = "vad"
)

// IsValid reports whether the commit strategy is a recognized value.
func (s CommitStrategy) IsValid() bool {
	switch s {
	case CommitStrategyDefault, CommitStrategyManual, CommitStrategyVAD:
		return true
	default:
		return false
	}
}

// RealtimeTranscriptionConfig configures a bidirectional real-time speech-to-text stream.
type RealtimeTranscriptionConfig struct {
	// Model specifies the real-time STT model ID. Empty uses the provider default.
	Model ModelID `json:"model,omitempty"`

	// Format describes the incoming audio stream format (e.g., core.AudioFormatPCM16000).
	// Required so the recognizer knows how to decode raw audio chunks.
	Format AudioFormat `json:"format"`

	// Language is an optional ISO-639 / BCP-47 language code hint.
	Language string `json:"language,omitempty"`

	// CommitStrategy selects manual or VAD-based utterance finalization.
	CommitStrategy CommitStrategy `json:"commit_strategy,omitempty"`

	// IncludeTimestamps requests word-level timestamps on committed transcripts when supported.
	IncludeTimestamps bool `json:"include_timestamps,omitempty"`

	// MaxChunkBytes optionally bounds the maximum bytes permitted per SendAudio chunk.
	// Zero uses the provider's default safe chunk limit.
	MaxChunkBytes int `json:"max_chunk_bytes,omitempty"`

	// EventBufferSize optionally sets the bounded capacity of the session's Events channel.
	// Zero uses the provider's default bounded buffer size.
	EventBufferSize int `json:"event_buffer_size,omitempty"`

	// Metadata holds optional provider-agnostic key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks that RealtimeTranscriptionConfig carries a valid audio format,
// commit strategy, and non-negative buffer limits. Returned errors wrap
// [ErrInvalidTranscriptionRequest].
func (c RealtimeTranscriptionConfig) Validate() error {
	if err := c.Format.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidTranscriptionRequest, err)
	}
	if !c.CommitStrategy.IsValid() {
		return fmt.Errorf("%w: unsupported commit_strategy %q", ErrInvalidTranscriptionRequest, c.CommitStrategy)
	}
	if c.MaxChunkBytes < 0 {
		return fmt.Errorf("%w: max_chunk_bytes must be non-negative, got %d", ErrInvalidTranscriptionRequest, c.MaxChunkBytes)
	}
	if c.EventBufferSize < 0 {
		return fmt.Errorf("%w: event_buffer_size must be non-negative, got %d", ErrInvalidTranscriptionRequest, c.EventBufferSize)
	}
	return nil
}

// RealtimeTranscriptEventType identifies the kind of event emitted during a
// real-time speech-to-text session.
type RealtimeTranscriptEventType string

const (
	// RealtimeTranscriptEventSessionStarted confirms the remote STT session is active.
	RealtimeTranscriptEventSessionStarted RealtimeTranscriptEventType = "session_started"

	// RealtimeTranscriptEventPartial carries an interim, mutable transcript hypothesis.
	RealtimeTranscriptEventPartial RealtimeTranscriptEventType = "partial"

	// RealtimeTranscriptEventCommitted carries a finalized, stable utterance transcript.
	RealtimeTranscriptEventCommitted RealtimeTranscriptEventType = "committed"
)

// RealtimeTranscriptEvent represents a single event emitted by a
// [RealtimeTranscriptionSession].
//
// Partial events carry provisional hypotheses (IsFinal == false) that may be
// revised as more audio arrives. Committed events carry finalized utterances
// (IsFinal == true) suitable for downstream persistence or LLM turns.
//
// Privacy: String and GoString omit raw transcript text.
type RealtimeTranscriptEvent struct {
	// Type distinguishes session lifecycle, partial, and committed transcript events.
	Type RealtimeTranscriptEventType `json:"type"`

	// SessionID identifies the remote real-time session when provided.
	SessionID string `json:"session_id,omitempty"`

	// Transcript carries the partial or committed transcript payload when applicable.
	Transcript *TranscriptionResponse `json:"transcript,omitempty"`

	// IsFinal is true for committed utterances and false for interim partial hypotheses.
	IsFinal bool `json:"is_final,omitempty"`

	// Timestamp records when the SDK received the event.
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// String returns a log-safe metadata summary without raw transcript text.
func (e RealtimeTranscriptEvent) String() string {
	tr := "<nil>"
	if e.Transcript != nil {
		tr = e.Transcript.String()
	}
	return fmt.Sprintf("RealtimeTranscriptEvent{type=%q, session_id=%q, final=%t, transcript=%s}",
		e.Type, e.SessionID, e.IsFinal, tr)
}

// GoString returns a log-safe representation for %#v formatting.
func (e RealtimeTranscriptEvent) GoString() string {
	return "core." + e.String()
}

// RealtimeTranscriptionSession represents an active bidirectional streaming
// speech-to-text session.
//
// Implementations MUST:
//   - Validate incoming chunks before transmission (rejecting oversized or malformed chunks)
//   - Maintain bounded event buffers so a slow consumer cannot cause unbounded memory growth
//   - Close Events() and Err() promptly when the session ends or the parent context cancels
//   - Make Close() idempotent and safe for concurrent calls
type RealtimeTranscriptionSession interface {
	// SendAudio validates and transmits a single incremental audio chunk.
	SendAudio(ctx context.Context, chunk AudioChunk) error

	// Commit requests finalization of the current uncommitted audio segment.
	Commit(ctx context.Context) error

	// Events returns a bounded receive-only channel of transcript events.
	// Closed when the session terminates.
	Events() <-chan RealtimeTranscriptEvent

	// Err returns a receive-only channel emitting at most one terminal error before closing.
	Err() <-chan error

	// Close terminates the session and releases all underlying connections and goroutines.
	Close() error
}

// RealtimeSpeechRecognizer is an optional interface for providers that support
// low-latency bidirectional streaming speech-to-text.
type RealtimeSpeechRecognizer interface {
	// StartRealtimeTranscription opens a new real-time STT session.
	StartRealtimeTranscription(ctx context.Context, cfg RealtimeTranscriptionConfig) (RealtimeTranscriptionSession, error)
}

// AsRealtimeSpeechRecognizer discovers real-time speech recognition on a
// Provider or its [ProviderUnwrapper] chain. It returns nil and false when no
// provider implements [RealtimeSpeechRecognizer].
func AsRealtimeSpeechRecognizer(p Provider) (RealtimeSpeechRecognizer, bool) {
	return asProviderCapability[RealtimeSpeechRecognizer](p)
}

// StartRealtimeTranscription opens a bidirectional real-time speech-to-text
// session through the client's provider. It validates cfg, verifies
// [RealtimeSpeechRecognizer] support, and applies client rate limiting.
func (c *Client) StartRealtimeTranscription(ctx context.Context, cfg RealtimeTranscriptionConfig) (RealtimeTranscriptionSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: realtime transcription context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	recognizer, ok := AsRealtimeSpeechRecognizer(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support realtime speech recognition: %w", c.speechProviderID(), ErrNotSupported)
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	return recognizer.StartRealtimeTranscription(ctx, cfg)
}

// RealtimeSynthesisConfig configures a bidirectional incremental-input
// text-to-speech session where text fragments (such as streaming LLM tokens)
// are sent incrementally and synthesized audio chunks are streamed back.
//
// Privacy: String and GoString omit metadata values that could carry sensitive
// caller data and never include credentials or synthesis text.
type RealtimeSynthesisConfig struct {
	// Model specifies the real-time TTS model ID. Empty uses the provider default.
	Model ModelID `json:"model,omitempty"`

	// VoiceID identifies the voice to synthesize with. Required.
	VoiceID string `json:"voice_id"`

	// Format specifies the desired output audio format. Zero uses the provider default.
	Format AudioFormat `json:"format,omitempty"`

	// Language is an optional ISO-639 / BCP-47 language code hint (e.g., "en").
	Language string `json:"language,omitempty"`

	// VoiceSettings specifies optional synthesis tuning knobs.
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`

	// ChunkLengthSchedule optionally specifies progressive character-count
	// thresholds used by the synthesizer to trigger incremental audio generation
	// (e.g., [120, 160, 250, 290]). All elements must be positive when provided.
	ChunkLengthSchedule []int `json:"chunk_length_schedule,omitempty"`

	// AutoMode enables provider-managed low-latency chunk scheduling when supported.
	AutoMode bool `json:"auto_mode,omitempty"`

	// InactivityTimeout optionally bounds how long the remote session stays open
	// without client messages before the server closes it. Must be non-negative.
	InactivityTimeout time.Duration `json:"inactivity_timeout_ns,omitempty"`

	// SyncAlignment requests character timing alignment metadata when supported.
	SyncAlignment bool `json:"sync_alignment,omitempty"`

	// BufferSize optionally sets the bounded capacity of the session's Audio channel.
	// Zero uses the provider's default bounded buffer size.
	BufferSize int `json:"buffer_size,omitempty"`

	// Metadata holds optional provider-agnostic key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks that RealtimeSynthesisConfig contains required fields and valid
// parameter bounds. Returned errors wrap [ErrInvalidSpeechRequest].
func (c RealtimeSynthesisConfig) Validate() error {
	if strings.TrimSpace(c.VoiceID) == "" {
		return fmt.Errorf("%w: voice_id is required", ErrInvalidSpeechRequest)
	}
	if !c.Format.IsZero() {
		if err := c.Format.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSpeechRequest, err)
		}
	}
	if err := c.VoiceSettings.Validate(); err != nil {
		return err
	}
	for i, threshold := range c.ChunkLengthSchedule {
		if threshold <= 0 {
			return fmt.Errorf("%w: chunk_length_schedule[%d] must be positive, got %d",
				ErrInvalidSpeechRequest, i, threshold)
		}
	}
	if c.InactivityTimeout < 0 {
		return fmt.Errorf("%w: inactivity_timeout must be non-negative, got %s",
			ErrInvalidSpeechRequest, c.InactivityTimeout)
	}
	if c.BufferSize < 0 {
		return fmt.Errorf("%w: buffer_size must be non-negative, got %d",
			ErrInvalidSpeechRequest, c.BufferSize)
	}
	return nil
}

// String returns a log-safe summary of the synthesis configuration.
func (c RealtimeSynthesisConfig) String() string {
	return fmt.Sprintf("RealtimeSynthesisConfig{model=%q, voice_id=%q, format=%s, language=%q, auto_mode=%t}",
		c.Model, c.VoiceID, c.Format.String(), c.Language, c.AutoMode)
}

// GoString returns a log-safe representation for %#v formatting.
func (c RealtimeSynthesisConfig) GoString() string {
	return "core." + c.String()
}

// RealtimeSynthesisSession represents an active bidirectional incremental-input
// text-to-speech session.
//
// Typical lifecycle:
//   - Call SendText one or more times as text fragments arrive.
//   - Call Flush to force generation of buffered text while keeping the session
//     open for additional turns.
//   - Call Finish to signal end-of-input so remaining buffered audio drains cleanly
//     and Audio() closes naturally.
//   - Call Close (always defer Close) to immediately abort or clean up all
//     underlying connections and goroutines.
//
// Implementations MUST:
//   - Maintain bounded Audio() channel capacity so slow consumers cannot cause
//     unbounded memory growth
//   - Close Audio() and Err() promptly when the session ends or the parent context cancels
//   - Make Close() idempotent and safe for concurrent calls
type RealtimeSynthesisSession interface {
	// SendText transmits an incremental non-empty text fragment for synthesis.
	SendText(ctx context.Context, text string) error

	// Flush forces synthesis of all currently buffered text without closing the session.
	Flush(ctx context.Context) error

	// Finish signals end-of-input so the synthesizer flushes remaining audio and
	// closes the stream cleanly. Subsequent SendText, Flush, or Finish calls return
	// [ErrRealtimeSessionClosed].
	Finish(ctx context.Context) error

	// Audio returns a bounded receive-only channel of synthesized audio chunks.
	// Closed when the session terminates.
	Audio() <-chan AudioChunk

	// Err returns a receive-only channel emitting at most one terminal error before closing.
	Err() <-chan error

	// Close terminates the session immediately and releases all underlying
	// connections and goroutines.
	Close() error
}

// RealtimeSpeechSynthesizer is an optional interface for providers that support
// bidirectional incremental-input text-to-speech synthesis (e.g., over WebSocket).
type RealtimeSpeechSynthesizer interface {
	// StartRealtimeSynthesis opens a new incremental-input TTS session.
	StartRealtimeSynthesis(ctx context.Context, cfg RealtimeSynthesisConfig) (RealtimeSynthesisSession, error)
}

// AsRealtimeSpeechSynthesizer discovers incremental-input real-time speech
// synthesis on a Provider or its [ProviderUnwrapper] chain. It returns nil and
// false when no provider implements [RealtimeSpeechSynthesizer].
func AsRealtimeSpeechSynthesizer(p Provider) (RealtimeSpeechSynthesizer, bool) {
	return asProviderCapability[RealtimeSpeechSynthesizer](p)
}

// StartRealtimeSynthesis opens a bidirectional incremental-input text-to-speech
// session through the client's provider. It validates cfg, verifies
// [RealtimeSpeechSynthesizer] support, and applies client rate limiting.
func (c *Client) StartRealtimeSynthesis(ctx context.Context, cfg RealtimeSynthesisConfig) (RealtimeSynthesisSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: realtime synthesis context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	synth, ok := AsRealtimeSpeechSynthesizer(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support realtime speech synthesis: %w", c.speechProviderID(), ErrNotSupported)
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	return synth.StartRealtimeSynthesis(ctx, cfg)
}
