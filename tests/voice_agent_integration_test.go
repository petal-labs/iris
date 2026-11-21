package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
	"github.com/petal-labs/iris/tools"
	"github.com/petal-labs/iris/voice"
)

// --- Deterministic voice-agent integration test doubles ---

type mockRealtimeSTT struct {
	mu   sync.Mutex
	sess *mockRealtimeSTTSession
}

func (m *mockRealtimeSTT) StartRealtimeTranscription(_ context.Context, _ core.RealtimeTranscriptionConfig) (core.RealtimeTranscriptionSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sess == nil {
		m.sess = newMockRealtimeSTTSession()
	}
	return m.sess, nil
}

type mockRealtimeSTTSession struct {
	events    chan core.RealtimeTranscriptEvent
	errCh     chan error
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
	chunks    int
	commits   int
}

func newMockRealtimeSTTSession() *mockRealtimeSTTSession {
	return &mockRealtimeSTTSession{
		events: make(chan core.RealtimeTranscriptEvent, 64),
		errCh:  make(chan error, 1),
	}
}

func (s *mockRealtimeSTTSession) SendAudio(_ context.Context, _ core.AudioChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrRealtimeSessionClosed
	}
	s.chunks++
	return nil
}

func (s *mockRealtimeSTTSession) Commit(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrRealtimeSessionClosed
	}
	s.commits++
	return nil
}

func (s *mockRealtimeSTTSession) Events() <-chan core.RealtimeTranscriptEvent { return s.events }
func (s *mockRealtimeSTTSession) Err() <-chan error                           { return s.errCh }

func (s *mockRealtimeSTTSession) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.events)
		close(s.errCh)
	})
	return nil
}

func (s *mockRealtimeSTTSession) fail(err error) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.errCh <- err
		close(s.events)
		close(s.errCh)
	})
}

type mockRealtimeTTS struct {
	failStart error
	mu        sync.Mutex
	sessions  []*mockRealtimeTTSSession
	onStart   func(ctx context.Context, cfg core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error)
}

func (m *mockRealtimeTTS) StartRealtimeSynthesis(ctx context.Context, cfg core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error) {
	if m.failStart != nil {
		return nil, m.failStart
	}
	if m.onStart != nil {
		return m.onStart(ctx, cfg)
	}
	s := newMockRealtimeTTSSession(cfg.Format)
	m.mu.Lock()
	m.sessions = append(m.sessions, s)
	m.mu.Unlock()
	return s, nil
}

type mockRealtimeTTSSession struct {
	format    core.AudioFormat
	audio     chan core.AudioChunk
	errCh     chan error
	mu        sync.Mutex
	seq       int64
	closed    bool
	closeOnce sync.Once
}

func newMockRealtimeTTSSession(format core.AudioFormat) *mockRealtimeTTSSession {
	if format.IsZero() {
		format = core.AudioFormatPCM16000
	}
	return &mockRealtimeTTSSession{
		format: format,
		audio:  make(chan core.AudioChunk, 64),
		errCh:  make(chan error, 1),
	}
}

func (s *mockRealtimeTTSSession) SendText(_ context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrRealtimeSessionClosed
	}
	s.audio <- core.AudioChunk{
		Data:     []byte("pcm[" + text + "]"),
		Format:   s.format,
		Sequence: s.seq,
	}
	s.seq++
	return nil
}

func (s *mockRealtimeTTSSession) Flush(_ context.Context) error { return nil }

func (s *mockRealtimeTTSSession) Finish(_ context.Context) error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.audio <- core.AudioChunk{Format: s.format, Sequence: s.seq, IsFinal: true}
		s.mu.Unlock()
		close(s.audio)
		close(s.errCh)
	})
	return nil
}

func (s *mockRealtimeTTSSession) Audio() <-chan core.AudioChunk { return s.audio }
func (s *mockRealtimeTTSSession) Err() <-chan error             { return s.errCh }

func (s *mockRealtimeTTSSession) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.audio)
		close(s.errCh)
	})
	return nil
}

type mockHTTPTTS struct{}

func (mockHTTPTTS) Synthesize(_ context.Context, _ *core.SpeechRequest) (*core.SpeechResponse, error) {
	return nil, core.ErrNotSupported
}

