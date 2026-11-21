package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/tools"
)

// --- Deterministic test fakes (no TCP sockets) ---

type fakeSTTProvider struct {
	startErr error
	sess     *fakeSTTSession
	mu       sync.Mutex
	lastCfg  core.RealtimeTranscriptionConfig
}

func (p *fakeSTTProvider) StartRealtimeTranscription(_ context.Context, cfg core.RealtimeTranscriptionConfig) (core.RealtimeTranscriptionSession, error) {
	p.mu.Lock()
	p.lastCfg = cfg
	p.mu.Unlock()
	if p.startErr != nil {
		return nil, p.startErr
	}
	if p.sess == nil {
		p.sess = newFakeSTTSession()
	}
	return p.sess, nil
}

type fakeSTTSession struct {
	events   chan core.RealtimeTranscriptEvent
	errCh    chan error
	mu       sync.Mutex
	chunks   []core.AudioChunk
	commits  int
	closed   bool
	closeCh  chan struct{}
	closeRun sync.Once
}

func newFakeSTTSession() *fakeSTTSession {
	return &fakeSTTSession{
		events:  make(chan core.RealtimeTranscriptEvent, 32),
		errCh:   make(chan error, 1),
		closeCh: make(chan struct{}),
	}
}

func (s *fakeSTTSession) SendAudio(_ context.Context, chunk core.AudioChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrRealtimeSessionClosed
	}
	s.chunks = append(s.chunks, chunk)
	return nil
}

func (s *fakeSTTSession) Commit(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrRealtimeSessionClosed
	}
	s.commits++
	return nil
}

func (s *fakeSTTSession) Events() <-chan core.RealtimeTranscriptEvent { return s.events }
func (s *fakeSTTSession) Err() <-chan error                           { return s.errCh }

func (s *fakeSTTSession) Close() error {
	s.closeRun.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.closeCh)
		close(s.events)
		close(s.errCh)
	})
	return nil
}

func (s *fakeSTTSession) fail(err error) {
	s.closeRun.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.closeCh)
		s.errCh <- err
		close(s.events)
		close(s.errCh)
	})
}

type fakeLLMProvider struct {
	id        string
	streaming bool
	chatFn    func(context.Context, *core.ChatRequest) (*core.ChatResponse, error)
	streamFn  func(context.Context, *core.ChatRequest) (*core.ChatStream, error)
	mu        sync.Mutex
	seenReqs  []*core.ChatRequest
}

func (p *fakeLLMProvider) ID() string               { return p.id }
func (p *fakeLLMProvider) Models() []core.ModelInfo { return nil }
func (p *fakeLLMProvider) Supports(f core.Feature) bool {
	if f == core.FeatureChat {
		return true
	}
	if f == core.FeatureChatStreaming {
		return p.streaming
	}
	return false
}

func (p *fakeLLMProvider) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	p.mu.Lock()
	p.seenReqs = append(p.seenReqs, req)
	p.mu.Unlock()
	if p.chatFn != nil {
		return p.chatFn(ctx, req)
	}
	return &core.ChatResponse{Output: "sync reply"}, nil
}

func (p *fakeLLMProvider) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	p.mu.Lock()
	p.seenReqs = append(p.seenReqs, req)
	p.mu.Unlock()
	if p.streamFn != nil {
		return p.streamFn(ctx, req)
	}
	return nil, core.ErrNotSupported
}

type fakeRealtimeTTSProvider struct {
	mu       sync.Mutex
	sessions []*fakeRealtimeTTSSession
	startFn  func(context.Context, core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error)
}

func (p *fakeRealtimeTTSProvider) StartRealtimeSynthesis(ctx context.Context, cfg core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error) {
	if p.startFn != nil {
		return p.startFn(ctx, cfg)
	}
	s := newFakeRealtimeTTSSession(cfg.Format)
	p.mu.Lock()
	p.sessions = append(p.sessions, s)
	p.mu.Unlock()
	return s, nil
}

type fakeRealtimeTTSSession struct {
	format    core.AudioFormat
	audio     chan core.AudioChunk
	errCh     chan error
	mu        sync.Mutex
	sentTexts []string
	seq       int64
	closed    bool
	closeOnce sync.Once
}

