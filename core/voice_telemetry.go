package core

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// VoiceOperation identifies the speech or voice operation recorded by a
// [VoiceTelemetryHook].
type VoiceOperation string

const (
	VoiceOperationSynthesize         VoiceOperation = "synthesize"
	VoiceOperationStreamSynthesize   VoiceOperation = "stream_synthesize"
	VoiceOperationRealtimeSynthesize VoiceOperation = "realtime_synthesize"
	VoiceOperationTranscribe         VoiceOperation = "transcribe"
	VoiceOperationRealtimeTranscribe VoiceOperation = "realtime_transcribe"
	VoiceOperationVoiceSession       VoiceOperation = "voice_session"
	VoiceOperationVoiceTurn          VoiceOperation = "voice_turn"
)

// VoiceRequestStartEvent carries operational metadata when a speech request begins.
//
// # Security
//
// This struct intentionally excludes API keys, raw audio bytes, synthesis text,
// prompts, and transcripts.
type VoiceRequestStartEvent struct {
	Operation VoiceOperation
	Provider  string
	Model     ModelID
	VoiceID   string
	Format    AudioFormat
	Start     time.Time
}

// VoiceFirstChunkEvent carries timing metadata when the first audio chunk
// arrives for a streaming synthesis request or voice turn.
//
// # Security
//
// Only chunk byte length and timestamps are exposed; raw audio bytes are never
// included.
type VoiceFirstChunkEvent struct {
	Operation    VoiceOperation
	Provider     string
	Model        ModelID
	SessionID    string
	TurnID       string
	Start        time.Time
	FirstChunkAt time.Time
	ChunkBytes   int
}

// FirstChunkLatency returns the elapsed time from request/turn start until the
// first non-empty audio chunk arrived.
func (e VoiceFirstChunkEvent) FirstChunkLatency() time.Duration {
	if e.FirstChunkAt.IsZero() || e.Start.IsZero() {
		return 0
	}
	return e.FirstChunkAt.Sub(e.Start)
}

// VoiceRequestEndEvent carries operational usage, latency, and error metadata
// when a speech request completes.
//
// # Security
//
// Excludes API keys, raw audio bytes, synthesis text, and transcript content.
type VoiceRequestEndEvent struct {
	Operation         VoiceOperation
	Provider          string
	Model             ModelID
	VoiceID           string
	Start             time.Time
	End               time.Time
	FirstChunkLatency time.Duration
	InputBytes        int64
	OutputBytes       int64
	InputCharacters   int
	AudioDuration     time.Duration
	Chunks            int
	Err               error
}

// Duration returns the total elapsed duration of the speech request.
func (e VoiceRequestEndEvent) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// VoiceSessionStartEvent carries operational metadata when a full-duplex voice
// session opens.
//
// # Security
//
// Excludes API keys, signed URLs, system prompts, and user metadata values.
type VoiceSessionStartEvent struct {
	SessionID    string
	Provider     string
	Mode         string
	Model        ModelID
	VoiceID      string
	InputFormat  AudioFormat
	OutputFormat AudioFormat
	Start        time.Time
}

// VoiceTurnEndEvent carries per-turn latency, audio volume, tool-call count,
// interruption status, and error metadata for a single conversational turn.
//
// # Security
//
// Excludes user transcripts, assistant text, raw audio bytes, and tool
// argument/result payloads.
type VoiceTurnEndEvent struct {
	SessionID          string
	TurnID             string
	Provider           string
	Model              ModelID
	Start              time.Time
	End                time.Time
	FirstAudioLatency  time.Duration
	OutputAudioBytes   int64
	AudioChunks        int
	ToolCalls          int
	Interrupted        bool
	InterruptionReason VoiceInterruptionReason
	Err                error
}

