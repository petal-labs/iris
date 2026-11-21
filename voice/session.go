package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/tools"
)

type turnRequest struct {
	turnID   string
	turnCtx  context.Context
	userText string
}

// Session is an active Iris-managed full-duplex voice session implementing
// [core.VoiceSession].
type Session struct {
	id            string
	pipeline      *Pipeline
	llmModel      core.ModelID
	voiceID       string
	voiceSettings *core.VoiceSettings
	language      string
	outputFormat  core.AudioFormat
	systemPrompt  string
	tools         []core.Tool
	toolIndex     map[string]core.Tool

	stt core.RealtimeTranscriptionSession

	ctx    context.Context
	cancel context.CancelFunc

	tracker *core.VoiceTurnTracker
	bus     *core.VoiceEventEmitter

	stateMu sync.RWMutex
	state   core.VoiceSessionState

	historyMu sync.RWMutex
	history   []core.Message

	turnMu      sync.Mutex
	turnTrigger chan turnRequest

	pendingMu    sync.Mutex
	pendingTools map[string]chan core.ToolResult

	wg           sync.WaitGroup
	closed       atomic.Bool
	closeOnce    sync.Once
	finalizeOnce sync.Once
	closeErr     error
}

// ID returns the session identifier.
func (s *Session) ID() string {
	return s.id
}

// State returns the current lifecycle state of the session.
func (s *Session) State() core.VoiceSessionState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *Session) setState(st core.VoiceSessionState) {
	s.stateMu.Lock()
	s.state = st
	s.stateMu.Unlock()
}

func (s *Session) transitionState(ctx context.Context, st core.VoiceSessionState, turnID string) {
	s.stateMu.Lock()
	if s.state == st || s.state == core.VoiceSessionStateClosed {
		s.stateMu.Unlock()
		return
	}
	s.state = st
	s.stateMu.Unlock()

	_ = s.bus.Emit(ctx, core.VoiceEvent{
		Type:   core.VoiceEventSessionState,
		TurnID: turnID,
		State:  st,
	})
}

// History returns a defensive copy of the committed conversation messages
// retained by the session (excluding the system prompt).
func (s *Session) History() []core.Message {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()
	out := make([]core.Message, len(s.history))
	copy(out, s.history)
	return out
}

func (s *Session) appendHistory(msgs ...core.Message) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	s.history = append(s.history, msgs...)
	maxMsgs := s.pipeline.cfg.MaxHistoryMessages
	if maxMsgs > 0 && len(s.history) > maxMsgs {
		excess := len(s.history) - maxMsgs
		// Trim oldest messages cleanly; avoid leaving an orphaned RoleTool at index 0.
		for excess < len(s.history) && s.history[excess].Role == core.RoleTool {
			excess++
		}
		if excess >= len(s.history) {
			s.history = nil
			return
		}
		trimmed := make([]core.Message, len(s.history)-excess)
		copy(trimmed, s.history[excess:])
		s.history = trimmed
	}
}

func (s *Session) buildLLMMessages() []core.Message {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	extra := 0
	if s.systemPrompt != "" {
		extra = 1
	}
	msgs := make([]core.Message, 0, len(s.history)+extra)
	if s.systemPrompt != "" {
		msgs = append(msgs, core.Message{
			Role:    core.RoleSystem,
			Content: s.systemPrompt,
		})
	}
	msgs = append(msgs, s.history...)
	return msgs
}

// SendAudio validates and transmits an incremental user audio chunk to the
// session's real-time STT recognizer. Returns [core.ErrNotSupported] if the
// pipeline was constructed without an STT recognizer.
func (s *Session) SendAudio(ctx context.Context, chunk core.AudioChunk) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if s.stt == nil {
		return fmt.Errorf("voice: session has no STT recognizer: %w", core.ErrNotSupported)
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.stt.SendAudio(ctx, chunk)
}

// CommitAudio requests finalization of the current uncommitted audio segment on
// the session's real-time STT recognizer (for manual commit mode).
func (s *Session) CommitAudio(ctx context.Context) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if s.stt == nil {
		return fmt.Errorf("voice: session has no STT recognizer: %w", core.ErrNotSupported)
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.stt.Commit(ctx)
}

// SendText injects a committed user text utterance directly into the pipeline,
// triggering an LLM reasoning and streaming TTS turn.
func (s *Session) SendText(ctx context.Context, text string) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: text cannot be empty", core.ErrInvalidSpeechRequest)
	}
	if ctx == nil {
		ctx = s.ctx
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	turnID := s.scheduleUserTurn(text, &core.TranscriptionResponse{
		Text:    text,
		IsFinal: true,
	})
	if turnID == "" {
		return core.ErrVoiceSessionClosed
	}
	return nil
}