func newFakeRealtimeTTSSession(format core.AudioFormat) *fakeRealtimeTTSSession {
	if format.IsZero() {
		format = core.AudioFormatPCM16000
	}
	return &fakeRealtimeTTSSession{
		format: format,
		audio:  make(chan core.AudioChunk, 32),
		errCh:  make(chan error, 1),
	}
}

func (s *fakeRealtimeTTSSession) SendText(_ context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return core.ErrRealtimeSessionClosed
	}
	s.sentTexts = append(s.sentTexts, text)
	s.audio <- core.AudioChunk{
		Data:     []byte("audio:" + text),
		Format:   s.format,
		Sequence: s.seq,
	}
	s.seq++
	return nil
}

func (s *fakeRealtimeTTSSession) Flush(_ context.Context) error { return nil }

func (s *fakeRealtimeTTSSession) Finish(_ context.Context) error {
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

func (s *fakeRealtimeTTSSession) Audio() <-chan core.AudioChunk { return s.audio }
func (s *fakeRealtimeTTSSession) Err() <-chan error             { return s.errCh }

func (s *fakeRealtimeTTSSession) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.audio)
		close(s.errCh)
	})
	return nil
}

type fakeHTTPTTSProvider struct {
	streamFn func(context.Context, *core.SpeechRequest) (*core.SpeechStream, error)
}

func (p *fakeHTTPTTSProvider) Synthesize(_ context.Context, _ *core.SpeechRequest) (*core.SpeechResponse, error) {
	return nil, core.ErrNotSupported
}

func (p *fakeHTTPTTSProvider) StreamSynthesize(ctx context.Context, req *core.SpeechRequest) (*core.SpeechStream, error) {
	if p.streamFn != nil {
		return p.streamFn(ctx, req)
	}
	ch := make(chan core.AudioChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.SpeechResponse, 1)
	ch <- core.AudioChunk{Data: []byte("http-tts:" + req.Text), Format: core.AudioFormatPCM16000, Sequence: 0}
	ch <- core.AudioChunk{Format: core.AudioFormatPCM16000, Sequence: 1, IsFinal: true}
	close(ch)
	finalCh <- &core.SpeechResponse{Model: req.Model, VoiceID: req.VoiceID}
	close(finalCh)
	close(errCh)
	return &core.SpeechStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

type executableTestTool struct {
	name string
	fn   func(context.Context, json.RawMessage) (any, error)
}

func (t executableTestTool) Name() string            { return t.name }
func (t executableTestTool) Description() string     { return "executable test tool" }
func (t executableTestTool) Schema() core.ToolSchema { return core.ToolSchema{} }
func (t executableTestTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return t.fn(ctx, args)
}

type schemaOnlyTestTool struct {
	name string
}

func (t schemaOnlyTestTool) Name() string            { return t.name }
func (t schemaOnlyTestTool) Description() string     { return "schema-only test tool" }
func (t schemaOnlyTestTool) Schema() core.ToolSchema { return core.ToolSchema{} }

func collectUntil(t *testing.T, events <-chan core.VoiceEvent, pred func(core.VoiceEvent) bool) []core.VoiceEvent {
	t.Helper()
	var collected []core.VoiceEvent
	deadline := time.After(3 * time.Second)
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				t.Fatalf("events channel closed before predicate matched; got %d events: %v", len(collected), collected)
			}
			collected = append(collected, evt)
			if pred(evt) {
				return collected
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event; collected %d events: %v", len(collected), collected)
		}
	}
}

// --- Tests ---