// Duration returns the elapsed duration of the conversational turn.
func (e VoiceTurnEndEvent) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// VoiceSessionEndEvent carries aggregate session-level metrics when a
// full-duplex voice session terminates.
//
// # Security
//
// Excludes audio bytes, transcripts, prompts, and credentials.
type VoiceSessionEndEvent struct {
	SessionID        string
	Provider         string
	Mode             string
	Start            time.Time
	End              time.Time
	Turns            int
	Interruptions    int
	ToolCalls        int
	InputAudioBytes  int64
	OutputAudioBytes int64
	ErrorCount       int
	Err              error
}

// Duration returns the total elapsed duration of the voice session.
func (e VoiceSessionEndEvent) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// VoiceTelemetryHook receives privacy-safe operational notifications for speech
// requests, first-audio latency, full-duplex voice sessions, and turns.
//
// OnVoiceRequestStart and OnVoiceSessionStart return a context that may carry
// tracing span information propagated to downstream calls and end callbacks.
//
// # Security
//
// All event structs intentionally exclude API keys, signed URLs, raw audio
// bytes, user transcripts, prompts, assistant text, and tool payloads.
type VoiceTelemetryHook interface {
	OnVoiceRequestStart(ctx context.Context, e VoiceRequestStartEvent) context.Context
	OnVoiceFirstChunk(ctx context.Context, e VoiceFirstChunkEvent)
	OnVoiceRequestEnd(ctx context.Context, e VoiceRequestEndEvent)

	OnVoiceSessionStart(ctx context.Context, e VoiceSessionStartEvent) context.Context
	OnVoiceTurnEnd(ctx context.Context, e VoiceTurnEndEvent)
	OnVoiceSessionEnd(ctx context.Context, e VoiceSessionEndEvent)
}

// NoopVoiceTelemetryHook is a zero-cost no-op implementation of
// [VoiceTelemetryHook]. Embed it to implement only a subset of callbacks.
type NoopVoiceTelemetryHook struct{}

func (NoopVoiceTelemetryHook) OnVoiceRequestStart(ctx context.Context, _ VoiceRequestStartEvent) context.Context {
	return ctx
}
func (NoopVoiceTelemetryHook) OnVoiceFirstChunk(context.Context, VoiceFirstChunkEvent) {}
func (NoopVoiceTelemetryHook) OnVoiceRequestEnd(context.Context, VoiceRequestEndEvent) {}
func (NoopVoiceTelemetryHook) OnVoiceSessionStart(ctx context.Context, _ VoiceSessionStartEvent) context.Context {
	return ctx
}
func (NoopVoiceTelemetryHook) OnVoiceTurnEnd(context.Context, VoiceTurnEndEvent)       {}
func (NoopVoiceTelemetryHook) OnVoiceSessionEnd(context.Context, VoiceSessionEndEvent) {}

var _ VoiceTelemetryHook = NoopVoiceTelemetryHook{}

// VoiceSessionObserverMeta provides static operational metadata attached to
// session and turn telemetry emitted by [ObserveVoiceSession].
type VoiceSessionObserverMeta struct {
	Provider     string
	Mode         string
	Model        ModelID
	VoiceID      string
	InputFormat  AudioFormat
	OutputFormat AudioFormat
}

// ObserveVoiceSession wraps sess so all session lifecycle, per-turn timing,
// first-audio latency, byte volume, tool call, interruption, and error metrics
// are reported to hook without retaining or logging audio or transcript payloads.
//
// If sess is nil or hook is nil (or [NoopVoiceTelemetryHook]), sess is returned
// unchanged.
func ObserveVoiceSession(ctx context.Context, sess VoiceSession, hook VoiceTelemetryHook, meta VoiceSessionObserverMeta) VoiceSession {
	if sess == nil || hook == nil {
		return sess
	}
	if _, isNoop := hook.(NoopVoiceTelemetryHook); isNoop {
		return sess
	}
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	sessCtx := hook.OnVoiceSessionStart(ctx, VoiceSessionStartEvent{
		SessionID:    sess.ID(),
		Provider:     meta.Provider,
		Mode:         meta.Mode,
		Model:        meta.Model,
		VoiceID:      meta.VoiceID,
		InputFormat:  meta.InputFormat,
		OutputFormat: meta.OutputFormat,
		Start:        start,
	})

	outEvents := make(chan VoiceEvent, max(cap(sess.Events()), 1))
	outErr := make(chan error, 1)

	obs := &observedVoiceSession{
		inner:   sess,
		hook:    hook,
		meta:    meta,
		sessCtx: sessCtx,
		start:   start,
		events:  outEvents,
		errCh:   outErr,
		closing: make(chan struct{}),
		done:    make(chan struct{}),
	}

	go obs.forwardLoop()
	return obs
}