// SubmitToolResult delivers the execution result for a pending
// [core.VoiceEventToolCall] that is not handled by an automatic tool executor.
func (s *Session) SubmitToolResult(ctx context.Context, result core.ToolResult) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if strings.TrimSpace(result.CallID) == "" {
		return fmt.Errorf("%w: tool result call_id is required", core.ErrInvalidVoiceSessionConfig)
	}
	if ctx == nil {
		ctx = s.ctx
	}

	s.pendingMu.Lock()
	ch, ok := s.pendingTools[result.CallID]
	s.pendingMu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrToolCallNotFound, result.CallID)
	}

	select {
	case ch <- result:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return core.ErrVoiceSessionClosed
	}
}

// Interrupt cancels any active assistant turn immediately, aborts in-flight
// LLM/tool/TTS streams, suppresses stale playback audio, and emits
// [core.VoiceEventTurnInterrupted].
func (s *Session) Interrupt(ctx context.Context) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	s.interruptActiveTurn(core.VoiceInterruptionClientCancel)
	return nil
}

// Events returns the bounded receive-only stream of typed [core.VoiceEvent] values.
func (s *Session) Events() <-chan core.VoiceEvent {
	return s.bus.Events()
}

// Err returns the receive-only terminal error channel.
func (s *Session) Err() <-chan error {
	return s.bus.Err()
}

// Close terminates the voice session immediately and releases all underlying
// connections and goroutines. Idempotent and safe for concurrent calls.
func (s *Session) Close() error {
	s.shutdown(nil, true)
	return s.closeErr
}

func (s *Session) shutdown(termErr error, callerInitiated bool) {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		s.tracker.Close()
		s.cancel()
		if s.stt != nil {
			s.closeErr = s.stt.Close()
		}
	})

	// Wait for worker goroutines (except when called from watchContext itself,
	// which is why watchContext calls finishShutdown without self-waiting).
	if callerInitiated {
		s.wg.Wait()
		s.finalizeBus(termErr)
	}
}

func (s *Session) finalizeBus(termErr error) {
	s.finalizeOnce.Do(func() {
		s.setState(core.VoiceSessionStateClosed)
		if termErr == nil && len(s.bus.Events()) < cap(s.bus.Events()) {
			_ = s.bus.Emit(context.Background(), core.VoiceEvent{
				Type:  core.VoiceEventSessionEnded,
				State: core.VoiceSessionStateClosed,
			})
		}
		s.bus.Fail(termErr)
	})
}

func (s *Session) watchContext() {
	defer s.wg.Done()
	<-s.ctx.Done()

	// If Close() was not explicitly called by the user, parent context cancelled.
	if !s.closed.Load() {
		err := s.ctx.Err()
		s.closed.Store(true)
		s.tracker.Close()
		if s.stt != nil {
			_ = s.stt.Close()
		}
		// Close bus immediately so any worker blocked on Emit unblocks.
		s.setState(core.VoiceSessionStateClosed)
		s.bus.Fail(err)
	}
}

func (s *Session) idleState() core.VoiceSessionState {
	if s.stt != nil {
		return core.VoiceSessionStateListening
	}
	return core.VoiceSessionStateActive
}

func (s *Session) interruptActiveTurn(reason core.VoiceInterruptionReason) {
	intr, ok := s.tracker.Interrupt(reason)
	if !ok {
		return
	}
	s.transitionState(s.ctx, core.VoiceSessionStateInterrupted, intr.CancelledTurnID)
	_ = s.bus.Emit(s.ctx, core.VoiceEvent{
		Type:         core.VoiceEventTurnInterrupted,
		TurnID:       intr.CancelledTurnID,
		Interruption: &intr,
	})
	s.transitionState(s.ctx, s.idleState(), "")
}