func TestPipelineEndToEndStreamingSTTLLMAndRealtimeTTS(t *testing.T) {
	stt := &fakeSTTProvider{sess: newFakeSTTSession()}
	llm := &fakeLLMProvider{
		id:        "openai",
		streaming: true,
		streamFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
			ch := make(chan core.ChatChunk, 4)
			errCh := make(chan error, 1)
			finalCh := make(chan *core.ChatResponse, 1)
			ch <- core.ChatChunk{Delta: "Hello "}
			ch <- core.ChatChunk{Delta: "from "}
			ch <- core.ChatChunk{Delta: "Iris!"}
			close(ch)
			finalCh <- &core.ChatResponse{Output: "Hello from Iris!"}
			close(finalCh)
			close(errCh)
			return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
		},
	}
	rtTTS := &fakeRealtimeTTSProvider{}

	pipeline, err := NewPipeline(PipelineConfig{
		STT:          stt,
		LLM:          llm,
		LLMModel:     "gpt-5",
		RealtimeTTS:  rtTTS,
		VoiceID:      "voice-1",
		SystemPrompt: "Be helpful.",
	})
	if err != nil {
		t.Fatalf("NewPipeline() err = %v", err)
	}

	rawSess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{
		InputFormat:  core.AudioFormatPCM16000,
		OutputFormat: core.AudioFormatPCM16000,
	})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}
	sess := rawSess.(*Session)
	defer sess.Close()

	// Send audio chunk + manual commit.
	if err := sess.SendAudio(context.Background(), core.AudioChunk{
		Data:   []byte{1, 2, 3, 4},
		Format: core.AudioFormatPCM16000,
	}); err != nil {
		t.Fatalf("SendAudio() err = %v", err)
	}
	if err := sess.CommitAudio(context.Background()); err != nil {
		t.Fatalf("CommitAudio() err = %v", err)
	}

	// Simulate STT partial followed by committed utterance.
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventPartial,
		IsFinal:    false,
		Transcript: &core.TranscriptionResponse{Text: "Hi", IsFinal: false},
	}
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventCommitted,
		IsFinal:    true,
		Transcript: &core.TranscriptionResponse{Text: "Hi there", IsFinal: true},
	}

	events := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})

	var (
		sawPartial     bool
		sawCommitted   bool
		agentDeltas    strings.Builder
		agentCommitted string
		audioBuf       bytes.Buffer
		sawAudioEnded  bool
		turnID         string
	)
	for _, e := range events {
		if err := e.Validate(); err != nil {
			t.Errorf("emitted invalid event %s: %v", e.String(), err)
		}
		switch e.Type {
		case core.VoiceEventUserTranscriptPartial:
			sawPartial = true
			if e.IsFinal {
				t.Error("partial transcript should have IsFinal=false")
			}
		case core.VoiceEventUserTranscriptCommitted:
			sawCommitted = true
			turnID = e.TurnID
			if !e.IsFinal || e.Transcript.Text != "Hi there" {
				t.Errorf("unexpected committed transcript: %+v", e.Transcript)
			}
		case core.VoiceEventAgentTextDelta:
			if e.TurnID != turnID {
				t.Errorf("delta TurnID=%q, want %q", e.TurnID, turnID)
			}
			agentDeltas.WriteString(e.Text)
		case core.VoiceEventAgentTextCommitted:
			agentCommitted = e.Text
		case core.VoiceEventAudioOutput:
			if e.TurnID != turnID {
				t.Errorf("audio TurnID=%q, want %q", e.TurnID, turnID)
			}
			audioBuf.Write(e.Audio.Data)
		case core.VoiceEventAudioOutputEnded:
			sawAudioEnded = true
		}
	}

	if !sawPartial || !sawCommitted || !sawAudioEnded {
		t.Fatalf("missing expected events: partial=%t committed=%t audioEnded=%t", sawPartial, sawCommitted, sawAudioEnded)
	}
	if agentDeltas.String() != "Hello from Iris!" || agentCommitted != "Hello from Iris!" {
		t.Errorf("agent text mismatch: deltas=%q committed=%q", agentDeltas.String(), agentCommitted)
	}
	if audioBuf.String() != "audio:Hello audio:from audio:Iris!" {
		t.Errorf("streamed audio = %q", audioBuf.String())
	}

	// Verify history captured committed user + assistant turns only (no partials).
	hist := sess.History()
	if len(hist) != 2 || hist[0].Role != core.RoleUser || hist[0].Content != "Hi there" ||
		hist[1].Role != core.RoleAssistant || hist[1].Content != "Hello from Iris!" {
		t.Fatalf("unexpected history: %+v", hist)
	}

	// Clean Close emits SessionEnded and nil Err.
	if err := sess.Close(); err != nil {
		t.Fatalf("Close() err = %v", err)
	}
	if sess.State() != core.VoiceSessionStateClosed {
		t.Errorf("State() after Close = %q, want closed", sess.State())
	}
	if err := <-sess.Err(); err != nil {
		t.Errorf("unexpected terminal Err() after clean Close: %v", err)
	}
}

