package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Voice session sentinel errors.
var (
	// ErrInvalidVoiceSessionConfig indicates a VoiceSessionConfig failed validation.
	ErrInvalidVoiceSessionConfig = errors.New("invalid voice session config")

	// ErrVoiceSessionClosed indicates an operation was attempted on a closed VoiceSession.
	ErrVoiceSessionClosed = errors.New("voice session closed")
)

// DefaultVoiceEventBufferSize is the default bounded capacity of a [VoiceEventEmitter]
// events channel when no positive buffer size is configured.
const DefaultVoiceEventBufferSize = 64

// maxCancelledTurnHistory bounds the cancelled-turn history retained by
// [VoiceTurnTracker] so long-running voice sessions use O(1) memory.
const maxCancelledTurnHistory = 128

// VoiceSessionState represents the lifecycle state of a full-duplex [VoiceSession].
type VoiceSessionState string

const (
	// VoiceSessionStateConnecting indicates the session transport is handshaking.
	VoiceSessionStateConnecting VoiceSessionState = "connecting"
	// VoiceSessionStateActive indicates the session is open and idle between turns.
	VoiceSessionStateActive VoiceSessionState = "active"
	// VoiceSessionStateListening indicates the session is actively capturing user speech.
	VoiceSessionStateListening VoiceSessionState = "listening"
	// VoiceSessionStateThinking indicates the session is running LLM or tool execution.
	VoiceSessionStateThinking VoiceSessionState = "thinking"
	// VoiceSessionStateSpeaking indicates the session is streaming assistant audio output.
	VoiceSessionStateSpeaking VoiceSessionState = "speaking"
	// VoiceSessionStateInterrupted indicates the current assistant turn was interrupted.
	VoiceSessionStateInterrupted VoiceSessionState = "interrupted"
	// VoiceSessionStateClosed indicates the session has terminated.
	VoiceSessionStateClosed VoiceSessionState = "closed"
)

// IsValid reports whether s is a recognized [VoiceSessionState].
func (s VoiceSessionState) IsValid() bool {
	switch s {
	case VoiceSessionStateConnecting,
		VoiceSessionStateActive,
		VoiceSessionStateListening,
		VoiceSessionStateThinking,
		VoiceSessionStateSpeaking,
		VoiceSessionStateInterrupted,
		VoiceSessionStateClosed:
		return true
	default:
		return false
	}
}

