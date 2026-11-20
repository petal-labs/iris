package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type voiceSessionTestTool struct {
	name string
}

func (t voiceSessionTestTool) Name() string        { return t.name }
func (t voiceSessionTestTool) Description() string { return "test tool" }
func (t voiceSessionTestTool) Schema() ToolSchema  { return ToolSchema{} }

type voiceSessionTestProvider struct {
	*capabilityBaseProvider
	start func(context.Context, VoiceSessionConfig) (VoiceSession, error)
}

func (p *voiceSessionTestProvider) StartVoiceSession(ctx context.Context, cfg VoiceSessionConfig) (VoiceSession, error) {
	if p.start == nil {
		return nil, ErrNotSupported
	}
	return p.start(ctx, cfg)
}

type noopVoiceSession struct {
	id    string
	state VoiceSessionState
	bus   *VoiceEventEmitter
}

func (s *noopVoiceSession) ID() string                                         { return s.id }
func (s *noopVoiceSession) State() VoiceSessionState                           { return s.state }
func (s *noopVoiceSession) SendAudio(context.Context, AudioChunk) error        { return nil }
func (s *noopVoiceSession) SendText(context.Context, string) error             { return nil }
func (s *noopVoiceSession) SubmitToolResult(context.Context, ToolResult) error { return nil }
func (s *noopVoiceSession) Interrupt(context.Context) error                    { return nil }
func (s *noopVoiceSession) Events() <-chan VoiceEvent                          { return s.bus.Events() }
func (s *noopVoiceSession) Err() <-chan error                                  { return s.bus.Err() }
func (s *noopVoiceSession) Close() error {
	s.bus.Close()
	return nil
}

func TestVoiceSessionEnumsValidity(t *testing.T) {
	states := []VoiceSessionState{
		VoiceSessionStateConnecting,
		VoiceSessionStateActive,
		VoiceSessionStateListening,
		VoiceSessionStateThinking,
		VoiceSessionStateSpeaking,
		VoiceSessionStateInterrupted,
		VoiceSessionStateClosed,
	}
	for _, s := range states {
		if !s.IsValid() {
			t.Errorf("state %q should be valid", s)
		}
	}
	if VoiceSessionState("unknown").IsValid() {
		t.Error("unknown state should be invalid")
	}

	types := []VoiceEventType{
		VoiceEventSessionStarted,
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
		VoiceEventError,
	}
	for _, et := range types {
		if !et.IsValid() {
			t.Errorf("event type %q should be valid", et)
		}
	}
	if VoiceEventType("bogus").IsValid() {
		t.Error("bogus event type should be invalid")
	}

	reasons := []VoiceInterruptionReason{
		"",
		VoiceInterruptionBargeIn,
		VoiceInterruptionClientCancel,
		VoiceInterruptionSessionClosed,
	}
	for _, r := range reasons {
		if !r.IsValid() {
			t.Errorf("interruption reason %q should be valid", r)
		}
	}
	if VoiceInterruptionReason("mystery").IsValid() {
		t.Error("mystery interruption reason should be invalid")
	}
}