func TestPipelineFallbackNonStreamingLLMAndHTTPTTS(t *testing.T) {
	llm := &fakeLLMProvider{
		id:        "anthropic",
		streaming: false,
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			return &core.ChatResponse{Output: "Fallback response."}, nil
		},
	}
	httpTTS := &fakeHTTPTTSProvider{}

	pipeline, err := NewPipeline(PipelineConfig{
		LLM:                llm,
		LLMModel:           "claude-sonnet",
		TTS:                httpTTS,
		VoiceID:            "voice-http",
		MaxHistoryMessages: 2,
	})
	if err != nil {
		t.Fatalf("NewPipeline() err = %v", err)
	}

	rawSess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}
	sess := rawSess.(*Session)
	defer sess.Close()

	// SendAudio / CommitAudio without STT return ErrNotSupported.
	if err := sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2}}); !errors.Is(err, core.ErrNotSupported) {
		t.Errorf("SendAudio without STT err = %v, want ErrNotSupported", err)
	}
	if err := sess.CommitAudio(context.Background()); !errors.Is(err, core.ErrNotSupported) {
		t.Errorf("CommitAudio without STT err = %v, want ErrNotSupported", err)
	}

	// Turn 1 & Turn 2 via SendText (also verifies MaxHistoryMessages = 2 trimming).
	for _, msg := range []string{"First question", "Second question"} {
		if err := sess.SendText(context.Background(), msg); err != nil {
			t.Fatalf("SendText(%q) err = %v", msg, err)
		}
		evts := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventTurnCompleted
		})
		sawAudio := false
		for _, e := range evts {
			if e.Type == core.VoiceEventAudioOutput && string(e.Audio.Data) == "http-tts:Fallback response." {
				sawAudio = true
			}
		}
		if !sawAudio {
			t.Errorf("turn %q did not receive HTTP fallback TTS audio", msg)
		}
	}

	hist := sess.History()
	if len(hist) != 2 || hist[0].Content != "Second question" {
		t.Errorf("expected bounded history len=2 starting with latest turn, got %+v", hist)
	}
}