type observedTurnState struct {
	id                string
	start             time.Time
	firstAudioLatency time.Duration
	sawAudio          bool
	outputAudioBytes  int64
	audioChunks       int
	toolCalls         int
	lastErr           error
}

type observedVoiceSession struct {
	inner   VoiceSession
	hook    VoiceTelemetryHook
	meta    VoiceSessionObserverMeta
	sessCtx context.Context
	start   time.Time

	events  chan VoiceEvent
	errCh   chan error
	closing chan struct{}
	done    chan struct{}

	inputAudioBytes atomic.Int64
	closeOnce       sync.Once
	closeErr        error
}

func (o *observedVoiceSession) ID() string               { return o.inner.ID() }
func (o *observedVoiceSession) State() VoiceSessionState { return o.inner.State() }

func (o *observedVoiceSession) SendAudio(ctx context.Context, chunk AudioChunk) error {
	err := o.inner.SendAudio(ctx, chunk)
	if err == nil && len(chunk.Data) > 0 {
		o.inputAudioBytes.Add(int64(len(chunk.Data)))
	}
	return err
}

func (o *observedVoiceSession) SendText(ctx context.Context, text string) error {
	return o.inner.SendText(ctx, text)
}

func (o *observedVoiceSession) SubmitToolResult(ctx context.Context, result ToolResult) error {
	return o.inner.SubmitToolResult(ctx, result)
}

func (o *observedVoiceSession) Interrupt(ctx context.Context) error {
	return o.inner.Interrupt(ctx)
}

func (o *observedVoiceSession) Events() <-chan VoiceEvent { return o.events }
func (o *observedVoiceSession) Err() <-chan error         { return o.errCh }

func (o *observedVoiceSession) Close() error {
	o.closeOnce.Do(func() {
		close(o.closing)
		o.closeErr = o.inner.Close()
	})
	<-o.done
	return o.closeErr
}