func TestVoiceSessionConfigValidationAndPrivacy(t *testing.T) {
	stab := 0.5
	badStab := 2.0

	valid := VoiceSessionConfig{
		SessionID:          "sess-1",
		AgentID:            "agent-1",
		Model:              "gpt-5",
		VoiceID:            "voice-1",
		VoiceSettings:      &VoiceSettings{Stability: &stab},
		SystemPrompt:       "TOP_SECRET_SYSTEM_INSTRUCTIONS",
		Language:           "en",
		InputFormat:        AudioFormatPCM16000,
		OutputFormat:       AudioFormatPCM24000,
		Tools:              []Tool{voiceSessionTestTool{name: "lookup_order"}},
		EventBufferSize:    32,
		MaxAudioChunkBytes: 65536,
		Metadata:           map[string]string{"user_secret": "SUPER_SECRET_USER_DATA"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid.Validate() = %v", err)
	}
	if err := (VoiceSessionConfig{}).Validate(); err != nil {
		t.Fatalf("empty VoiceSessionConfig.Validate() = %v", err)
	}

	// Privacy: String and GoString must omit SystemPrompt and Metadata values.
	for _, out := range []string{valid.String(), fmt.Sprintf("%#v", valid)} {
		if strings.Contains(out, "TOP_SECRET_SYSTEM_INSTRUCTIONS") {
			t.Errorf("config output leaked SystemPrompt: %s", out)
		}
		if strings.Contains(out, "SUPER_SECRET_USER_DATA") {
			t.Errorf("config output leaked Metadata value: %s", out)
		}
		if !strings.Contains(out, "sess-1") {
			t.Errorf("config output missing session_id: %s", out)
		}
	}

	invalid := []struct {
		name string
		cfg  VoiceSessionConfig
	}{
		{"invalid input format", VoiceSessionConfig{InputFormat: AudioFormat{Encoding: "bad"}}},
		{"invalid output format", VoiceSessionConfig{OutputFormat: AudioFormat{Encoding: "bad"}}},
		{"invalid voice settings", VoiceSessionConfig{VoiceSettings: &VoiceSettings{Stability: &badStab}}},
		{"negative event buffer", VoiceSessionConfig{EventBufferSize: -1}},
		{"negative max chunk bytes", VoiceSessionConfig{MaxAudioChunkBytes: -1}},
		{"nil tool", VoiceSessionConfig{Tools: []Tool{nil}}},
		{"blank tool name", VoiceSessionConfig{Tools: []Tool{voiceSessionTestTool{name: "   "}}}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); !errors.Is(err, ErrInvalidVoiceSessionConfig) {
				t.Errorf("Validate() = %v, want ErrInvalidVoiceSessionConfig", err)
			}
		})
	}
}