func TestPipelineAutoAndExternalToolHandling(t *testing.T) {
	t.Run("automatic tools.Tool and RegistryToolExecutor", func(t *testing.T) {
		reg := tools.NewRegistry()
		if err := reg.Register(executableTestTool{
			name: "get_weather",
			fn: func(_ context.Context, args json.RawMessage) (any, error) {
				return map[string]string{"temp": "72F"}, nil
			},
		}); err != nil {
			t.Fatalf("Register() err = %v", err)
		}
		if err := reg.Register(executableTestTool{
			name: "failing_tool",
			fn: func(_ context.Context, _ json.RawMessage) (any, error) {
				return nil, errors.New("tool backend down")
			},
		}); err != nil {
			t.Fatalf("Register(failing) err = %v", err)
		}

		var round atomic.Int32
		llm := &fakeLLMProvider{
			id:        "openai",
			streaming: true,
			streamFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
				ch := make(chan core.ChatChunk, 2)
				errCh := make(chan error, 1)
				finalCh := make(chan *core.ChatResponse, 1)
				if round.Add(1) == 1 {
					close(ch)
					finalCh <- &core.ChatResponse{
						ToolCalls: []core.ToolCall{
							{ID: "call-1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"SF"}`)},
							{ID: "call-2", Name: "failing_tool", Arguments: json.RawMessage(`{}`)},
						},
					}
				} else {
					ch <- core.ChatChunk{Delta: "It is 72F in SF."}
					close(ch)
					finalCh <- &core.ChatResponse{Output: "It is 72F in SF."}
				}
				close(finalCh)
				close(errCh)
				return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		rtTTS := &fakeRealtimeTTSProvider{}
		regTools := reg.List()
		coreTools := make([]core.Tool, len(regTools))
		for i, rt := range regTools {
			coreTools[i] = rt
		}

		pipeline, err := NewPipeline(PipelineConfig{
			LLM:          llm,
			LLMModel:     "gpt-5",
			RealtimeTTS:  rtTTS,
			VoiceID:      "v1",
			Tools:        coreTools,
			ToolExecutor: RegistryToolExecutor(reg),
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}

		sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		if err := sess.SendText(context.Background(), "Weather in SF?"); err != nil {
			t.Fatalf("SendText() err = %v", err)
		}

		evts := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventTurnCompleted
		})

		var toolCalls, toolResults int
		var sawErrorResult bool
		for _, e := range evts {
			if e.Type == core.VoiceEventToolCall {
				toolCalls++
			}
			if e.Type == core.VoiceEventToolResult {
				toolResults++
				if e.ToolResult.CallID == "call-2" && e.ToolResult.IsError {
					sawErrorResult = true
				}
			}
		}
		if toolCalls != 2 || toolResults != 2 || !sawErrorResult {
			t.Fatalf("tool events mismatch: calls=%d results=%d sawErrorResult=%t", toolCalls, toolResults, sawErrorResult)
		}
		// Lazy TTS open: tool-only round 1 must NOT open a TTS session; only round 2 opens 1 session.
		rtTTS.mu.Lock()
		ttsCount := len(rtTTS.sessions)
		rtTTS.mu.Unlock()
		if ttsCount != 1 {
			t.Errorf("TTS sessions opened = %d, want 1 (lazy open on spoken round)", ttsCount)
		}
	})

	t.Run("external schema-only tool via SubmitToolResult", func(t *testing.T) {
		var round atomic.Int32
		llm := &fakeLLMProvider{
			id:        "gemini",
			streaming: false,
			chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
				if round.Add(1) == 1 {
					return &core.ChatResponse{
						ToolCalls: []core.ToolCall{
							{ID: "ext-call-9", Name: "external_lookup", Arguments: json.RawMessage(`{"id":9}`)},
						},
					}, nil
				}
				return &core.ChatResponse{Output: "Order 9 is shipped."}, nil
			},
		}

		pipeline, err := NewPipeline(PipelineConfig{
			LLM:         llm,
			LLMModel:    "gemini-2.5",
			RealtimeTTS: &fakeRealtimeTTSProvider{},
			VoiceID:     "v1",
			Tools:       []core.Tool{schemaOnlyTestTool{name: "external_lookup"}},
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}

		sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		// Unknown CallID fails fast.
		if err := sess.SubmitToolResult(context.Background(), core.ToolResult{CallID: "nope"}); !errors.Is(err, ErrToolCallNotFound) {
			t.Errorf("unknown CallID err = %v, want ErrToolCallNotFound", err)
		}

		if err := sess.SendText(context.Background(), "Where is order 9?"); err != nil {
			t.Fatalf("SendText() err = %v", err)
		}

		collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventToolCall && e.ToolCall.ID == "ext-call-9"
		})

		if err := sess.SubmitToolResult(context.Background(), core.ToolResult{
			CallID:  "ext-call-9",
			Content: "shipped",
		}); err != nil {
			t.Fatalf("SubmitToolResult() err = %v", err)
		}

		evts := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventTurnCompleted
		})
		sawFinalText := false
		for _, e := range evts {
			if e.Type == core.VoiceEventAgentTextCommitted && e.Text == "Order 9 is shipped." {
				sawFinalText = true
			}
		}
		if !sawFinalText {
			t.Error("expected committed final response after SubmitToolResult")
		}
	})

	t.Run("max tool rounds exceeded emits turn error", func(t *testing.T) {
		llm := &fakeLLMProvider{
			id:        "openai",
			streaming: false,
			chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
				return &core.ChatResponse{
					ToolCalls: []core.ToolCall{{ID: "loop", Name: "ping", Arguments: json.RawMessage(`{}`)}},
				}, nil
			},
		}
		pipeline, err := NewPipeline(PipelineConfig{
			LLM:           llm,
			LLMModel:      "gpt-5",
			RealtimeTTS:   &fakeRealtimeTTSProvider{},
			VoiceID:       "v1",
			MaxToolRounds: 1,
			Tools: []core.Tool{executableTestTool{
				name: "ping",
				fn:   func(_ context.Context, _ json.RawMessage) (any, error) { return "pong", nil },
			}},
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}
		sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		_ = sess.SendText(context.Background(), "loop forever")
		evts := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventError
		})
		last := evts[len(evts)-1]
		if !errors.Is(last.Err, ErrMaxToolRoundsExceeded) {
			t.Errorf("error event = %v, want ErrMaxToolRoundsExceeded", last.Err)
		}
	})
}