func (mockHTTPTTS) StreamSynthesize(_ context.Context, req *core.SpeechRequest) (*core.SpeechStream, error) {
	ch := make(chan core.AudioChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.SpeechResponse, 1)
	ch <- core.AudioChunk{Data: []byte("http[" + req.Text + "]"), Format: core.AudioFormatPCM16000, Sequence: 0}
	ch <- core.AudioChunk{Format: core.AudioFormatPCM16000, Sequence: 1, IsFinal: true}
	close(ch)
	finalCh <- &core.SpeechResponse{Model: req.Model, VoiceID: req.VoiceID}
	close(finalCh)
	close(errCh)
	return &core.SpeechStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

type mockVoiceLLM struct {
	id        string
	streaming bool
	chatFn    func(context.Context, *core.ChatRequest) (*core.ChatResponse, error)
	streamFn  func(context.Context, *core.ChatRequest) (*core.ChatStream, error)
}

func (m *mockVoiceLLM) ID() string               { return m.id }
func (m *mockVoiceLLM) Models() []core.ModelInfo { return nil }
func (m *mockVoiceLLM) Supports(f core.Feature) bool {
	if f == core.FeatureChat {
		return true
	}
	return f == core.FeatureChatStreaming && m.streaming
}

func (m *mockVoiceLLM) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, req)
	}
	return &core.ChatResponse{Output: "reply from " + m.id}, nil
}

func (m *mockVoiceLLM) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, req)
	}
	ch := make(chan core.ChatChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)
	ch <- core.ChatChunk{Delta: "reply from "}
	ch <- core.ChatChunk{Delta: m.id}
	close(ch)
	finalCh <- &core.ChatResponse{Output: "reply from " + m.id}
	close(finalCh)
	close(errCh)
	return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

type mockExecTool struct {
	name string
	fn   func(context.Context, json.RawMessage) (any, error)
}

func (t mockExecTool) Name() string            { return t.name }
func (t mockExecTool) Description() string     { return "integration test tool" }
func (t mockExecTool) Schema() core.ToolSchema { return core.ToolSchema{} }
func (t mockExecTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return t.fn(ctx, args)
}

type mockSchemaTool struct{ name string }

func (t mockSchemaTool) Name() string            { return t.name }
func (t mockSchemaTool) Description() string     { return "external schema tool" }
func (t mockSchemaTool) Schema() core.ToolSchema { return core.ToolSchema{} }

type voiceMetricsCapture struct {
	core.NoopVoiceTelemetryHook
	mu          sync.Mutex
	firstChunks []core.VoiceFirstChunkEvent
	turnEnds    []core.VoiceTurnEndEvent
	sessEnds    []core.VoiceSessionEndEvent
}

func (c *voiceMetricsCapture) OnVoiceFirstChunk(_ context.Context, e core.VoiceFirstChunkEvent) {
	c.mu.Lock()
	c.firstChunks = append(c.firstChunks, e)
	c.mu.Unlock()
}

func (c *voiceMetricsCapture) OnVoiceTurnEnd(_ context.Context, e core.VoiceTurnEndEvent) {
	c.mu.Lock()
	c.turnEnds = append(c.turnEnds, e)
	c.mu.Unlock()
}

func (c *voiceMetricsCapture) OnVoiceSessionEnd(_ context.Context, e core.VoiceSessionEndEvent) {
	c.mu.Lock()
	c.sessEnds = append(c.sessEnds, e)
	c.mu.Unlock()
}

func collectVoiceEventsUntil(t *testing.T, ch <-chan core.VoiceEvent, pred func(core.VoiceEvent) bool) []core.VoiceEvent {
	t.Helper()
	var out []core.VoiceEvent
	deadline := time.After(3 * time.Second)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				t.Fatalf("voice events closed early after %d events", len(out))
			}
			out = append(out, evt)
			if pred(evt) {
				return out
			}
		case <-deadline:
			t.Fatalf("timed out waiting for expected voice event; collected %d events", len(out))
		}
	}
}

// --- End-to-End Integration Tests ---