func (s *Session) scheduleUserTurn(userText string, transcript *core.TranscriptionResponse) string {
	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	if s.closed.Load() || s.ctx.Err() != nil {
		return ""
	}

	// If an assistant turn is currently active, interrupt it before starting the new turn.
	if intr, interrupted := s.tracker.Interrupt(core.VoiceInterruptionBargeIn); interrupted {
		_ = s.bus.Emit(s.ctx, core.VoiceEvent{
			Type:         core.VoiceEventTurnInterrupted,
			TurnID:       intr.CancelledTurnID,
			Interruption: &intr,
		})
	}

	turnID, turnCtx := s.tracker.StartTurnWithContext(s.ctx)

	_ = s.bus.Emit(s.ctx, core.VoiceEvent{
		Type:       core.VoiceEventUserTranscriptCommitted,
		TurnID:     turnID,
		Role:       core.RoleUser,
		Transcript: transcript,
		IsFinal:    true,
	})

	req := turnRequest{
		turnID:   turnID,
		turnCtx:  turnCtx,
		userText: userText,
	}

	// Non-blocking latest-wins enqueue so callers/STT loop never stall.
	select {
	case <-s.turnTrigger:
	default:
	}
	s.turnTrigger <- req
	return turnID
}

func (s *Session) sttLoop() {
	defer s.wg.Done()

	for {
		select {
		case evt, ok := <-s.stt.Events():
			if !ok {
				// Check STT error channel before concluding clean EOF.
				if err := <-s.stt.Err(); err != nil && !s.closed.Load() {
					s.closed.Store(true)
					s.tracker.Close()
					s.cancel()
					s.setState(core.VoiceSessionStateClosed)
					s.bus.Fail(err)
				}
				return
			}

			switch evt.Type {
			case core.RealtimeTranscriptEventPartial:
				if evt.Transcript == nil {
					continue
				}
				activeTurn := s.tracker.ActiveTurnID()
				if !s.pipeline.cfg.DisableBargeIn && strings.TrimSpace(evt.Transcript.Text) != "" && activeTurn != "" {
					s.interruptActiveTurn(core.VoiceInterruptionBargeIn)
				}
				_ = s.bus.Emit(s.ctx, core.VoiceEvent{
					Type:       core.VoiceEventUserTranscriptPartial,
					TurnID:     s.tracker.ActiveTurnID(),
					Role:       core.RoleUser,
					Transcript: evt.Transcript,
					IsFinal:    false,
				})

			case core.RealtimeTranscriptEventCommitted:
				if evt.Transcript == nil || strings.TrimSpace(evt.Transcript.Text) == "" {
					continue
				}
				s.scheduleUserTurn(evt.Transcript.Text, evt.Transcript)
			}

		case err, ok := <-s.stt.Err():
			if ok && err != nil && !s.closed.Load() {
				s.closed.Store(true)
				s.tracker.Close()
				s.cancel()
				s.setState(core.VoiceSessionStateClosed)
				s.bus.Fail(err)
			}
			return

		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) turnWorker() {
	defer s.wg.Done()

	for {
		select {
		case req := <-s.turnTrigger:
			s.executeTurn(req)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) executeTurn(req turnRequest) {
	turnID := req.turnID
	turnCtx := req.turnCtx

	if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
		return
	}

	_ = s.bus.Emit(turnCtx, core.VoiceEvent{
		Type:   core.VoiceEventTurnStarted,
		TurnID: turnID,
	})
	s.transitionState(turnCtx, core.VoiceSessionStateThinking, turnID)

	// Record user utterance in conversation history.
	s.appendHistory(core.Message{
		Role:    core.RoleUser,
		Content: req.userText,
	})

	maxRounds := s.pipeline.cfg.MaxToolRounds
	for round := 0; round <= maxRounds; round++ {
		if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
			return
		}

		resp, streamedAudio, err := s.runLLMRound(turnCtx, turnID)
		if err != nil {
			if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
				return
			}
			s.emitTurnError(turnID, err)
			s.tracker.CompleteTurn(turnID)
			s.transitionState(s.ctx, s.idleState(), "")
			return
		}

		// Record assistant response in history (text and/or tool calls).
		if resp.Output != "" || len(resp.ToolCalls) > 0 {
			s.appendHistory(core.Message{
				Role:      core.RoleAssistant,
				Content:   resp.Output,
				ToolCalls: resp.ToolCalls,
			})
		}

		if len(resp.ToolCalls) == 0 {
			// Final assistant reply for this turn.
			if resp.Output != "" {
				_ = s.bus.Emit(turnCtx, core.VoiceEvent{
					Type:    core.VoiceEventAgentTextCommitted,
					TurnID:  turnID,
					Role:    core.RoleAssistant,
					Text:    resp.Output,
					IsFinal: true,
				})
			}

			// If we didn't stream audio incrementally during LLM generation (e.g. HTTP TTS fallback),
			// synthesize the full response now.
			if !streamedAudio && strings.TrimSpace(resp.Output) != "" {
				if err := s.synthesizeFallback(turnCtx, turnID, resp.Output); err != nil {
					if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
						return
					}
					s.emitTurnError(turnID, err)
					s.tracker.CompleteTurn(turnID)
					s.transitionState(s.ctx, s.idleState(), "")
					return
				}
			}

			if s.tracker.CompleteTurn(turnID) {
				_ = s.bus.Emit(s.ctx, core.VoiceEvent{
					Type:   core.VoiceEventTurnCompleted,
					TurnID: turnID,
				})
				s.transitionState(s.ctx, s.idleState(), "")
			}
			return
		}

		if round == maxRounds {
			s.emitTurnError(turnID, fmt.Errorf("%w (%d)", ErrMaxToolRoundsExceeded, maxRounds))
			s.tracker.CompleteTurn(turnID)
			s.transitionState(s.ctx, s.idleState(), "")
			return
		}

		// Execute or await results for all requested tool calls.
		toolResults, err := s.handleToolCalls(turnCtx, turnID, resp.ToolCalls)
		if err != nil {
			if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
				return
			}
			s.emitTurnError(turnID, err)
			s.tracker.CompleteTurn(turnID)
			s.transitionState(s.ctx, s.idleState(), "")
			return
		}

		s.appendHistory(core.Message{
			Role:        core.RoleTool,
			ToolResults: toolResults,
		})
	}
}