func TestPipelineInterruptionBargeInAndStaleAudioSuppression(t *testing.T) {
	stt := &fakeSTTProvider{sess: newFakeSTTSession()}
	var turnNum atomic.Int32
	llmStarted := make(chan struct{}, 2)

	llm := &fakeLLMProvider{
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
					ch <- core.ChatChunk{Delta: "Long speaking turn... "}
					llmStarted <- struct{}{}
					// Block until interrupted/cancelled.
					<-ctx.Done()
					errCh <- ctx.Err()
					return
				}
				ch <- core.ChatChunk{Delta: "Fresh answer."}
				finalCh <- &core.ChatResponse{Output: "Fresh answer."}
			}()
			return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
		},
	}

	rtTTS := &fakeRealtimeTTSProvider{}
	pipeline, err := NewPipeline(PipelineConfig{
		STT:         stt,
		LLM:         llm,
		LLMModel:    "gpt-5",
		RealtimeTTS: rtTTS,
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

	// Start turn 1 via SendText; wait until LLM has emitted first delta and blocked.
	if err := sess.SendText(context.Background(), "Tell me a long story"); err != nil {
		t.Fatalf("SendText() err = %v", err)
	}
	<-llmStarted

	// Trigger automatic barge-in via non-empty partial transcript from STT.
	stt.sess.events <- core.RealtimeTranscriptEvent{
		Type:       core.RealtimeTranscriptEventPartial,
		IsFinal:    false,
		Transcript: &core.TranscriptionResponse{Text: "Stop!", IsFinal: false},
	}

	evts := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnInterrupted
	})
	var cancelledTurnID string
	for _, e := range evts {
		if e.Type == core.VoiceEventTurnInterrupted {
			cancelledTurnID = e.Interruption.CancelledTurnID
			if e.Interruption.Reason != core.VoiceInterruptionBargeIn {
				t.Errorf("reason = %q, want barge_in", e.Interruption.Reason)
			}
		}
	}
	if cancelledTurnID == "" {
		t.Fatal("expected non-empty CancelledTurnID")
	}

	// Follow-up turn 2 succeeds cleanly after interruption.
	if err := sess.SendText(context.Background(), "Short answer please"); err != nil {
		t.Fatalf("SendText(2) err = %v", err)
	}
	evts2 := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})
	for _, e := range evts2 {
		if e.Type == core.VoiceEventAudioOutput && e.TurnID == cancelledTurnID {
			t.Errorf("stale audio from cancelled turn %q leaked after interruption!", cancelledTurnID)
		}
	}
}