func TestVoiceEventValidationAndPrivacy(t *testing.T) {
	validEvents := []VoiceEvent{
		{Type: VoiceEventSessionStarted, SessionID: "s1", State: VoiceSessionStateActive},
		{Type: VoiceEventSessionState, SessionID: "s1", State: VoiceSessionStateListening},
		{Type: VoiceEventSessionEnded, SessionID: "s1", State: VoiceSessionStateClosed},
		{Type: VoiceEventTurnStarted, SessionID: "s1", TurnID: "turn-1"},
		{Type: VoiceEventTurnCompleted, SessionID: "s1", TurnID: "turn-1"},
		{
			Type:   VoiceEventTurnInterrupted,
			TurnID: "turn-1",
			Interruption: &VoiceInterruption{
				CancelledTurnID: "turn-1",
				NextTurnID:      "turn-2",
				Reason:          VoiceInterruptionBargeIn,
			},
		},
		{
			Type:       VoiceEventUserTranscriptPartial,
			TurnID:     "turn-1",
			IsFinal:    false,
			Transcript: &TranscriptionResponse{Text: "SECRET_USER_PARTIAL", IsFinal: false},
		},
		{
			Type:       VoiceEventUserTranscriptCommitted,
			TurnID:     "turn-1",
			IsFinal:    true,
			Transcript: &TranscriptionResponse{Text: "SECRET_USER_COMMITTED", IsFinal: true},
		},
		{Type: VoiceEventAgentTextDelta, TurnID: "turn-1", Text: "SECRET_AGENT_DELTA", IsFinal: false},
		{Type: VoiceEventAgentTextCommitted, TurnID: "turn-1", Text: "SECRET_AGENT_FINAL", IsFinal: true},
		{
			Type:   VoiceEventAudioOutput,
			TurnID: "turn-1",
			Audio:  &AudioChunk{Data: []byte("SECRET_RAW_AUDIO_BYTES"), Format: AudioFormatPCM16000, Sequence: 0},
		},
		{Type: VoiceEventAudioOutputEnded, TurnID: "turn-1"},
		{
			Type:     VoiceEventToolCall,
			TurnID:   "turn-1",
			ToolCall: &ToolCall{ID: "call-1", Name: "get_weather", Arguments: json.RawMessage(`{"secret_arg":"SECRET_TOOL_ARG"}`)},
		},
		{
			Type:       VoiceEventToolResult,
			TurnID:     "turn-1",
			ToolResult: &ToolResult{CallID: "call-1", Content: "SECRET_TOOL_OUTPUT", IsError: false},
		},
		{Type: VoiceEventError, TurnID: "turn-1", Err: errors.New("backend failure")},
	}

	secrets := []string{
		"SECRET_USER_PARTIAL",
		"SECRET_USER_COMMITTED",
		"SECRET_AGENT_DELTA",
		"SECRET_AGENT_FINAL",
		"SECRET_RAW_AUDIO_BYTES",
		"SECRET_TOOL_ARG",
		"SECRET_TOOL_OUTPUT",
	}

	for _, evt := range validEvents {
		if err := evt.Validate(); err != nil {
			t.Errorf("valid event %q failed Validate(): %v", evt.Type, err)
		}
		for _, formatted := range []string{evt.String(), fmt.Sprintf("%#v", evt)} {
			for _, sec := range secrets {
				if strings.Contains(formatted, sec) {
					t.Errorf("event %q leaked sensitive payload %q: %s", evt.Type, sec, formatted)
				}
			}
		}
	}

	invalidEvents := []struct {
		name string
		evt  VoiceEvent
	}{
		{"invalid type", VoiceEvent{Type: "unknown"}},
		{"negative sequence", VoiceEvent{Type: VoiceEventSessionStarted, Sequence: -1}},
		{"invalid state", VoiceEvent{Type: VoiceEventSessionStarted, State: "bad"}},
		{"session_state missing state", VoiceEvent{Type: VoiceEventSessionState}},
		{"turn_started missing turn_id", VoiceEvent{Type: VoiceEventTurnStarted}},
		{"turn_completed missing turn_id", VoiceEvent{Type: VoiceEventTurnCompleted}},
		{"turn_interrupted missing payload", VoiceEvent{Type: VoiceEventTurnInterrupted}},
		{"turn_interrupted invalid reason", VoiceEvent{Type: VoiceEventTurnInterrupted, Interruption: &VoiceInterruption{CancelledTurnID: "t1", Reason: "bad"}}},
		{"partial missing transcript", VoiceEvent{Type: VoiceEventUserTranscriptPartial}},
		{"partial marked final", VoiceEvent{Type: VoiceEventUserTranscriptPartial, IsFinal: true, Transcript: &TranscriptionResponse{}}},
		{"committed not final", VoiceEvent{Type: VoiceEventUserTranscriptCommitted, IsFinal: false, Transcript: &TranscriptionResponse{}}},
		{"agent delta marked final", VoiceEvent{Type: VoiceEventAgentTextDelta, IsFinal: true}},
		{"agent committed not final", VoiceEvent{Type: VoiceEventAgentTextCommitted, IsFinal: false}},
		{"audio_output missing chunk", VoiceEvent{Type: VoiceEventAudioOutput}},
		{"audio_output invalid chunk", VoiceEvent{Type: VoiceEventAudioOutput, Audio: &AudioChunk{}}},
		{"tool_call missing name", VoiceEvent{Type: VoiceEventToolCall, ToolCall: &ToolCall{ID: "c1"}}},
		{"tool_result missing call_id", VoiceEvent{Type: VoiceEventToolResult, ToolResult: &ToolResult{}}},
		{"error missing err and message", VoiceEvent{Type: VoiceEventError}},
	}
	for _, tc := range invalidEvents {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.evt.Validate(); err == nil {
				t.Errorf("expected Validate() error for %s", tc.name)
			}
		})
	}
}