func TestVoiceAgent_EndToEndMultiTurnConversationAndTranscriptOrder(t *testing.T) {
	stt := &mockRealtimeSTT{sess: newMockRealtimeSTTSession()}
	var turnCount atomic.Int32
	llm := &mockVoiceLLM{
		id:        "openai",
		streaming: true,
		streamFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
			n := turnCount.Add(1)
			ch := make(chan core.ChatChunk, 3)
			errCh := make(chan error, 1)
			finalCh := make(chan *core.ChatResponse, 1)
			if n == 1 {
				ch <- core.ChatChunk{Delta: "Hello "}
				ch <- core.ChatChunk{Delta: "world."}
				finalCh <- &core.ChatResponse{Output: "Hello world."}
			} else {
				// Confirm prior committed turn is present in LLM context (and partials are NOT).
				if len(req.Messages) < 3 {
					errCh <- fmt.Errorf("expected conversation history >= 3 msgs, got %d", len(req.Messages))
				} else {
					ch <- core.ChatChunk{Delta: "Second "}
					ch <- core.ChatChunk{Delta: "reply."}
					finalCh <- &core.ChatResponse{Output: "Second reply."}
				}
			}
			close(ch)
			close(finalCh)
			close(errCh)
			return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
		},
	}

	metrics := &voiceMetricsCapture{}
	pipeline, err := voice.NewPipeline(voice.PipelineConfig{
		STT:          stt,
		LLM:          llm,
		LLMModel:     "gpt-5",
		RealtimeTTS:  &mockRealtimeTTS{},
		VoiceID:      "voice-e2e",
		SystemPrompt: "SECRET_SYSTEM_INSTRUCTIONS",
		Telemetry:    metrics,
	})
	if err != nil {
		t.Fatalf("NewPipeline() err = %v", err)
	}

	sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{
		InputFormat:  core.AudioFormatPCM16000,
		OutputFormat: core.AudioFormatPCM16000,
	})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}
	defer sess.Close()

	// Turn 1: send audio + partial -> partial -> committed transcripts.
	if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2, 3, 4}, Format: core.AudioFormatPCM16000}); err != nil {
		t.Fatalf("SendAudio() err = %v", err)
	}
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventPartial,
		IsFinal:    false,
		Transcript: &core.TranscriptionResponse{Text: "SECRET_PARTIAL_1", IsFinal: false},
	}
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventCommitted,
		IsFinal:    true,
		Transcript: &core.TranscriptionResponse{Text: "SECRET_USER_TURN_1", IsFinal: true},
	}

	turn1Events := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})

	// Turn 2 via committed STT transcript.
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventCommitted,
		IsFinal:    true,
		Transcript: &core.TranscriptionResponse{Text: "Follow up question", IsFinal: true},
	}
	turn2Events := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})

	allEvents := append(turn1Events, turn2Events...)
	tracker := core.NewVoiceTurnTracker("consumer")

	var prevSeq int64
	var partialIdx, committedIdx, audioIdx, completedIdx = -1, -1, -1, -1
	var playableChunks int
	for i, evt := range allEvents {
		tracker.ObserveEvent(evt)
		if evt.Sequence <= prevSeq {
			t.Errorf("non-monotonic event sequence at index %d: %d <= %d", i, evt.Sequence, prevSeq)
		}
		prevSeq = evt.Sequence

		// Privacy gate: formatted events must never leak transcripts, prompts, or audio bytes.
		formatted := evt.String() + " " + fmt.Sprintf("%#v", evt)
		for _, forbidden := range []string{"SECRET_PARTIAL_1", "SECRET_USER_TURN_1", "SECRET_SYSTEM_INSTRUCTIONS", "pcm[Hello ]"} {
			if strings.Contains(formatted, forbidden) {
				t.Errorf("event[%d] leaked %q: %s", i, forbidden, formatted)
			}
		}

		if i < len(turn1Events) {
			switch evt.Type {
			case core.VoiceEventUserTranscriptPartial:
				partialIdx = i
			case core.VoiceEventUserTranscriptCommitted:
				committedIdx = i
			case core.VoiceEventAudioOutput:
				if audioIdx == -1 {
					audioIdx = i
				}
				if tracker.ShouldPlayAudio(evt) {
					playableChunks++
				}
			case core.VoiceEventTurnCompleted:
				completedIdx = i
			}
		} else if evt.Type == core.VoiceEventAudioOutput && tracker.ShouldPlayAudio(evt) {
			playableChunks++
		}
	}

	// Verify strict transcript & turn ordering: partial < committed < audio < completed.
	if partialIdx < 0 || partialIdx >= committedIdx || committedIdx >= audioIdx || audioIdx >= completedIdx {
		t.Fatalf("ordering violated: partial=%d committed=%d audio=%d completed=%d",
			partialIdx, committedIdx, audioIdx, completedIdx)
	}
	if playableChunks != 4 {
		t.Errorf("playable audio chunks across 2 turns = %d, want 4", playableChunks)
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("Close() err = %v", err)
	}
	if err := <-sess.Err(); err != nil {
		t.Fatalf("Err() after clean Close = %v", err)
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.firstChunks) != 2 || len(metrics.turnEnds) != 2 || len(metrics.sessEnds) != 1 {
		t.Fatalf("metrics mismatch: firstChunks=%d turnEnds=%d sessEnds=%d",
			len(metrics.firstChunks), len(metrics.turnEnds), len(metrics.sessEnds))
	}
	if metrics.sessEnds[0].Turns != 2 || metrics.sessEnds[0].InputAudioBytes != 4 || metrics.sessEnds[0].OutputAudioBytes == 0 {
		t.Errorf("unexpected session metrics: %+v", metrics.sessEnds[0])
	}
}