func TestPipelineFailuresCancellationAndConcurrency(t *testing.T) {
	t.Run("validation and config privacy", func(t *testing.T) {
		cfg := PipelineConfig{
			LLM:          &fakeLLMProvider{id: "openai"},
			RealtimeTTS:  &fakeRealtimeTTSProvider{},
			SystemPrompt: "SECRET_PIPELINE_PROMPT",
		}
		for _, out := range []string{cfg.String(), fmt.Sprintf("%#v", cfg)} {
			if strings.Contains(out, "SECRET_PIPELINE_PROMPT") {
				t.Errorf("PipelineConfig leaked SystemPrompt: %s", out)
			}
		}

		badConfigs := []PipelineConfig{
			{},                                    // missing LLM
			{LLM: &fakeLLMProvider{id: "openai"}}, // missing TTS
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, CommitStrategy: "bad"},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, MaxToolRounds: -1},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, MaxHistoryMessages: -1},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, EventBufferSize: -1},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, ToolTimeout: -time.Second},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, ChunkLengthSchedule: []int{0}},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, Tools: []core.Tool{nil}},
			{LLM: &fakeLLMProvider{id: "openai"}, RealtimeTTS: &fakeRealtimeTTSProvider{}, Tools: []core.Tool{schemaOnlyTestTool{name: "  "}}},
		}
		for i, bc := range badConfigs {
			if _, err := NewPipeline(bc); !errors.Is(err, ErrInvalidPipelineConfig) {
				t.Errorf("badConfig[%d] err = %v, want ErrInvalidPipelineConfig", i, err)
			}
		}

		p, err := NewPipeline(PipelineConfig{
			LLM:         &fakeLLMProvider{id: "openai"},
			RealtimeTTS: &fakeRealtimeTTSProvider{},
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}
		//nolint:staticcheck // intentionally testing nil context guard
		if _, err := p.StartVoiceSession(nil, core.VoiceSessionConfig{}); err == nil {
			t.Error("expected error for nil ctx")
		}
		if _, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{}); !errors.Is(err, core.ErrInvalidVoiceSessionConfig) {
			t.Errorf("missing model err = %v, want ErrInvalidVoiceSessionConfig", err)
		}
		if _, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{Model: "gpt-5"}); !errors.Is(err, core.ErrInvalidVoiceSessionConfig) {
			t.Errorf("missing voice_id err = %v, want ErrInvalidVoiceSessionConfig", err)
		}
	})

	t.Run("STT transport failure fails session", func(t *testing.T) {
		stt := &fakeSTTProvider{sess: newFakeSTTSession()}
		p, err := NewPipeline(PipelineConfig{
			STT:         stt,
			LLM:         &fakeLLMProvider{id: "openai"},
			LLMModel:    "gpt-5",
			RealtimeTTS: &fakeRealtimeTTSProvider{},
			VoiceID:     "v1",
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}
		sess, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		boom := errors.New("stt socket dropped")
		stt.sess.fail(boom)

		for range sess.Events() {
		}
		if err := <-sess.Err(); !errors.Is(err, boom) {
			t.Fatalf("Err() = %v, want %v", err, boom)
		}
	})

	t.Run("LLM error emits turn VoiceEventError and session recovers", func(t *testing.T) {
		var calls atomic.Int32
		llm := &fakeLLMProvider{
			id:        "openai",
			streaming: false,
			chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
				if calls.Add(1) == 1 {
					return nil, core.ErrRateLimited
				}
				return &core.ChatResponse{Output: "Recovered!"}, nil
			},
		}
		p, err := NewPipeline(PipelineConfig{
			LLM:         llm,
			LLMModel:    "gpt-5",
			RealtimeTTS: &fakeRealtimeTTSProvider{},
			VoiceID:     "v1",
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}
		sess, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		defer sess.Close()

		_ = sess.SendText(context.Background(), "turn 1")
		evts := collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventError
		})
		if !errors.Is(evts[len(evts)-1].Err, core.ErrRateLimited) {
			t.Errorf("turn err = %v, want ErrRateLimited", evts[len(evts)-1].Err)
		}

		_ = sess.SendText(context.Background(), "turn 2")
		collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
			return e.Type == core.VoiceEventTurnCompleted
		})
	})

	t.Run("parent context cancellation and full-buffer Close do not deadlock", func(t *testing.T) {
		p, err := NewPipeline(PipelineConfig{
			LLM:             &fakeLLMProvider{id: "openai"},
			LLMModel:        "gpt-5",
			RealtimeTTS:     &fakeRealtimeTTSProvider{},
			VoiceID:         "v1",
			EventBufferSize: 1,
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		sess, err := p.StartVoiceSession(ctx, core.VoiceSessionConfig{EventBufferSize: 1})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		// Do not drain Events() (buffer cap 1 is already filled by SessionStarted).
		cancel()

		select {
		case gotErr := <-sess.Err():
			if !errors.Is(gotErr, context.Canceled) {
				t.Errorf("Err() = %v, want context.Canceled", gotErr)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for parent cancellation cleanup")
		}
		_ = sess.Close()

		// Also verify Close() on full unconsumed buffer never deadlocks.
		sess2, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{EventBufferSize: 1})
		if err != nil {
			t.Fatalf("StartVoiceSession(2) err = %v", err)
		}
		done := make(chan struct{})
		go func() {
			_ = sess2.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Close() deadlocked on full unconsumed Events() buffer")
		}
	})

	t.Run("concurrent SendText Interrupt History State and Close under race", func(t *testing.T) {
		p, err := NewPipeline(PipelineConfig{
			STT:         &fakeSTTProvider{sess: newFakeSTTSession()},
			LLM:         &fakeLLMProvider{id: "openai"},
			LLMModel:    "gpt-5",
			RealtimeTTS: &fakeRealtimeTTSProvider{},
			VoiceID:     "v1",
		})
		if err != nil {
			t.Fatalf("NewPipeline() err = %v", err)
		}
		rawSess, err := p.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
		if err != nil {
			t.Fatalf("StartVoiceSession() err = %v", err)
		}
		sess := rawSess.(*Session)

		drainDone := make(chan struct{})
		go func() {
			defer close(drainDone)
			for range sess.Events() {
			}
		}()

		var wg sync.WaitGroup
		for range 6 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					_ = sess.SendAudio(context.Background(), core.AudioChunk{Data: []byte{1, 2}})
					_ = sess.SendText(context.Background(), "hello")
					_ = sess.Interrupt(context.Background())
					_ = sess.State()
					_ = sess.History()
				}
			}()
		}
		time.Sleep(5 * time.Millisecond)
		_ = sess.Close()
		_ = sess.Close()
		wg.Wait()
		<-drainDone
	})
}

