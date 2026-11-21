package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// Pipeline errors and bounded defaults.
var (
	// ErrInvalidPipelineConfig indicates a PipelineConfig failed validation.
	ErrInvalidPipelineConfig = errors.New("invalid voice pipeline config")

	// ErrToolCallNotFound indicates SubmitToolResult referenced an unknown or expired CallID.
	ErrToolCallNotFound = errors.New("voice tool call not pending")

	// ErrMaxToolRoundsExceeded indicates an LLM turn exceeded PipelineConfig.MaxToolRounds.
	ErrMaxToolRoundsExceeded = errors.New("voice turn exceeded max tool rounds")
)

const (
	// DefaultMaxToolRounds caps sequential tool-calling iterations per turn.
	DefaultMaxToolRounds = 5

	// DefaultMaxHistoryMessages bounds retained non-system conversation messages.
	DefaultMaxHistoryMessages = 64

	// DefaultToolTimeout bounds automatic tool execution when no shorter deadline exists.
	DefaultToolTimeout = 30 * time.Second
)

// ToolExecutor executes model-requested tool calls during a voice turn.
type ToolExecutor interface {
	Execute(ctx context.Context, call core.ToolCall) (any, error)
}

// ToolExecutorFunc adapts a function to [ToolExecutor].
type ToolExecutorFunc func(ctx context.Context, call core.ToolCall) (any, error)

// Execute invokes f(ctx, call).
func (f ToolExecutorFunc) Execute(ctx context.Context, call core.ToolCall) (any, error) {
	return f(ctx, call)
}

// RegistryToolExecutor adapts a [*tools.Registry] to [ToolExecutor] so voice
// turns automatically inherit any registry-configured tool middlewares.
func RegistryToolExecutor(reg *tools.Registry) ToolExecutor {
	if reg == nil {
		return nil
	}
	return ToolExecutorFunc(func(ctx context.Context, call core.ToolCall) (any, error) {
		return reg.Execute(ctx, call.Name, call.Arguments)
	})
}

// PipelineConfig configures an Iris-managed streaming voice [Pipeline].
//
// Privacy: String and GoString omit SystemPrompt content so default logging
// never exposes sensitive instructions.
type PipelineConfig struct {
	// STT is an optional real-time speech recognizer (e.g., ElevenLabs Scribe).
	// Required for sessions that call SendAudio; may be nil for text-driven sessions.
	STT core.RealtimeSpeechRecognizer

	// STTModel overrides the default real-time STT model.
	STTModel core.ModelID

	// CommitStrategy configures STT utterance finalization (manual or VAD).
	CommitStrategy core.CommitStrategy

	// IncludeTimestamps requests word-level timestamps on committed STT transcripts.
	IncludeTimestamps bool

	// LLM is the Iris chat provider used for conversational reasoning. Required.
	LLM core.Provider

	// LLMModel is the default chat model ID used for LLM turns. Can be overridden
	// per session via core.VoiceSessionConfig.Model.
	LLMModel core.ModelID

	// RealtimeTTS is an optional incremental-input streaming TTS synthesizer
	// (e.g., ElevenLabs WebSocket TTS). Preferred when set so LLM tokens stream
	// directly to audio synthesis.
	RealtimeTTS core.RealtimeSpeechSynthesizer

	// TTS is an optional HTTP streaming TTS synthesizer fallback used when
	// RealtimeTTS is nil. At least one of RealtimeTTS or TTS is required.
	TTS core.SpeechSynthesizer

	// TTSModel overrides the default TTS model.
	TTSModel core.ModelID

	// VoiceID is the default voice identifier for TTS synthesis. Can be
	// overridden per session via core.VoiceSessionConfig.VoiceID.
	VoiceID string

	// VoiceSettings provides default synthesis tuning knobs (overridable per session).
	VoiceSettings *core.VoiceSettings

	// ChunkLengthSchedule optionally hints at character thresholds for RealtimeTTS.
	ChunkLengthSchedule []int

	// AutoMode enables provider-managed low-latency chunking on RealtimeTTS when supported.
	AutoMode bool

	// SystemPrompt sets default system instructions for LLM turns (overridable per session).
	// Privacy: omitted from String() and GoString().
	SystemPrompt string

	// Temperature optionally overrides the LLM sampling temperature.
	Temperature *float32

	// MaxTokens optionally caps LLM output tokens per round.
	MaxTokens *int

	// Tools registers default tools exposed to the LLM. Any entry implementing
	// [tools.Tool] is executed automatically unless ToolExecutor handles it;
	// entries implementing only [core.Tool] pause for caller SubmitToolResult.
	Tools []core.Tool

	// ToolExecutor optionally executes model-requested tool calls automatically.
	ToolExecutor ToolExecutor

	// MaxToolRounds caps sequential tool-calling iterations within a single turn.
	// Zero uses DefaultMaxToolRounds (5).
	MaxToolRounds int

	// MaxHistoryMessages bounds retained conversation messages (excluding system prompt).
	// Zero uses DefaultMaxHistoryMessages (64).
	MaxHistoryMessages int

	// DisableBargeIn disables automatic turn interruption when non-empty user
	// partial speech arrives during an active assistant turn.
	DisableBargeIn bool

	// EventBufferSize overrides the default bounded capacity of Session.Events().
	// Zero uses core.DefaultVoiceEventBufferSize (64).
	EventBufferSize int

	// ToolTimeout bounds each automatic tool execution when no shorter deadline exists.
	// Zero uses DefaultToolTimeout (30s).
	ToolTimeout time.Duration
}