func TestVoiceAgent_InterruptionBargeInAndStaleAudioSuppression(t *testing.T) {
	stt := &mockRealtimeSTT{sess: newMockRealtimeSTTSession()}
	turn1Active := make(chan struct{}, 1)
	turn1Aborted := make(chan struct{}, 1)
	var turnNum atomic.Int32

	llm := &mockVoiceLLM{
		id:        "openai",
		streaming: true,
		streamFn: func(ctx context.Context, _ *core.ChatRequest) (*core.ChatStream, error) {
			n := turnNum.Add(1)
			ch := make(chan core.ChatChunk, 4)
			errCh := make(chan error, 1)
			finalCh := make(chan *core.ChatResponse, 1)
			go func() {
				defer close(ch)
				defer close(errCh)
				defer close(finalCh)
				if n == 1 {
					ch <- core.ChatChunk{Delta: "Interrupted story... "}
					turn1Active <- struct{}{}
					<-ctx.Done()
					turn1Aborted <- struct{}{}
					errCh <- ctx.Err()
					return
				}
				ch <- core.ChatChunk{Delta: "Concise answer."}
				finalCh <- &core.ChatResponse{Output: "Concise answer."}
			}()
			return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
		},
	}

	pipeline, err := voice.NewPipeline(voice.PipelineConfig{
		STT:         stt,
		LLM:         llm,
		LLMModel:    "gpt-5",
		RealtimeTTS: &mockRealtimeTTS{},
		VoiceID:     "v1",
	})
	if err != nil {
		t.Fatalf("NewPipeline() err = %v", err)
	}

	sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}
	defer sess.Close()

	if err := sess.SendText(context.Background(), "Tell me a very long story"); err != nil {
		t.Fatalf("SendText(1) err = %v", err)
	}
	<-turn1Active

	// User barges in mid-turn.
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventPartial,
		IsFinal:    false,
		Transcript: &core.TranscriptionResponse{Text: "Hold on", IsFinal: false},
	}

	select {
	case <-turn1Aborted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for barge-in to cancel turn 1 LLM context")
	}

	tracker := core.NewVoiceTurnTracker("playback")
	evts1 := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnInterrupted
	})
	var cancelledTurnID string
	for _, e := range evts1 {
		tracker.ObserveEvent(e)
		if e.Type == core.VoiceEventTurnInterrupted {
			cancelledTurnID = e.Interruption.CancelledTurnID
		}
	}
	if cancelledTurnID == "" {
		t.Fatal("missing CancelledTurnID on interruption")
	}

	// Simulate a late/stale audio chunk arriving for the cancelled turn: tracker MUST drop it.
	if tracker.ShouldPlayAudio(core.VoiceEvent{
		Type:   core.VoiceEventAudioOutput,
		TurnID: cancelledTurnID,
		Audio:  &core.AudioChunk{Data: []byte("stale-pcm")},
	}) {
		t.Fatal("stale audio chunk from cancelled turn must be suppressed")
	}

	// New turn completes cleanly after interruption.
	if err := sess.SendText(context.Background(), "Just give the summary"); err != nil {
		t.Fatalf("SendText(2) err = %v", err)
	}
	evts2 := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})
	var playedNewAudio bytes.Buffer
	for _, e := range evts2 {
		tracker.ObserveEvent(e)
		if e.Type == core.VoiceEventAudioOutput {
			if e.TurnID == cancelledTurnID {
				t.Errorf("stale audio from cancelled turn %q leaked into turn 2", cancelledTurnID)
			}
			if tracker.ShouldPlayAudio(e) {
				playedNewAudio.Write(e.Audio.Data)
			}
		}
	}
	if playedNewAudio.String() != "pcm[Concise answer.]" {
		t.Errorf("turn 2 audio = %q, want pcm[Concise answer.]", playedNewAudio.String())
	}
}