func (s *Session) runLLMRound(turnCtx context.Context, turnID string) (*core.ChatResponse, bool, error) {
	chatReq := &core.ChatRequest{
		Model:       s.llmModel,
		Messages:    s.buildLLMMessages(),
		Temperature: s.pipeline.cfg.Temperature,
		MaxTokens:   s.pipeline.cfg.MaxTokens,
		Tools:       s.tools,
	}

	llm := s.pipeline.cfg.LLM
	if !llm.Supports(core.FeatureChatStreaming) {
		resp, err := llm.Chat(turnCtx, chatReq)
		if err != nil {
			return nil, false, err
		}
		if resp == nil {
			return &core.ChatResponse{}, false, nil
		}
		if resp.Output != "" {
			_ = s.bus.Emit(turnCtx, core.VoiceEvent{
				Type:    core.VoiceEventAgentTextDelta,
				TurnID:  turnID,
				Role:    core.RoleAssistant,
				Text:    resp.Output,
				IsFinal: false,
			})
		}
		return resp, false, nil
	}

	stream, err := llm.StreamChat(turnCtx, chatReq)
	if err != nil {
		return nil, false, err
	}
	if stream == nil {
		return nil, false, core.ErrNilStream
	}

	var (
		textBuf    strings.Builder
		finalResp  *core.ChatResponse
		streamErr  error
		ttsSess    core.RealtimeSynthesisSession
		ttsDone    chan error
		startedTTS bool
		chClosed   = stream.Ch == nil
		errDone    = stream.Err == nil
		finalDone  = stream.Final == nil
	)

	closeTTS := func() {
		if ttsSess != nil {
			_ = ttsSess.Close()
			if ttsDone != nil {
				<-ttsDone
				ttsDone = nil
			}
			ttsSess = nil
		}
	}
	defer closeTTS()

	for !chClosed || !errDone || !finalDone {
		var chRecv <-chan core.ChatChunk
		if !chClosed {
			chRecv = stream.Ch
		}
		var errRecv <-chan error
		if !errDone {
			errRecv = stream.Err
		}
		var finalRecv <-chan *core.ChatResponse
		if !finalDone {
			finalRecv = stream.Final
		}

		select {
		case <-turnCtx.Done():
			closeTTS()
			return nil, startedTTS, turnCtx.Err()

		case chunk, ok := <-chRecv:
			if !ok {
				chClosed = true
				continue
			}
			if chunk.Delta == "" {
				continue
			}
			textBuf.WriteString(chunk.Delta)
			_ = s.bus.Emit(turnCtx, core.VoiceEvent{
				Type:    core.VoiceEventAgentTextDelta,
				TurnID:  turnID,
				Role:    core.RoleAssistant,
				Text:    chunk.Delta,
				IsFinal: false,
			})

			if s.pipeline.cfg.RealtimeTTS != nil {
				if !startedTTS {
					var startErr error
					ttsSess, ttsDone, startErr = s.startRealtimeTTS(turnCtx, turnID)
					if startErr != nil {
						return nil, false, startErr
					}
					startedTTS = true
				}
				if sendErr := ttsSess.SendText(turnCtx, chunk.Delta); sendErr != nil {
					return nil, startedTTS, sendErr
				}
			}

		case err, ok := <-errRecv:
			errDone = true
			if ok && err != nil {
				streamErr = err
			}

		case resp, ok := <-finalRecv:
			finalDone = true
			if ok && resp != nil {
				finalResp = resp
			}
		}
	}

	if streamErr != nil {
		return nil, startedTTS, streamErr
	}

	if finalResp == nil {
		finalResp = &core.ChatResponse{Output: textBuf.String()}
	} else if finalResp.Output == "" {
		finalResp.Output = textBuf.String()
	}

	if ttsSess != nil {
		if err := ttsSess.Finish(turnCtx); err != nil && !errors.Is(err, core.ErrRealtimeSessionClosed) {
			return nil, startedTTS, err
		}
		ttsErr := <-ttsDone
		ttsDone = nil
		_ = ttsSess.Close()
		ttsSess = nil
		if ttsErr != nil {
			return nil, startedTTS, ttsErr
		}
	}

	return finalResp, startedTTS, nil
}