// Validate checks that PipelineConfig has required providers and valid bounds.
// Returned errors wrap [ErrInvalidPipelineConfig].
func (c PipelineConfig) Validate() error {
	if c.LLM == nil {
		return fmt.Errorf("%w: LLM provider is required", ErrInvalidPipelineConfig)
	}
	if c.RealtimeTTS == nil && c.TTS == nil {
		return fmt.Errorf("%w: at least one of RealtimeTTS or TTS is required", ErrInvalidPipelineConfig)
	}
	if !c.CommitStrategy.IsValid() {
		return fmt.Errorf("%w: unsupported commit_strategy %q", ErrInvalidPipelineConfig, c.CommitStrategy)
	}
	if err := c.VoiceSettings.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPipelineConfig, err)
	}
	for i, n := range c.ChunkLengthSchedule {
		if n <= 0 {
			return fmt.Errorf("%w: chunk_length_schedule[%d] must be positive, got %d", ErrInvalidPipelineConfig, i, n)
		}
	}
	if c.MaxToolRounds < 0 {
		return fmt.Errorf("%w: max_tool_rounds must be non-negative, got %d", ErrInvalidPipelineConfig, c.MaxToolRounds)
	}
	if c.MaxHistoryMessages < 0 {
		return fmt.Errorf("%w: max_history_messages must be non-negative, got %d", ErrInvalidPipelineConfig, c.MaxHistoryMessages)
	}
	if c.EventBufferSize < 0 {
		return fmt.Errorf("%w: event_buffer_size must be non-negative, got %d", ErrInvalidPipelineConfig, c.EventBufferSize)
	}
	if c.ToolTimeout < 0 {
		return fmt.Errorf("%w: tool_timeout must be non-negative, got %s", ErrInvalidPipelineConfig, c.ToolTimeout)
	}
	for i, t := range c.Tools {
		if t == nil {
			return fmt.Errorf("%w: tools[%d] cannot be nil", ErrInvalidPipelineConfig, i)
		}
		if strings.TrimSpace(t.Name()) == "" {
			return fmt.Errorf("%w: tools[%d] name cannot be empty", ErrInvalidPipelineConfig, i)
		}
	}
	return nil
}

// String returns a log-safe summary omitting SystemPrompt content.
func (c PipelineConfig) String() string {
	llmID := "<nil>"
	if c.LLM != nil {
		llmID = c.LLM.ID()
	}
	return fmt.Sprintf(
		"PipelineConfig{llm=%q, llm_model=%q, has_stt=%t, has_realtime_tts=%t, has_tts=%t, voice_id=%q, prompt_len=%d, tools=%d}",
		llmID, c.LLMModel, c.STT != nil, c.RealtimeTTS != nil, c.TTS != nil, c.VoiceID, len(c.SystemPrompt), len(c.Tools),
	)
}

// GoString returns a log-safe representation for %#v formatting.
func (c PipelineConfig) GoString() string {
	return "voice." + c.String()
}

// Pipeline is a reusable, concurrency-safe factory for Iris-managed
// [core.VoiceSession] instances. It implements [core.VoiceSessionProvider].
type Pipeline struct {
	cfg        PipelineConfig
	sessionSeq atomic.Uint64
}

// NewPipeline validates cfg and returns a new Iris-managed voice [Pipeline].
func NewPipeline(cfg PipelineConfig) (*Pipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.MaxToolRounds == 0 {
		cfg.MaxToolRounds = DefaultMaxToolRounds
	}
	if cfg.MaxHistoryMessages == 0 {
		cfg.MaxHistoryMessages = DefaultMaxHistoryMessages
	}
	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = core.DefaultVoiceEventBufferSize
	}
	if cfg.ToolTimeout == 0 {
		cfg.ToolTimeout = DefaultToolTimeout
	}
	if len(cfg.Tools) > 0 {
		cp := make([]core.Tool, len(cfg.Tools))
		copy(cp, cfg.Tools)
		cfg.Tools = cp
	}
	if len(cfg.ChunkLengthSchedule) > 0 {
		cp := make([]int, len(cfg.ChunkLengthSchedule))
		copy(cp, cfg.ChunkLengthSchedule)
		cfg.ChunkLengthSchedule = cp
	}
	return &Pipeline{cfg: cfg}, nil
}