func TestVoiceTurnTrackerLifecycleInterruptionAndStaleAudio(t *testing.T) {
	tracker := NewVoiceTurnTracker("")

	// Start turn 1.
	turn1, ctx1 := tracker.StartTurn()
	if turn1 != "turn-1" || tracker.ActiveTurnID() != "turn-1" || !tracker.IsCurrentTurn("turn-1") {
		t.Fatalf("unexpected turn 1 state: turn=%q active=%q", turn1, tracker.ActiveTurnID())
	}

	audio1 := VoiceEvent{
		Type:   VoiceEventAudioOutput,
		TurnID: turn1,
		Audio:  &AudioChunk{Data: []byte("pcm1"), Sequence: 0},
	}
	if !tracker.ShouldPlayAudio(audio1) {
		t.Fatal("expected audio1 to be playable while turn-1 is active")
	}
	if tracker.ShouldPlayAudio(VoiceEvent{Type: VoiceEventTurnStarted, TurnID: turn1}) {
		t.Error("non-audio event should return false from ShouldPlayAudio")
	}
	if tracker.IsCurrentTurn("") {
		t.Error("empty turn ID should not be current")
	}

	// Interrupt turn 1 (barge-in): ctx1 cancels immediately and stale audio1 is dropped.
	intr, ok := tracker.Interrupt(VoiceInterruptionBargeIn)
	if !ok || intr.CancelledTurnID != turn1 || intr.Reason != VoiceInterruptionBargeIn {
		t.Fatalf("Interrupt() = (%+v, %v)", intr, ok)
	}
	if ctx1.Err() == nil {
		t.Error("expected turn-1 context to be cancelled on Interrupt")
	}
	if tracker.ActiveTurnID() != "" {
		t.Errorf("ActiveTurnID after Interrupt = %q, want empty", tracker.ActiveTurnID())
	}
	if tracker.ShouldPlayAudio(audio1) {
		t.Error("stale audio1 must NOT play after turn-1 interruption")
	}
	// Duplicate interrupt is a no-op.
	if _, ok := tracker.Interrupt(VoiceInterruptionClientCancel); ok {
		t.Error("second Interrupt while already interrupted should return false")
	}

	// Start turn 2 with parent context; superseding with turn 3 cancels turn 2.
	turn2, ctx2 := tracker.StartTurnWithContext(context.Background())
	if turn2 != "turn-2" {
		t.Fatalf("turn2 = %q, want turn-2", turn2)
	}
	if tracker.ShouldPlayAudio(audio1) {
		t.Error("stale audio1 must remain suppressed during turn-2")
	}
	audio2 := VoiceEvent{Type: VoiceEventAudioOutput, TurnID: turn2, Audio: &AudioChunk{Data: []byte("pcm2")}}
	if !tracker.ShouldPlayAudio(audio2) {
		t.Error("audio2 should be playable while turn-2 is active")
	}

	//nolint:staticcheck // intentionally testing nil parent context guard
	turn3, ctx3 := tracker.StartTurnWithContext(nil)
	if turn3 != "turn-3" {
		t.Fatalf("turn3 = %q, want turn-3", turn3)
	}
	if ctx2.Err() == nil {
		t.Error("expected turn-2 context to be cancelled when superseded by turn-3")
	}
	if tracker.ShouldPlayAudio(audio2) {
		t.Error("superseded audio2 must not play after turn-3 started")
	}

	// Complete turn 3.
	if !tracker.CompleteTurn(turn3) {
		t.Error("expected CompleteTurn(turn3) to return true")
	}
	if ctx3.Err() == nil {
		t.Error("expected turn-3 context to be cancelled on CompleteTurn")
	}
	if tracker.CompleteTurn(turn3) {
		t.Error("second CompleteTurn should return false")
	}

	// Close on idle tracker is safe and idempotent.
	tracker.Close()
	tracker.Close()
}