func (s *Session) startRealtimeTTS(turnCtx context.Context, turnID string) (core.RealtimeSynthesisSession, chan error, error) {
	ttsSess, err := s.pipeline.cfg.RealtimeTTS.StartRealtimeSynthesis(turnCtx, core.RealtimeSynthesisConfig{
		Model:               s.pipeline.cfg.TTSModel,
		VoiceID:             s.voiceID,
		Format:              s.outputFormat,
		Language:            s.language,
		VoiceSettings:       s.voiceSettings,
		ChunkLengthSchedule: s.pipeline.cfg.ChunkLengthSchedule,
		AutoMode:            s.pipeline.cfg.AutoMode,
		BufferSize:          s.pipeline.cfg.EventBufferSize,
	})
	if err != nil {
		return nil, nil, err
	}

	s.transitionState(turnCtx, core.VoiceSessionStateSpeaking, turnID)

	done := make(chan error, 1)
	go func() {
		var firstErr error
		for chunk := range ttsSess.Audio() {
			if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
				continue
			}
			if len(chunk.Data) > 0 {
				cp := chunk
				_ = s.bus.Emit(turnCtx, core.VoiceEvent{
					Type:   core.VoiceEventAudioOutput,
					TurnID: turnID,
					Role:   core.RoleAssistant,
					Audio:  &cp,
				})
			}
		}
		if err := <-ttsSess.Err(); err != nil {
			firstErr = err
		}
		if firstErr == nil && turnCtx.Err() == nil && s.tracker.IsCurrentTurn(turnID) {
			_ = s.bus.Emit(turnCtx, core.VoiceEvent{
				Type:   core.VoiceEventAudioOutputEnded,
				TurnID: turnID,
				Role:   core.RoleAssistant,
			})
		}
		done <- firstErr
		close(done)
	}()

	return ttsSess, done, nil
}

func (s *Session) synthesizeFallback(turnCtx context.Context, turnID, text string) error {
	if s.pipeline.cfg.RealtimeTTS != nil {
		ttsSess, done, err := s.startRealtimeTTS(turnCtx, turnID)
		if err != nil {
			return err
		}
		defer func() { _ = ttsSess.Close() }()
		if err := ttsSess.SendText(turnCtx, text); err != nil {
			_ = ttsSess.Close()
			<-done
			return err
		}
		if err := ttsSess.Finish(turnCtx); err != nil {
			_ = ttsSess.Close()
			<-done
			return err
		}
		return <-done
	}

	if s.pipeline.cfg.TTS == nil {
		return nil
	}

	stream, err := s.pipeline.cfg.TTS.StreamSynthesize(turnCtx, &core.SpeechRequest{
		Model:         s.pipeline.cfg.TTSModel,
		VoiceID:       s.voiceID,
		Text:          text,
		Format:        s.outputFormat,
		Language:      s.language,
		VoiceSettings: s.voiceSettings,
	})
	if err != nil {
		return err
	}
	if stream == nil {
		return core.ErrNilSpeechStream
	}

	s.transitionState(turnCtx, core.VoiceSessionStateSpeaking, turnID)

	for chunk := range stream.Ch {
		if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
			continue
		}
		if len(chunk.Data) > 0 {
			cp := chunk
			_ = s.bus.Emit(turnCtx, core.VoiceEvent{
				Type:   core.VoiceEventAudioOutput,
				TurnID: turnID,
				Role:   core.RoleAssistant,
				Audio:  &cp,
			})
		}
	}
	if err := <-stream.Err; err != nil {
		return err
	}
	// Drain final channel cleanly.
	if stream.Final != nil {
		<-stream.Final
	}

	if turnCtx.Err() == nil && s.tracker.IsCurrentTurn(turnID) {
		_ = s.bus.Emit(turnCtx, core.VoiceEvent{
			Type:   core.VoiceEventAudioOutputEnded,
			TurnID: turnID,
			Role:   core.RoleAssistant,
		})
	}
	return nil
}