func TestVoiceAgent_ToolExecutionAutoAndExternalAndInterruptMidTool(t *testing.T) {
	t.Run("registry middleware tool + error recovery", func(t *testing.T) {
		var middlewareHits atomic.Int32
		reg := tools.NewRegistry(tools.WithRegistryMiddleware(func(next tools.ToolCallFunc) tools.ToolCallFunc {
			return func(ctx context.Context, args json.RawMessage) (any, error) {
				middlewareHits.Add(1)
				return next(ctx, args)
			}
		}))
		_ = reg.Register(mockExecTool{
			name: "get_order",
			fn: func(_ context.Context, _ json.RawMessage) (any, error) {
				return map[string]string{"state": "delivered"}, nil
			},
		})

		var round atomic.Int32
		llm := &mockVoiceLLM{
			id:        "anthropic",
			streaming: true,
			streamFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatStream, error) {
				ch := make(chan core.ChatChunk, 2)
				errCh := make(chan error, 1)
				finalCh := make(chan *core.ChatResponse, 1)
				if round.Add(1) == 1 {
					finalCh <- &core.ChatResponse{
						ToolCalls: []core.ToolCall{
							{ID: "c1", Name: "get_order", Arguments: json.RawMessage(`{"id":1}`)},
							{ID: "c2", Name: "missing_tool", Arguments: json.RawMessage(`{}`)},
						},
					}
				} else {
					ch <- core.ChatChunk{Delta: "Delivered."}
					finalCh <- &core.ChatResponse{Output: "Delivered."}
				}
				close(ch)
				close(finalCh)
				close(errCh)
				return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		regList := reg.List()
		coreTools := make([]core.Tool, len(regList))
		for i, rt := range regList {
			coreTools[i] = rt
		}

		pipeline, err := voice.NewPipeline(voice.PipelineConfig{
			LLM:          llm,
			LLMModel:     "claude-sonnet",
			RealtimeTTS:  &mockRealtimeTTS{},
			VoiceID:      "v1",
			Tools:        coreTools,
			ToolExecutor: voice.RegistryToolExecutor(reg),
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}

		sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		_ = sess.SendText(context.Background(), "Check order 1")
		evts := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventTurnCompleted
		})

		var okRes, errRes bool
		for _, e := range evts {
			if e.Type == core.VoiceEventToolResult {
				if e.ToolResult.CallID == "c1" && !e.ToolResult.IsError {
					okRes = true
				}
				if e.ToolResult.CallID == "c2" && e.ToolResult.IsError {
					errRes = true
				}
			}
		}
		if !okRes || !errRes || middlewareHits.Load() != 1 {
			t.Errorf("tool execution mismatch: okRes=%t errRes=%t middlewareHits=%d", okRes, errRes, middlewareHits.Load())
		}
	})

	t.Run("interrupt unblocks pending external SubmitToolResult cleanly", func(t *testing.T) {
		llm := &mockVoiceLLM{
			id:        "gemini",
			streaming: false,
			chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
				return &core.ChatResponse{
					ToolCalls: []core.ToolCall{{ID: "ext-wait", Name: "slow_external", Arguments: json.RawMessage(`{}`)}},
				}, nil
			},
		}
		pipeline, err := voice.NewPipeline(voice.PipelineConfig{
			LLM:         llm,
			LLMModel:    "gemini-2.5",
			RealtimeTTS: &mockRealtimeTTS{},
			VoiceID:     "v1",
			Tools:       []core.Tool{mockSchemaTool{name: "slow_external"}},
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}

		sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		_ = sess.SendText(context.Background(), "Run slow tool")
		collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventToolCall && e.ToolCall.ID == "ext-wait"
		})

		// Interrupt while blocked waiting for SubmitToolResult.
		if err := sess.Interrupt(context.Background()); err != nil {
			t.Fatalf("Interrupt() err = %v", err)
		}
		collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventTurnInterrupted
		})
	})
}