func (o *observedVoiceSession) forwardLoop() {
	defer close(o.done)
	defer close(o.events)
	defer close(o.errCh)

	var (
		sessionID        = o.inner.ID()
		turns            int
		interruptions    int
		totalToolCalls   int
		totalOutputBytes int64
		errorCount       int
		termErr          error
		activeTurns      = make(map[string]*observedTurnState)
	)

	getOrStartTurn := func(turnID string, ts time.Time) *observedTurnState {
		if turnID == "" {
			return nil
		}
		st, ok := activeTurns[turnID]
		if !ok {
			if ts.IsZero() {
				ts = time.Now()
			}
			st = &observedTurnState{id: turnID, start: ts}
			activeTurns[turnID] = st
			turns++
		}
		return st
	}

	finishTurn := func(turnID string, end time.Time, interrupted bool, reason VoiceInterruptionReason, err error) {
		if turnID == "" {
			return
		}
		st, ok := activeTurns[turnID]
		if !ok {
			return
		}
		delete(activeTurns, turnID)
		if end.IsZero() {
			end = time.Now()
		}
		if err == nil {
			err = st.lastErr
		}
		o.hook.OnVoiceTurnEnd(o.sessCtx, VoiceTurnEndEvent{
			SessionID:          sessionID,
			TurnID:             turnID,
			Provider:           o.meta.Provider,
			Model:              o.meta.Model,
			Start:              st.start,
			End:                end,
			FirstAudioLatency:  st.firstAudioLatency,
			OutputAudioBytes:   st.outputAudioBytes,
			AudioChunks:        st.audioChunks,
			ToolCalls:          st.toolCalls,
			Interrupted:        interrupted,
			InterruptionReason: reason,
			Err:                err,
		})
	}

	evCh := o.inner.Events()
	errCh := o.inner.Err()

	for evCh != nil || errCh != nil {
		select {
		case evt, ok := <-evCh:
			if !ok {
				evCh = nil
				continue
			}
			if sessionID == "" && evt.SessionID != "" {
				sessionID = evt.SessionID
			}
			now := evt.Timestamp
			if now.IsZero() {
				now = time.Now()
			}

			switch evt.Type {
			case VoiceEventTurnStarted:
				getOrStartTurn(evt.TurnID, now)

			case VoiceEventAudioOutput:
				if evt.Audio != nil && len(evt.Audio.Data) > 0 {
					chunkLen := len(evt.Audio.Data)
					totalOutputBytes += int64(chunkLen)
					if st := getOrStartTurn(evt.TurnID, now); st != nil {
						st.outputAudioBytes += int64(chunkLen)
						st.audioChunks++
						if !st.sawAudio {
							st.sawAudio = true
							st.firstAudioLatency = now.Sub(st.start)
							o.hook.OnVoiceFirstChunk(o.sessCtx, VoiceFirstChunkEvent{
								Operation:    VoiceOperationVoiceTurn,
								Provider:     o.meta.Provider,
								Model:        o.meta.Model,
								SessionID:    sessionID,
								TurnID:       evt.TurnID,
								Start:        st.start,
								FirstChunkAt: now,
								ChunkBytes:   chunkLen,
							})
						}
					}
				}

			case VoiceEventToolCall:
				totalToolCalls++
				if st := getOrStartTurn(evt.TurnID, now); st != nil {
					st.toolCalls++
				}

			case VoiceEventTurnInterrupted:
				interruptions++
				target := evt.TurnID
				var reason VoiceInterruptionReason
				if evt.Interruption != nil {
					if evt.Interruption.CancelledTurnID != "" {
						target = evt.Interruption.CancelledTurnID
					}
					reason = evt.Interruption.Reason
				}
				finishTurn(target, now, true, reason, nil)

			case VoiceEventTurnCompleted:
				finishTurn(evt.TurnID, now, false, "", nil)

			case VoiceEventError:
				errorCount++
				if st := activeTurns[evt.TurnID]; st != nil && evt.Err != nil {
					st.lastErr = evt.Err
				}
			}

			// Forward event to consumer without deadlocking if Close() is invoked on a full buffer.
			select {
			case o.events <- evt:
			case <-o.closing:
			}

		case err, ok := <-errCh:
			errCh = nil
			if ok && err != nil {
				termErr = err
				errorCount++
				o.errCh <- err
			}
		}
	}

	// Flush any open turns still active when the session terminated.
	end := time.Now()
	for id := range activeTurns {
		finishTurn(id, end, termErr == nil, VoiceInterruptionSessionClosed, termErr)
	}

	if sessionID == "" {
		sessionID = o.inner.ID()
	}

	o.hook.OnVoiceSessionEnd(o.sessCtx, VoiceSessionEndEvent{
		SessionID:        sessionID,
		Provider:         o.meta.Provider,
		Mode:             o.meta.Mode,
		Start:            o.start,
		End:              end,
		Turns:            turns,
		Interruptions:    interruptions,
		ToolCalls:        totalToolCalls,
		InputAudioBytes:  o.inputAudioBytes.Load(),
		OutputAudioBytes: totalOutputBytes,
		ErrorCount:       errorCount,
		Err:              termErr,
	})
}