type pipelineTelemetryRecorder struct {
	core.NoopVoiceTelemetryHook
	mu          sync.Mutex
	firstChunks int
	turns       int
	sessEnds    []core.VoiceSessionEndEvent
}

func (r *pipelineTelemetryRecorder) OnVoiceFirstChunk(_ context.Context, _ core.VoiceFirstChunkEvent) {
	r.mu.Lock()
	r.firstChunks++
	r.mu.Unlock()
}

func (r *pipelineTelemetryRecorder) OnVoiceTurnEnd(_ context.Context, _ core.VoiceTurnEndEvent) {
	r.mu.Lock()
	r.turns++
	r.mu.Unlock()
}

func (r *pipelineTelemetryRecorder) OnVoiceSessionEnd(_ context.Context, e core.VoiceSessionEndEvent) {
	r.mu.Lock()
	r.sessEnds = append(r.sessEnds, e)
	r.mu.Unlock()
}

func TestPipelineTelemetryHook(t *testing.T) {
	rec := &pipelineTelemetryRecorder{}
	pipeline, err := NewPipeline(PipelineConfig{
		LLM:         &fakeLLMProvider{id: "openai"},
		LLMModel:    "gpt-5",
		RealtimeTTS: &fakeRealtimeTTSProvider{},
		VoiceID:     "v1",
		Telemetry:   rec,
	})
	if err != nil {
		t.Fatalf("NewPipeline() err = %v", err)
	}

	sess, err := pipeline.StartVoiceSession(context.Background(), core.VoiceSessionConfig{})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}

	if err := sess.SendText(context.Background(), "Hello"); err != nil {
		t.Fatalf("SendText() err = %v", err)
	}
	collectUntil(t, sess.Events(), func(e core.VoiceEvent) bool {
		return e.Type == core.VoiceEventTurnCompleted
	})

	if err := sess.Close(); err != nil {
		t.Fatalf("Close() err = %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.firstChunks != 1 || rec.turns != 1 || len(rec.sessEnds) != 1 {
		t.Fatalf("unexpected telemetry counts: firstChunks=%d turns=%d sessEnds=%d",
			rec.firstChunks, rec.turns, len(rec.sessEnds))
	}
	if rec.sessEnds[0].Mode != "pipeline" || rec.sessEnds[0].OutputAudioBytes == 0 {
		t.Errorf("unexpected session end metrics: %+v", rec.sessEnds[0])
	}
}