func TestVoiceAgent_ProviderSwitching(t *testing.T) {
	// Verify seamless switching across LLM providers (streaming & non-streaming)
	// and TTS transports (Realtime WS TTS vs HTTP streaming TTS).
	combos := []struct {
		name        string
		llm         core.Provider
		realtimeTTS core.RealtimeSpeechSynthesizer
		httpTTS     core.SpeechSynthesizer
		wantAudio   string
	}{
		{
			name:        "openai_streaming_plus_ws_tts",
			llm:         &mockVoiceLLM{id: "openai", streaming: true},
			realtimeTTS: &mockRealtimeTTS{},
			wantAudio:   "pcm[reply from ]pcm[openai]",
		},
		{
			name:      "anthropic_sync_plus_http_tts",
			llm:       &mockVoiceLLM{id: "anthropic", streaming: false},
			httpTTS:   mockHTTPTTS{},
			wantAudio: "http[reply from anthropic]",
		},
		{
			name:        "gemini_streaming_plus_ws_tts",
			llm:         &mockVoiceLLM{id: "gemini", streaming: true},
			realtimeTTS: &mockRealtimeTTS{},
			wantAudio:   "pcm[reply from ]pcm[gemini]",
		},
	}

	for _, tc := range combos {
		t.Run(tc.name, func(t *testing.T) {
			pipeline, err := voice.NewPipeline(voice.PipelineConfig{
				LLM:         tc.llm,
				LLMModel:    "test-model",
				RealtimeTTS: tc.realtimeTTS,
				TTS:         tc.httpTTS,
				VoiceID:     "v1",
			})
			if err != nil {
				t.Fatalf("NewPipeline() err = %v", err)
			}

			sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
			if err != nil {
				t.Fatalf("StartVoiceSession() err = %v", err)
			}
			defer sess.Close()

			if err := sess.SendText(context.Background(), "Ping"); err != nil {
				t.Fatalf("SendText() err = %v", err)
			}
			evts := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
				return e.Type == core.VoiceEventTurnCompleted
			})

			var audio bytes.Buffer
			for _, e := range evts {
				if e.Type == core.VoiceEventAudioOutput {
					audio.Write(e.Audio.Data)
				}
			}
			if audio.String() != tc.wantAudio {
				t.Errorf("audio = %q, want %q", audio.String(), tc.wantAudio)
			}
		})
	}

	// Verify distinct ElevenLabs voice modes remain cleanly separated:
	// SpeechEngineHandler (http.Handler) and ElevenAgents (core.VoiceSessionProvider).
	el := elevenlabs.New("xi_switch_key")
	if _, err := el.NewSpeechEngineHandler(elevenlabs.SpeechEngineConfig{
		LLM:   &mockVoiceLLM{id: "openai", streaming: true},
		Model: "gpt-5",
	}); err != nil {
		t.Fatalf("NewSpeechEngineHandler() err = %v", err)
	}
	if _, ok := core.AsVoiceSessionProvider(el); !ok {
		t.Error("expected ElevenLabs to implement core.VoiceSessionProvider for ElevenAgents")
	}
}