func TestVoiceTurnTrackerConsumerObserveAndBoundedHistory(t *testing.T) {
	consumer := NewVoiceTurnTracker("rx")
	consumer.ObserveEvent(VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t-1"})
	evtAudio := VoiceEvent{Type: VoiceEventAudioOutput, TurnID: "t-1", Audio: &AudioChunk{Data: []byte("a")}}
	if !consumer.ShouldPlayAudio(evtAudio) {
		t.Fatal("expected t-1 audio playable after TurnStarted")
	}

	consumer.ObserveEvent(VoiceEvent{
		Type:         VoiceEventTurnInterrupted,
		TurnID:       "t-1",
		Interruption: &VoiceInterruption{CancelledTurnID: "t-1", Reason: VoiceInterruptionBargeIn},
	})
	if consumer.ShouldPlayAudio(evtAudio) {
		t.Fatal("expected t-1 audio rejected after TurnInterrupted")
	}
	// Replaying TurnStarted for already-cancelled turn must not re-enable it.
	consumer.ObserveEvent(VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t-1"})
	if consumer.ShouldPlayAudio(evtAudio) {
		t.Fatal("cancelled t-1 must not be re-activated")
	}

	// TurnCompleted and SessionEnded clear active turn.
	consumer.ObserveEvent(VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t-2"})
	consumer.ObserveEvent(VoiceEvent{Type: VoiceEventTurnCompleted, TurnID: "t-2"})
	if consumer.ActiveTurnID() != "" {
		t.Errorf("ActiveTurnID after TurnCompleted = %q", consumer.ActiveTurnID())
	}

	consumer.ObserveEvent(VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t-3"})
	consumer.ObserveEvent(VoiceEvent{Type: VoiceEventSessionEnded})
	if consumer.ActiveTurnID() != "" {
		t.Errorf("ActiveTurnID after SessionEnded = %q", consumer.ActiveTurnID())
	}

	// Exceed maxCancelledTurnHistory to verify bounded memory pruning.
	for i := range maxCancelledTurnHistory + 50 {
		id, _ := consumer.StartTurn()
		_ = id
		_, _ = consumer.Interrupt(VoiceInterruptionBargeIn)
		_ = i
	}
	consumer.mu.Lock()
	histLen := len(consumer.cancelled)
	consumer.mu.Unlock()
	if histLen > maxCancelledTurnHistory {
		t.Errorf("cancelled history grew to %d, max allowed %d", histLen, maxCancelledTurnHistory)
	}
}

func TestVoiceTurnTrackerConcurrentRace(t *testing.T) {
	tracker := NewVoiceTurnTracker("race")
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range 25 {
				turnID, _ := tracker.StartTurn()
				_ = tracker.ShouldPlayAudio(VoiceEvent{
					Type:   VoiceEventAudioOutput,
					TurnID: turnID,
					Audio:  &AudioChunk{Data: []byte("x")},
				})
				if id%2 == 0 {
					_, _ = tracker.Interrupt(VoiceInterruptionBargeIn)
				} else {
					_ = tracker.CompleteTurn(turnID)
				}
			}
		}(i)
	}
	wg.Wait()
	tracker.Close()
}

func TestVoiceEventEmitterLifecycleBackpressureAndRace(t *testing.T) {
	t.Run("stamping sequencing and clean close", func(t *testing.T) {
		bus := NewVoiceEventEmitter("sess-42", 4)
		if err := bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventSessionStarted, State: VoiceSessionStateActive}); err != nil {
			t.Fatalf("Emit(1) err = %v", err)
		}
		//nolint:staticcheck // intentionally testing nil context guard
		if err := bus.Emit(nil, VoiceEvent{Type: VoiceEventError, Err: errors.New("soft warning")}); err != nil {
			t.Fatalf("Emit(2) err = %v", err)
		}
		bus.Close()
		bus.Close() // idempotent

		if err := bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventSessionEnded}); !errors.Is(err, ErrVoiceSessionClosed) {
			t.Errorf("Emit after Close err = %v, want ErrVoiceSessionClosed", err)
		}

		var got []VoiceEvent
		for evt := range bus.Events() {
			got = append(got, evt)
		}
		if err := <-bus.Err(); err != nil {
			t.Fatalf("expected nil terminal err, got %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(events) = %d, want 2", len(got))
		}
		if got[0].SessionID != "sess-42" || got[0].Sequence != 1 || got[0].Timestamp.IsZero() {
			t.Errorf("unexpected event[0]: %+v", got[0])
		}
		if got[1].Sequence != 2 || got[1].ErrorMessage != "soft warning" {
			t.Errorf("unexpected event[1]: %+v", got[1])
		}
	})

	t.Run("bounded backpressure unblocks on ctx cancel", func(t *testing.T) {
		bus := NewVoiceEventEmitter("sess-bp", 1)
		defer bus.Close()

		if err := bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventSessionStarted}); err != nil {
			t.Fatalf("Emit(fill) err = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := bus.Emit(ctx, VoiceEvent{Type: VoiceEventSessionState, State: VoiceSessionStateListening})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked Emit err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("bounded backpressure unblocks on Fail without deadlock", func(t *testing.T) {
		bus := NewVoiceEventEmitter("sess-fail", 1)
		if err := bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventSessionStarted}); err != nil {
			t.Fatalf("Emit(fill) err = %v", err)
		}

		emitErrCh := make(chan error, 1)
		go func() {
			emitErrCh <- bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventSessionState, State: VoiceSessionStateActive})
		}()
		time.Sleep(10 * time.Millisecond)

		boom := errors.New("upstream closed")
		bus.Fail(boom)

		select {
		case err := <-emitErrCh:
			if !errors.Is(err, ErrVoiceSessionClosed) {
				t.Errorf("unblocked Emit err = %v, want ErrVoiceSessionClosed", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for blocked Emit to unblock on Fail")
		}

		for range bus.Events() {
		}
		if err := <-bus.Err(); !errors.Is(err, boom) {
			t.Fatalf("terminal Err() = %v, want %v", err, boom)
		}
	})

	t.Run("concurrent Emit and Close race safety", func(t *testing.T) {
		bus := NewVoiceEventEmitter("sess-race", 2)
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 50 {
					_ = bus.Emit(context.Background(), VoiceEvent{
						Type:   VoiceEventAgentTextDelta,
						TurnID: "t-1",
						Text:   "hello",
					})
				}
			}()
		}
		// Consumer drains concurrently while Close races with emitters.
		doneDrain := make(chan struct{})
		go func() {
			defer close(doneDrain)
			for range bus.Events() {
			}
		}()

		time.Sleep(5 * time.Millisecond)
		bus.Close()
		wg.Wait()
		<-doneDrain
	})
}