// StartVoiceSession opens a new Iris-managed full-duplex voice session.
//
// If the pipeline has an STT recognizer configured, StartVoiceSession opens
// the real-time STT stream before returning (defaulting InputFormat to
// [core.AudioFormatPCM16000] when unset). Callers must defer Close() on the
// returned session to guarantee resource and goroutine cleanup.
func (p *Pipeline) StartVoiceSession(ctx context.Context, cfg core.VoiceSessionConfig) (core.VoiceSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("voice: session context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	llmModel := cfg.Model
	if llmModel == "" {
		llmModel = p.cfg.LLMModel
	}
	if llmModel == "" {
		return nil, fmt.Errorf("%w: LLM model is required (set PipelineConfig.LLMModel or VoiceSessionConfig.Model)", core.ErrInvalidVoiceSessionConfig)
	}

	voiceID := strings.TrimSpace(cfg.VoiceID)
	if voiceID == "" {
		voiceID = strings.TrimSpace(p.cfg.VoiceID)
	}
	if voiceID == "" {
		return nil, fmt.Errorf("%w: voice_id is required (set PipelineConfig.VoiceID or VoiceSessionConfig.VoiceID)", core.ErrInvalidVoiceSessionConfig)
	}

	sessionID := strings.TrimSpace(cfg.SessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("voice-sess-%d", p.sessionSeq.Add(1))
	}

	bufSize := cfg.EventBufferSize
	if bufSize <= 0 {
		bufSize = p.cfg.EventBufferSize
	}

	inputFormat := cfg.InputFormat
	if inputFormat.IsZero() {
		inputFormat = core.AudioFormatPCM16000
	}

	var sttSess core.RealtimeTranscriptionSession
	if p.cfg.STT != nil {
		var err error
		sttSess, err = p.cfg.STT.StartRealtimeTranscription(ctx, core.RealtimeTranscriptionConfig{
			Model:             p.cfg.STTModel,
			Format:            inputFormat,
			Language:          cfg.Language,
			CommitStrategy:    p.cfg.CommitStrategy,
			IncludeTimestamps: p.cfg.IncludeTimestamps,
			MaxChunkBytes:     cfg.MaxAudioChunkBytes,
			EventBufferSize:   bufSize,
		})
		if err != nil {
			return nil, fmt.Errorf("voice: start realtime STT: %w", err)
		}
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = p.cfg.SystemPrompt
	}

	voiceSettings := cfg.VoiceSettings
	if voiceSettings == nil {
		voiceSettings = p.cfg.VoiceSettings
	}

	mergedTools := mergeTools(p.cfg.Tools, cfg.Tools)
	toolIndex := make(map[string]core.Tool, len(mergedTools))
	for _, t := range mergedTools {
		toolIndex[t.Name()] = t
	}

	sessCtx, cancel := context.WithCancel(ctx)
	s := &Session{
		id:            sessionID,
		pipeline:      p,
		llmModel:      llmModel,
		voiceID:       voiceID,
		voiceSettings: voiceSettings,
		language:      cfg.Language,
		outputFormat:  cfg.OutputFormat,
		systemPrompt:  systemPrompt,
		tools:         mergedTools,
		toolIndex:     toolIndex,
		stt:           sttSess,
		ctx:           sessCtx,
		cancel:        cancel,
		tracker:       core.NewVoiceTurnTracker("turn"),
		bus:           core.NewVoiceEventEmitter(sessionID, bufSize),
		turnTrigger:   make(chan turnRequest, 1),
		pendingTools:  make(map[string]chan core.ToolResult),
	}
	s.setState(core.VoiceSessionStateActive)

	_ = s.bus.Emit(sessCtx, core.VoiceEvent{
		Type:  core.VoiceEventSessionStarted,
		State: core.VoiceSessionStateActive,
	})

	s.wg.Add(1)
	go s.turnWorker()

	if sttSess != nil {
		s.setState(core.VoiceSessionStateListening)
		_ = s.bus.Emit(sessCtx, core.VoiceEvent{
			Type:  core.VoiceEventSessionState,
			State: core.VoiceSessionStateListening,
		})
		s.wg.Add(1)
		go s.sttLoop()
	}

	s.wg.Add(1)
	go s.watchContext()

	return s, nil
}

func mergeTools(base, override []core.Tool) []core.Tool {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	seen := make(map[string]int, len(base)+len(override))
	out := make([]core.Tool, 0, len(base)+len(override))
	for _, t := range base {
		seen[t.Name()] = len(out)
		out = append(out, t)
	}
	for _, t := range override {
		if idx, exists := seen[t.Name()]; exists {
			out[idx] = t
		} else {
			seen[t.Name()] = len(out)
			out = append(out, t)
		}
	}
	return out
}

// Compile-time check that Pipeline implements core.VoiceSessionProvider.
var _ core.VoiceSessionProvider = (*Pipeline)(nil)