func TestVoiceAgent_FailureCancellationAndGoroutineCleanup(t *testing.T) {
	// Allow any prior test goroutines to settle before capturing baseline.
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// 1. Mid-turn TTS error emits VoiceEventError; session recovers on next turn.
	ttsErr := errors.New("tts temporarily unavailable")
	var ttsAttempts atomic.Int32
	rtTTS := &mockRealtimeTTS{
		onStart: func(_ context.Context, cfg core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error) {
			if ttsAttempts.Add(1) == 1 {
				return nil, ttsErr
			}
			return newMockRealtimeTTSSession(cfg.Format), nil
		},
	}

	p, err := voice.NewPipeline(voice.PipelineConfig{
		STT:         &mockRealtimeSTT{},
		LLM:         &mockVoiceLLM{id: "openai", streaming: true},
		LLMModel:    "gpt-5",
		RealtimeTTS: rtTTS,
		VoiceID:     "v1",
	})
	if err != nil {
		t.Fatalf("NewPipeline() err = %v", err)
	}

	sess, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}

	_ = sess.SendText(context.Background(), "turn 1 fails tts")
	evts := collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventError
	})
	if !errors.Is(evts[len(evts)-1].Err, ttsErr) {
		t.Errorf("turn 1 error = %v, want %v", evts[len(evts)-1].Err, ttsErr)
	}

	_ = sess.SendText(context.Background(), "turn 2 recovers")
	collectVoiceEventsUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})
	if err := sess.Close(); err != nil {
		t.Fatalf("Close() err = %v", err)
	}

	// 2. Fatal STT failure terminates session and propagates error on Err().
	stt2 := &mockRealtimeSTT{sess: newMockRealtimeSTTSession()}
	p2, _ := voice.NewPipeline(voice.PipelineConfig{
		STT:         stt2,
		LLM:         &mockVoiceLLM{id: "openai"},
		LLMModel:    "gpt-5",
		RealtimeTTS: &mockRealtimeTTS{},
		VoiceID:     "v1",
	})
	sess2, err := p2.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
	if err != nil {
		t.Fatalf("StartVoiceSession(2) err = %v", err)
	}
	sttFatal := errors.New("stt websocket dropped")
	stt2.sess.fail(sttFatal)
	for range sess2.Events() {
	}
	if err := <-sess2.Err(); !errors.Is(err, sttFatal) {
		t.Errorf("sess2 Err() = %v, want %v", err, sttFatal)
	}
	_ = sess2.Close()

	// 3. Parent context cancel with full bounded buffer cleans up cleanly.
	ctx3, cancel3 := context.WithCancel(context.Background())
	p3, _ := voice.NewPipeline(voice.PipelineConfig{
		STT:             &mockRealtimeSTT{},
		LLM:             &mockVoiceLLM{id: "openai"},
		LLMModel:        "gpt-5",
		RealtimeTTS:     &mockRealtimeTTS{},
		VoiceID:         "v1",
		EventBufferSize: 1,
	})
	sess3, err := p3.StartVoiceSession(ctx3, core.VoiceSessionConfig{EventBufferSize: 1})
	if err != nil {
		t.Fatalf("StartVoiceSession(3) err = %v", err)
	}
	cancel3()
	if err := <-sess3.Err(); !errors.Is(err, context.Canceled) {
		t.Errorf("sess3 Err() = %v, want context.Canceled", err)
	}
	_ = sess3.Close()

	// Verify no goroutines leaked across all 3 sessions.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baselineGoroutines+2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("possible goroutine leak: baseline=%d current=%d", baselineGoroutines, runtime.NumGoroutine())
}

// BenchmarkVoiceAgent_FirstAudioLatency captures first-audio latency across
// mocked streaming STT -> LLM -> incremental TTS turns without asserting
// unsupported wall-clock SLA guarantees.
func BenchmarkVoiceAgent_FirstAudioLatency(b *testing.B) {
	metrics := &voiceMetricsCapture{}
	pipeline, err := voice.NewPipeline(voice.PipelineConfig{
		LLM:         &mockVoiceLLM{id: "openai", streaming: true},
		LLMModel:    "gpt-5",
		RealtimeTTS: &mockRealtimeTTS{},
		VoiceID:     "bench-voice",
		Telemetry:   metrics,
	})
	if err != nil {
		b.Fatalf("NewPipeline() err = %v", err)
	}

	sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{
		OutputFormat: core.AudioFormatPCM16000,
	})
	if err != nil {
		b.Fatalf("StartVoiceSession() err = %v", err)
	}
	defer sess.Close()

	b.ResetTimer()
	for range b.N {
		if err := sess.SendText(context.Background(), "Benchmark utterance"); err != nil {
			b.Fatalf("SendText() err = %v", err)
		}
		for evt := range sess.Events() {
			if evt.Type == core.VoiceEventTurnCompleted {
				break
			}
		}
	}
	b.StopTimer()

	metrics.mu.Lock()
	var totalLatency time.Duration
	for _, fc := range metrics.firstChunks {
		totalLatency += fc.FirstChunkLatency()
	}
	count := len(metrics.firstChunks)
	metrics.mu.Unlock()

	if count > 0 {
		avgMS := float64(totalLatency/time.Duration(count)) / float64(time.Millisecond)
		b.ReportMetric(avgMS, "first_audio_ms/op")
	}
}