// VoiceSessionConfig configures a full-duplex real-time [VoiceSession].
//
// Privacy: String and GoString omit SystemPrompt content and Metadata values so
// default logging never exposes sensitive instructions or user identifiers.
type VoiceSessionConfig struct {
	// SessionID optionally supplies a caller-assigned correlation identifier.
	// Empty allows the provider or pipeline to generate one.
	SessionID string `json:"session_id,omitempty"`

	// AgentID optionally identifies a hosted agent configuration (e.g., ElevenAgents).
	AgentID string `json:"agent_id,omitempty"`

	// Model optionally identifies the primary model for the voice session.
	Model ModelID `json:"model,omitempty"`

	// VoiceID optionally identifies the output synthesis voice.
	VoiceID string `json:"voice_id,omitempty"`

	// VoiceSettings specifies optional synthesis tuning parameters.
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`

	// SystemPrompt provides optional session-level instructions.
	// Privacy: omitted from String() and GoString().
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Language is an optional ISO-639 / BCP-47 language hint (e.g., "en").
	Language string `json:"language,omitempty"`

	// InputFormat specifies the expected incoming user audio format.
	// Zero uses the provider or pipeline default.
	InputFormat AudioFormat `json:"input_format,omitempty"`

	// OutputFormat specifies the desired outgoing assistant audio format.
	// Zero uses the provider or pipeline default.
	OutputFormat AudioFormat `json:"output_format,omitempty"`

	// Tools registers optional callable tools exposed during the voice session.
	Tools []Tool `json:"-"`

	// EventBufferSize optionally sets the bounded capacity of the session's Events channel.
	// Zero uses the provider or pipeline default.
	EventBufferSize int `json:"event_buffer_size,omitempty"`

	// MaxAudioChunkBytes optionally bounds the maximum bytes permitted per SendAudio chunk.
	// Zero uses the provider or pipeline default.
	MaxAudioChunkBytes int `json:"max_audio_chunk_bytes,omitempty"`

	// Metadata holds optional provider-agnostic key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks that VoiceSessionConfig carries valid audio formats, voice
// settings, tool definitions, and non-negative buffer limits. Returned errors
// wrap [ErrInvalidVoiceSessionConfig].
func (c VoiceSessionConfig) Validate() error {
	if !c.InputFormat.IsZero() {
		if err := c.InputFormat.Validate(); err != nil {
			return fmt.Errorf("%w: input_format: %w", ErrInvalidVoiceSessionConfig, err)
		}
	}
	if !c.OutputFormat.IsZero() {
		if err := c.OutputFormat.Validate(); err != nil {
			return fmt.Errorf("%w: output_format: %w", ErrInvalidVoiceSessionConfig, err)
		}
	}
	if err := c.VoiceSettings.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidVoiceSessionConfig, err)
	}
	if c.EventBufferSize < 0 {
		return fmt.Errorf("%w: event_buffer_size must be non-negative, got %d", ErrInvalidVoiceSessionConfig, c.EventBufferSize)
	}
	if c.MaxAudioChunkBytes < 0 {
		return fmt.Errorf("%w: max_audio_chunk_bytes must be non-negative, got %d", ErrInvalidVoiceSessionConfig, c.MaxAudioChunkBytes)
	}
	for i, tool := range c.Tools {
		if tool == nil {
			return fmt.Errorf("%w: tools[%d] cannot be nil", ErrInvalidVoiceSessionConfig, i)
		}
		if strings.TrimSpace(tool.Name()) == "" {
			return fmt.Errorf("%w: tools[%d] name cannot be empty", ErrInvalidVoiceSessionConfig, i)
		}
	}
	return nil
}

// String returns a log-safe summary omitting SystemPrompt content and Metadata values.
func (c VoiceSessionConfig) String() string {
	return fmt.Sprintf(
		"VoiceSessionConfig{session_id=%q, agent_id=%q, model=%q, voice_id=%q, input=%s, output=%s, prompt_len=%d, tools=%d, metadata_keys=%d}",
		c.SessionID, c.AgentID, c.Model, c.VoiceID, c.InputFormat.String(), c.OutputFormat.String(),
		len(c.SystemPrompt), len(c.Tools), len(c.Metadata),
	)
}

// GoString returns a log-safe representation for %#v formatting.
func (c VoiceSessionConfig) GoString() string {
	return "core." + c.String()
}

// VoiceEventType identifies the kind of event emitted by a [VoiceSession].
type VoiceEventType string

const (
	// Lifecycle events
	VoiceEventSessionStarted VoiceEventType = "session_started"
	VoiceEventSessionState   VoiceEventType = "session_state"
	VoiceEventSessionEnded   VoiceEventType = "session_ended"

	// Turn lifecycle and interruption events
	VoiceEventTurnStarted     VoiceEventType = "turn_started"
	VoiceEventTurnCompleted   VoiceEventType = "turn_completed"
	VoiceEventTurnInterrupted VoiceEventType = "turn_interrupted"

	// User and agent transcript events (partial vs committed)
	VoiceEventUserTranscriptPartial   VoiceEventType = "user_transcript_partial"
	VoiceEventUserTranscriptCommitted VoiceEventType = "user_transcript_committed"
	VoiceEventAgentTextDelta          VoiceEventType = "agent_text_delta"
	VoiceEventAgentTextCommitted      VoiceEventType = "agent_text_committed"

	// Synthesized audio output events
	VoiceEventAudioOutput      VoiceEventType = "audio_output"
	VoiceEventAudioOutputEnded VoiceEventType = "audio_output_ended"

	// Tool invocation and result events
	VoiceEventToolCall   VoiceEventType = "tool_call"
	VoiceEventToolResult VoiceEventType = "tool_result"

	// Error events
	VoiceEventError VoiceEventType = "error"
)

// IsValid reports whether t is a recognized [VoiceEventType].
func (t VoiceEventType) IsValid() bool {
	switch t {
	case VoiceEventSessionStarted,
		VoiceEventSessionState,
		VoiceEventSessionEnded,
		VoiceEventTurnStarted,
		VoiceEventTurnCompleted,
		VoiceEventTurnInterrupted,
		VoiceEventUserTranscriptPartial,
		VoiceEventUserTranscriptCommitted,
		VoiceEventAgentTextDelta,
		VoiceEventAgentTextCommitted,
		VoiceEventAudioOutput,
		VoiceEventAudioOutputEnded,
		VoiceEventToolCall,
		VoiceEventToolResult,
		VoiceEventError:
		return true
	default:
		return false
	}
}

// VoiceInterruptionReason classifies why an active assistant turn was cancelled.
type VoiceInterruptionReason string

const (
	// VoiceInterruptionBargeIn indicates user speech interrupted assistant playback.
	VoiceInterruptionBargeIn VoiceInterruptionReason = "barge_in"
	// VoiceInterruptionClientCancel indicates the caller explicitly invoked Interrupt.
	VoiceInterruptionClientCancel VoiceInterruptionReason = "client_cancel"
	// VoiceInterruptionSessionClosed indicates the session closed while a turn was active.
	VoiceInterruptionSessionClosed VoiceInterruptionReason = "session_closed"
)

// IsValid reports whether r is a recognized [VoiceInterruptionReason] (including empty default).
func (r VoiceInterruptionReason) IsValid() bool {
	switch r {
	case "", VoiceInterruptionBargeIn, VoiceInterruptionClientCancel, VoiceInterruptionSessionClosed:
		return true
	default:
		return false
	}
}

// VoiceInterruption describes the cancellation of an active conversational turn
// so downstream audio playback consumers immediately discard stale audio chunks
// belonging to CancelledTurnID.
type VoiceInterruption struct {
	// CancelledTurnID identifies the interrupted turn whose remaining audio MUST be dropped.
	CancelledTurnID string `json:"cancelled_turn_id"`

	// NextTurnID optionally identifies the superseding turn when known.
	NextTurnID string `json:"next_turn_id,omitempty"`

	// Reason describes what triggered the interruption.
	Reason VoiceInterruptionReason `json:"reason,omitempty"`

	// AudioSequenceCutoff optionally records the last audio sequence emitted prior to cancellation.
	AudioSequenceCutoff int64 `json:"audio_sequence_cutoff,omitempty"`
}

// VoiceEvent represents a typed lifecycle, transcript, audio, interruption,
// tool, or error event emitted during a [VoiceSession].
//
// TurnID correlates all transcripts, assistant text deltas, synthesized audio
// chunks, tool calls, and interruptions belonging to the same conversational
// turn. Consumers playing audio should pair VoiceEvent with [VoiceTurnTracker]
// so chunks from interrupted turns are never played after barge-in.
//
// Privacy: String and GoString emit metadata summaries only and never include
// raw audio bytes, transcript text, assistant text, or tool argument/result
// payloads.
type VoiceEvent struct {
	// Type identifies the event kind.
	Type VoiceEventType `json:"type"`

	// SessionID identifies the enclosing voice session.
	SessionID string `json:"session_id,omitempty"`

	// TurnID correlates events belonging to a single conversational turn.
	TurnID string `json:"turn_id,omitempty"`

	// Sequence is a monotonically increasing per-session event index.
	Sequence int64 `json:"sequence,omitempty"`

	// Timestamp records when the SDK produced or received the event.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// State carries the session lifecycle state for state/lifecycle events.
	State VoiceSessionState `json:"state,omitempty"`

	// Role identifies the speaker ("user", "assistant", "tool") when applicable.
	Role Role `json:"role,omitempty"`

	// Transcript carries speech-to-text data for user transcript events.
	Transcript *TranscriptionResponse `json:"transcript,omitempty"`

	// Text carries incremental or committed assistant text for agent text events.
	// Privacy: omitted from String() and GoString().
	Text string `json:"text,omitempty"`

	// IsFinal distinguishes committed utterances/text (true) from interim partials (false).
	IsFinal bool `json:"is_final,omitempty"`

	// Audio carries an incremental audio chunk for VoiceEventAudioOutput.
	Audio *AudioChunk `json:"audio,omitempty"`

	// Interruption carries cancellation details for VoiceEventTurnInterrupted.
	Interruption *VoiceInterruption `json:"interruption,omitempty"`

	// ToolCall carries a model-requested tool invocation for VoiceEventToolCall.
	ToolCall *ToolCall `json:"tool_call,omitempty"`

	// ToolResult carries a tool execution outcome for VoiceEventToolResult.
	ToolResult *ToolResult `json:"tool_result,omitempty"`

	// Err carries the Go error for VoiceEventError (excluded from JSON serialization).
	Err error `json:"-"`

	// ErrorMessage carries a safe string description when serialized to JSON.
	ErrorMessage string `json:"error_message,omitempty"`

	// Metadata holds optional non-sensitive event metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks structural consistency of a VoiceEvent against its Type.
func (e VoiceEvent) Validate() error {
	if !e.Type.IsValid() {
		return fmt.Errorf("invalid voice event: unsupported type %q", e.Type)
	}
	if e.Sequence < 0 {
		return fmt.Errorf("invalid voice event: sequence must be non-negative, got %d", e.Sequence)
	}
	if e.State != "" && !e.State.IsValid() {
		return fmt.Errorf("invalid voice event: unsupported state %q", e.State)
	}

	switch e.Type {
	case VoiceEventSessionState:
		if e.State == "" {
			return errors.New("invalid voice event: session_state event requires state")
		}
	case VoiceEventTurnStarted, VoiceEventTurnCompleted:
		if strings.TrimSpace(e.TurnID) == "" {
			return fmt.Errorf("invalid voice event: %s event requires turn_id", e.Type)
		}
	case VoiceEventTurnInterrupted:
		if e.Interruption == nil || strings.TrimSpace(e.Interruption.CancelledTurnID) == "" {
			return errors.New("invalid voice event: turn_interrupted event requires interruption.cancelled_turn_id")
		}
		if !e.Interruption.Reason.IsValid() {
			return fmt.Errorf("invalid voice event: unsupported interruption reason %q", e.Interruption.Reason)
		}
	case VoiceEventUserTranscriptPartial, VoiceEventUserTranscriptCommitted:
		if e.Transcript == nil {
			return fmt.Errorf("invalid voice event: %s event requires transcript", e.Type)
		}
		if e.Type == VoiceEventUserTranscriptPartial && e.IsFinal {
			return errors.New("invalid voice event: user_transcript_partial must not set is_final=true")
		}
		if e.Type == VoiceEventUserTranscriptCommitted && !e.IsFinal {
			return errors.New("invalid voice event: user_transcript_committed must set is_final=true")
		}
	case VoiceEventAgentTextDelta:
		if e.IsFinal {
			return errors.New("invalid voice event: agent_text_delta must not set is_final=true")
		}
	case VoiceEventAgentTextCommitted:
		if !e.IsFinal {
			return errors.New("invalid voice event: agent_text_committed must set is_final=true")
		}
	case VoiceEventAudioOutput:
		if e.Audio == nil {
			return errors.New("invalid voice event: audio_output event requires audio chunk")
		}
		if err := e.Audio.Validate(); err != nil {
			return fmt.Errorf("invalid voice event: %w", err)
		}
	case VoiceEventToolCall:
		if e.ToolCall == nil || strings.TrimSpace(e.ToolCall.Name) == "" {
			return errors.New("invalid voice event: tool_call event requires tool_call.name")
		}
	case VoiceEventToolResult:
		if e.ToolResult == nil || strings.TrimSpace(e.ToolResult.CallID) == "" {
			return errors.New("invalid voice event: tool_result event requires tool_result.call_id")
		}
	case VoiceEventError:
		if e.Err == nil && strings.TrimSpace(e.ErrorMessage) == "" {
			return errors.New("invalid voice event: error event requires err or error_message")
		}
	}
	return nil
}

// String returns a log-safe summary omitting raw transcript text, assistant
// text, audio bytes, and tool argument/result payloads.
func (e VoiceEvent) String() string {
	var parts []string
	parts = append(parts,
		fmt.Sprintf("type=%q", e.Type),
		fmt.Sprintf("session_id=%q", e.SessionID),
		fmt.Sprintf("turn_id=%q", e.TurnID),
		fmt.Sprintf("seq=%d", e.Sequence),
	)
	if e.State != "" {
		parts = append(parts, fmt.Sprintf("state=%q", e.State))
	}
	if e.IsFinal {
		parts = append(parts, "final=true")
	}
	if e.Text != "" {
		parts = append(parts, fmt.Sprintf("text_len=%d", len(e.Text)))
	}
	if e.Transcript != nil {
		parts = append(parts, "transcript="+e.Transcript.String())
	}
	if e.Audio != nil {
		parts = append(parts, "audio="+e.Audio.String())
	}
	if e.Interruption != nil {
		parts = append(parts, fmt.Sprintf("interruption={cancelled=%q, reason=%q}",
			e.Interruption.CancelledTurnID, e.Interruption.Reason))
	}
	if e.ToolCall != nil {
		parts = append(parts, fmt.Sprintf("tool_call={id=%q, name=%q}", e.ToolCall.ID, e.ToolCall.Name))
	}
	if e.ToolResult != nil {
		parts = append(parts, fmt.Sprintf("tool_result={call_id=%q, is_error=%t}", e.ToolResult.CallID, e.ToolResult.IsError))
	}
	if e.Err != nil || e.ErrorMessage != "" {
		parts = append(parts, "has_err=true")
	}
	return "VoiceEvent{" + strings.Join(parts, ", ") + "}"
}

// GoString returns a log-safe representation for %#v formatting.
func (e VoiceEvent) GoString() string {
	return "core." + e.String()
}

// VoiceSession represents an active full-duplex real-time voice conversation
// session (whether backed by an Iris-managed STT/LLM/TTS pipeline or a hosted
// provider such as ElevenAgents or Speech Engine).
//
// Implementations MUST:
//   - Maintain a bounded Events() channel so slow consumers cannot cause
//     unbounded memory growth
//   - Close Events() and Err() promptly when the session ends or the parent
//     context cancels
//   - Cancel in-flight generation and emit VoiceEventTurnInterrupted when
//     Interrupt is invoked during an active turn
//   - Make Close() idempotent and safe for concurrent calls
type VoiceSession interface {
	// ID returns the session identifier.
	ID() string

	// State returns the current lifecycle state of the session.
	State() VoiceSessionState

	// SendAudio validates and transmits an incremental user audio chunk.
	SendAudio(ctx context.Context, chunk AudioChunk) error

	// SendText injects a user text utterance into the voice session.
	SendText(ctx context.Context, text string) error

	// SubmitToolResult delivers the result for a prior VoiceEventToolCall.
	SubmitToolResult(ctx context.Context, result ToolResult) error

	// Interrupt cancels the active assistant turn and suppresses stale audio playback.
	Interrupt(ctx context.Context) error

	// Events returns a bounded receive-only stream of typed VoiceEvent values.
	// Closed when the session terminates.
	Events() <-chan VoiceEvent

	// Err returns a receive-only channel emitting at most one terminal error before closing.
	Err() <-chan error

	// Close terminates the session and releases all underlying connections and goroutines.
	Close() error
}

// VoiceSessionProvider is an optional interface for providers that natively
// support full-duplex real-time voice sessions.
type VoiceSessionProvider interface {
	// StartVoiceSession opens a new full-duplex voice session.
	StartVoiceSession(ctx context.Context, cfg VoiceSessionConfig) (VoiceSession, error)
}

// AsVoiceSessionProvider discovers full-duplex voice session support on a
// Provider or its [ProviderUnwrapper] chain. It returns nil and false when no
// provider implements [VoiceSessionProvider].
func AsVoiceSessionProvider(p Provider) (VoiceSessionProvider, bool) {
	return asProviderCapability[VoiceSessionProvider](p)
}

// StartVoiceSession opens a full-duplex real-time voice session through the
// client's provider. It validates cfg, verifies [VoiceSessionProvider] support,
// and applies client rate limiting.
func (c *Client) StartVoiceSession(ctx context.Context, cfg VoiceSessionConfig) (VoiceSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: voice session context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	vsp, ok := AsVoiceSessionProvider(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support voice sessions: %w", c.speechProviderID(), ErrNotSupported)
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	sess, err := vsp.StartVoiceSession(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return ObserveVoiceSession(ctx, sess, c.voiceTelemetry, VoiceSessionObserverMeta{
		Provider:     c.speechProviderID(),
		Mode:         "provider",
		Model:        cfg.Model,
		VoiceID:      cfg.VoiceID,
		InputFormat:  cfg.InputFormat,
		OutputFormat: cfg.OutputFormat,
	}), nil
}

// VoiceTurnTracker coordinates active turn IDs, per-turn cancellation contexts,
// and stale-audio filtering after interruptions (barge-in or client cancellation).
//
// It can be used both on the producer side (starting, interrupting, and
// completing turns) and on the consumer side (via [VoiceTurnTracker.ObserveEvent]
// and [VoiceTurnTracker.ShouldPlayAudio]).
//
// VoiceTurnTracker is safe for concurrent use across goroutines.
type VoiceTurnTracker struct {
	mu           sync.Mutex
	prefix       string
	counter      uint64
	activeTurnID string
	activeCancel context.CancelFunc
	interrupted  bool
	cancelled    map[string]struct{}
	cancelledLog []string
}

// NewVoiceTurnTracker creates a new tracker generating monotonic turn IDs with
// prefix (defaults to "turn" when empty, producing "turn-1", "turn-2", ...).
func NewVoiceTurnTracker(prefix string) *VoiceTurnTracker {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "turn"
	}
	return &VoiceTurnTracker{
		prefix:    prefix,
		cancelled: make(map[string]struct{}),
	}
}

// StartTurn begins a new turn using context.Background() as the parent context.
func (t *VoiceTurnTracker) StartTurn() (string, context.Context) {
	return t.StartTurnWithContext(context.Background())
}

// StartTurnWithContext begins a new turn derived from parent, cancelling any
// previously active turn so superseded generation stops promptly.
func (t *VoiceTurnTracker) StartTurnWithContext(parent context.Context) (string, context.Context) {
	if parent == nil {
		parent = context.Background()
	}

	t.mu.Lock()
	var prevCancel context.CancelFunc
	if t.activeTurnID != "" {
		t.recordCancelledLocked(t.activeTurnID)
		prevCancel = t.activeCancel
		t.activeCancel = nil
	}
	t.counter++
	turnID := fmt.Sprintf("%s-%d", t.prefix, t.counter)
	turnCtx, cancel := context.WithCancel(parent)
	t.activeTurnID = turnID
	t.activeCancel = cancel
	t.interrupted = false
	t.mu.Unlock()

	if prevCancel != nil {
		prevCancel()
	}
	return turnID, turnCtx
}

// ActiveTurnID returns the currently active non-interrupted turn ID, or "" if
// no turn is active or the active turn has been interrupted.
func (t *VoiceTurnTracker) ActiveTurnID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.interrupted {
		return ""
	}
	return t.activeTurnID
}

// Interrupt cancels the currently active turn (if any), marks it cancelled so
// stale audio chunks are suppressed, and returns the resulting interruption
// payload and true. Returns zero value and false if no active turn exists.
func (t *VoiceTurnTracker) Interrupt(reason VoiceInterruptionReason) (VoiceInterruption, bool) {
	t.mu.Lock()
	if t.activeTurnID == "" || t.interrupted {
		t.mu.Unlock()
		return VoiceInterruption{}, false
	}
	cancelledID := t.activeTurnID
	cancel := t.activeCancel
	t.activeCancel = nil
	t.interrupted = true
	t.recordCancelledLocked(cancelledID)
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return VoiceInterruption{
		CancelledTurnID: cancelledID,
		Reason:          reason,
	}, true
}

// CompleteTurn marks turnID as finished if it is still the active turn and
// releases its cancellation context. Returns true if turnID was completed.
func (t *VoiceTurnTracker) CompleteTurn(turnID string) bool {
	t.mu.Lock()
	if turnID == "" || t.activeTurnID != turnID || t.interrupted {
		t.mu.Unlock()
		return false
	}
	cancel := t.activeCancel
	t.activeTurnID = ""
	t.activeCancel = nil
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return true
}

// IsCurrentTurn reports whether turnID is currently active and has not been
// interrupted or superseded.
func (t *VoiceTurnTracker) IsCurrentTurn(turnID string) bool {
	if turnID == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.interrupted || t.activeTurnID != turnID {
		return false
	}
	_, wasCancelled := t.cancelled[turnID]
	return !wasCancelled
}

// ObserveEvent updates consumer-side turn tracking state from an incoming
// [VoiceEvent] stream (TurnStarted, TurnInterrupted, TurnCompleted, SessionEnded).
func (t *VoiceTurnTracker) ObserveEvent(evt VoiceEvent) {
	t.mu.Lock()
	var cancel context.CancelFunc
	switch evt.Type {
	case VoiceEventTurnStarted:
		if evt.TurnID != "" {
			if _, wasCancelled := t.cancelled[evt.TurnID]; !wasCancelled {
				t.activeTurnID = evt.TurnID
				t.interrupted = false
			}
		}
	case VoiceEventTurnInterrupted:
		var target string
		if evt.Interruption != nil && evt.Interruption.CancelledTurnID != "" {
			target = evt.Interruption.CancelledTurnID
		} else {
			target = evt.TurnID
		}
		if target != "" {
			t.recordCancelledLocked(target)
			if t.activeTurnID == target {
				t.interrupted = true
				cancel = t.activeCancel
				t.activeCancel = nil
			}
		}
	case VoiceEventTurnCompleted:
		if evt.TurnID != "" && t.activeTurnID == evt.TurnID {
			t.activeTurnID = ""
			t.interrupted = false
			cancel = t.activeCancel
			t.activeCancel = nil
		}
	case VoiceEventSessionEnded:
		if t.activeTurnID != "" {
			t.recordCancelledLocked(t.activeTurnID)
			t.activeTurnID = ""
			t.interrupted = true
			cancel = t.activeCancel
			t.activeCancel = nil
		}
	}
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// ShouldPlayAudio reports whether evt is a [VoiceEventAudioOutput] belonging to
// the active non-interrupted turn. Consumers should drop any audio event for
// which ShouldPlayAudio returns false.
func (t *VoiceTurnTracker) ShouldPlayAudio(evt VoiceEvent) bool {
	if evt.Type != VoiceEventAudioOutput || evt.Audio == nil {
		return false
	}
	return t.IsCurrentTurn(evt.TurnID)
}

// Close cancels any active turn and releases resources. Idempotent and safe for
// concurrent calls.
func (t *VoiceTurnTracker) Close() {
	_, _ = t.Interrupt(VoiceInterruptionSessionClosed)
}

func (t *VoiceTurnTracker) recordCancelledLocked(turnID string) {
	if turnID == "" {
		return
	}
	if _, exists := t.cancelled[turnID]; exists {
		return
	}
	if len(t.cancelledLog) >= maxCancelledTurnHistory {
		oldest := t.cancelledLog[0]
		delete(t.cancelled, oldest)
		copy(t.cancelledLog, t.cancelledLog[1:])
		t.cancelledLog = t.cancelledLog[:len(t.cancelledLog)-1]
	}
	t.cancelled[turnID] = struct{}{}
	t.cancelledLog = append(t.cancelledLog, turnID)
}

// VoiceEventEmitter provides bounded, race-free event delivery and clean
// channel closure for [VoiceSession] implementations.
//
// Concurrent calls to Emit, Fail, and Close are safe and never panic with
// send-on-closed-channel or deadlock when Events() is full.
type VoiceEventEmitter struct {
	sessionID string
	events    chan VoiceEvent
	errCh     chan error

	doneOnce sync.Once
	done     chan struct{}

	mu     sync.RWMutex
	closed bool

	seq atomic.Int64
}

// NewVoiceEventEmitter creates a bounded VoiceEventEmitter for sessionID.
// Non-positive bufferSize defaults to [DefaultVoiceEventBufferSize].
func NewVoiceEventEmitter(sessionID string, bufferSize int) *VoiceEventEmitter {
	if bufferSize <= 0 {
		bufferSize = DefaultVoiceEventBufferSize
	}
	return &VoiceEventEmitter{
		sessionID: sessionID,
		events:    make(chan VoiceEvent, bufferSize),
		errCh:     make(chan error, 1),
		done:      make(chan struct{}),
	}
}

// Events returns the bounded receive-only event stream channel.
func (e *VoiceEventEmitter) Events() <-chan VoiceEvent {
	return e.events
}

// Err returns the receive-only terminal error channel (buffered with capacity 1).
func (e *VoiceEventEmitter) Err() <-chan error {
	return e.errCh
}

// Emit delivers evt to the bounded Events() channel, stamping SessionID,
// monotonic Sequence, and Timestamp when unset. Returns [ErrVoiceSessionClosed]
// if the emitter is closed or ctx.Err() if ctx cancels before space is available.
func (e *VoiceEventEmitter) Emit(ctx context.Context, evt VoiceEvent) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-e.done:
		return ErrVoiceSessionClosed
	default:
	}

	e.mu.RLock()
	if e.closed {
		e.mu.RUnlock()
		return ErrVoiceSessionClosed
	}

	if evt.SessionID == "" {
		evt.SessionID = e.sessionID
	}
	if evt.Sequence == 0 {
		evt.Sequence = e.seq.Add(1)
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	if evt.Type == VoiceEventError && evt.Err != nil && evt.ErrorMessage == "" {
		evt.ErrorMessage = evt.Err.Error()
	}

	select {
	case e.events <- evt:
		e.mu.RUnlock()
		return nil
	case <-e.done:
		e.mu.RUnlock()
		return ErrVoiceSessionClosed
	case <-ctx.Done():
		e.mu.RUnlock()
		return ctx.Err()
	}
}

// Fail terminates the emitter with err (emitting err on Err() if non-nil) and
// closes both Events() and Err() promptly. Subsequent Fail/Close calls are no-ops.
func (e *VoiceEventEmitter) Fail(err error) {
	e.doneOnce.Do(func() {
		close(e.done)
	})

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	close(e.events)
	if err != nil {
		e.errCh <- err
	}
	close(e.errCh)
	e.mu.Unlock()
}

// Close terminates the emitter cleanly without a terminal error and closes both
// Events() and Err(). Idempotent and safe for concurrent calls.
func (e *VoiceEventEmitter) Close() {
	e.Fail(nil)
}