func TestAsVoiceSessionProviderAndClientStartVoiceSession(t *testing.T) {
	sess := &noopVoiceSession{
		id:    "vs-1",
		state: VoiceSessionStateActive,
		bus:   NewVoiceEventEmitter("vs-1", 4),
	}
	p := &voiceSessionTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "voice-provider"},
		start: func(_ context.Context, cfg VoiceSessionConfig) (VoiceSession, error) {
			return sess, nil
		},
	}
	wrapped := &unwrapTestWrapper{Provider: p}

	if got, ok := AsVoiceSessionProvider(p); !ok || got != p {
		t.Fatalf("AsVoiceSessionProvider(direct) = (%v, %v)", got, ok)
	}
	if got, ok := AsVoiceSessionProvider(wrapped); !ok || got != p {
		t.Fatalf("AsVoiceSessionProvider(wrapped) = (%v, %v)", got, ok)
	}
	if got, ok := AsVoiceSessionProvider(&capabilityBaseProvider{id: "base"}); ok || got != nil {
		t.Fatalf("AsVoiceSessionProvider(base) = (%v, %v), want nil, false", got, ok)
	}
	if got, ok := AsVoiceSessionProvider(nil); ok || got != nil {
		t.Fatalf("AsVoiceSessionProvider(nil) = (%v, %v), want nil, false", got, ok)
	}

	// Client success.
	gotSess, err := NewClient(p).StartVoiceSession(context.Background(), VoiceSessionConfig{
		InputFormat:  AudioFormatPCM16000,
		OutputFormat: AudioFormatPCM16000,
	})
	if err != nil || gotSess != sess {
		t.Fatalf("StartVoiceSession() = (%v, %v), want sess", gotSess, err)
	}
	if gotSess.ID() != "vs-1" || gotSess.State() != VoiceSessionStateActive {
		t.Errorf("unexpected session metadata: id=%q state=%q", gotSess.ID(), gotSess.State())
	}
	_ = gotSess.Close()

	// Nil ctx, invalid cfg, unsupported provider, rate limiter error.
	//nolint:staticcheck // intentionally testing nil context guard
	if _, err := NewClient(p).StartVoiceSession(nil, VoiceSessionConfig{}); err == nil {
		t.Error("expected error for nil ctx")
	}
	if _, err := NewClient(p).StartVoiceSession(context.Background(), VoiceSessionConfig{EventBufferSize: -1}); !errors.Is(err, ErrInvalidVoiceSessionConfig) {
		t.Errorf("invalid cfg err = %v, want ErrInvalidVoiceSessionConfig", err)
	}
	if _, err := NewClient(&capabilityBaseProvider{id: "base"}).StartVoiceSession(context.Background(), VoiceSessionConfig{}); !errors.Is(err, ErrNotSupported) {
		t.Errorf("unsupported err = %v, want ErrNotSupported", err)
	}

	limiterErr := errors.New("rate limited")
	if _, err := NewClient(p, WithRateLimiter(speechRateLimiter{wait: func(context.Context) error { return limiterErr }})).
		StartVoiceSession(context.Background(), VoiceSessionConfig{}); !errors.Is(err, limiterErr) {
		t.Errorf("rate limiter err = %v, want %v", err, limiterErr)
	}
}