func (s *Session) handleToolCalls(turnCtx context.Context, turnID string, calls []core.ToolCall) ([]core.ToolResult, error) {
	results := make([]core.ToolResult, 0, len(calls))
	for _, call := range calls {
		if turnCtx.Err() != nil || !s.tracker.IsCurrentTurn(turnID) {
			return nil, turnCtx.Err()
		}

		// Register external waiter BEFORE emitting VoiceEventToolCall so an
		// immediate caller SubmitToolResult can never race with registration.
		var waitCh chan core.ToolResult
		if !s.isAutoExecutableTool(call.Name) {
			waitCh = make(chan core.ToolResult, 1)
			s.pendingMu.Lock()
			s.pendingTools[call.ID] = waitCh
			s.pendingMu.Unlock()
		}

		callCopy := call
		_ = s.bus.Emit(turnCtx, core.VoiceEvent{
			Type:     core.VoiceEventToolCall,
			TurnID:   turnID,
			Role:     core.RoleAssistant,
			ToolCall: &callCopy,
		})

		res, err := s.resolveToolCall(turnCtx, call, waitCh)
		if err != nil {
			return nil, err
		}

		resCopy := res
		_ = s.bus.Emit(turnCtx, core.VoiceEvent{
			Type:       core.VoiceEventToolResult,
			TurnID:     turnID,
			Role:       core.RoleTool,
			ToolResult: &resCopy,
		})
		results = append(results, res)
	}
	return results, nil
}

func (s *Session) isAutoExecutableTool(name string) bool {
	if s.pipeline.cfg.ToolExecutor != nil {
		return true
	}
	if t, ok := s.toolIndex[name]; ok {
		_, isExec := t.(tools.Tool)
		return isExec
	}
	return false
}

func (s *Session) resolveToolCall(turnCtx context.Context, call core.ToolCall, waitCh chan core.ToolResult) (core.ToolResult, error) {
	if waitCh != nil {
		defer func() {
			s.pendingMu.Lock()
			delete(s.pendingTools, call.ID)
			s.pendingMu.Unlock()
		}()

		select {
		case res := <-waitCh:
			return res, nil
		case <-turnCtx.Done():
			return core.ToolResult{}, turnCtx.Err()
		}
	}

	execCtx := turnCtx
	var cancel context.CancelFunc
	if s.pipeline.cfg.ToolTimeout > 0 {
		execCtx, cancel = context.WithTimeout(turnCtx, s.pipeline.cfg.ToolTimeout)
		defer cancel()
	}

	// 1. Explicit ToolExecutor takes precedence.
	if s.pipeline.cfg.ToolExecutor != nil {
		out, err := s.pipeline.cfg.ToolExecutor.Execute(execCtx, call)
		if turnCtx.Err() != nil {
			return core.ToolResult{}, turnCtx.Err()
		}
		if err != nil {
			return core.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}, nil
		}
		return core.ToolResult{CallID: call.ID, Content: out, IsError: false}, nil
	}

	// 2. Registered executable tools.Tool executes directly.
	if t, ok := s.toolIndex[call.Name]; ok {
		if execTool, isExec := t.(tools.Tool); isExec {
			out, err := execTool.Call(execCtx, call.Arguments)
			if turnCtx.Err() != nil {
				return core.ToolResult{}, turnCtx.Err()
			}
			if err != nil {
				return core.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}, nil
			}
			return core.ToolResult{CallID: call.ID, Content: out, IsError: false}, nil
		}
	}

	return core.ToolResult{CallID: call.ID, Content: "tool not found", IsError: true}, nil
}

func (s *Session) emitTurnError(turnID string, err error) {
	_ = s.bus.Emit(s.ctx, core.VoiceEvent{
		Type:         core.VoiceEventError,
		TurnID:       turnID,
		Err:          err,
		ErrorMessage: err.Error(),
	})
}

// Compile-time check that Session implements core.VoiceSession.
var _ core.VoiceSession = (*Session)(nil)
