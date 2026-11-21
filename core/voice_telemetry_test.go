package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordingVoiceTelemetryHook struct {
	mu          sync.Mutex
	reqStarts   []VoiceRequestStartEvent
	firstChunks []VoiceFirstChunkEvent
	reqEnds     []VoiceRequestEndEvent
	sessStarts  []VoiceSessionStartEvent
	turnEnds    []VoiceTurnEndEvent
	sessEnds    []VoiceSessionEndEvent
}

func (h *recordingVoiceTelemetryHook) OnVoiceRequestStart(ctx context.Context, e VoiceRequestStartEvent) context.Context {
	h.mu.Lock()
	h.reqStarts = append(h.reqStarts, e)
	h.mu.Unlock()
	return ctx
}

func (h *recordingVoiceTelemetryHook) OnVoiceFirstChunk(_ context.Context, e VoiceFirstChunkEvent) {
	h.mu.Lock()
	h.firstChunks = append(h.firstChunks, e)
	h.mu.Unlock()
}

func (h *recordingVoiceTelemetryHook) OnVoiceRequestEnd(_ context.Context, e VoiceRequestEndEvent) {
	h.mu.Lock()
	h.reqEnds = append(h.reqEnds, e)
	h.mu.Unlock()
}

func (h *recordingVoiceTelemetryHook) OnVoiceSessionStart(ctx context.Context, e VoiceSessionStartEvent) context.Context {
	h.mu.Lock()
	h.sessStarts = append(h.sessStarts, e)
	h.mu.Unlock()
	return ctx
}

func (h *recordingVoiceTelemetryHook) OnVoiceTurnEnd(_ context.Context, e VoiceTurnEndEvent) {
	h.mu.Lock()
	h.turnEnds = append(h.turnEnds, e)
	h.mu.Unlock()
}

func (h *recordingVoiceTelemetryHook) OnVoiceSessionEnd(_ context.Context, e VoiceSessionEndEvent) {
	h.mu.Lock()
	h.sessEnds = append(h.sessEnds, e)
	h.mu.Unlock()
}

func TestClientVoiceTelemetrySynthesizeStreamAndTranscribe(t *testing.T) {
	hook := &recordingVoiceTelemetryHook{}

	p := &speechTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
		synthesize: func(_ context.Context, req *SpeechRequest) (*SpeechResponse, error) {
			return &SpeechResponse{
				Model:   req.Model,
				VoiceID: req.VoiceID,
				Audio:   AudioData{Data: []byte("audio-bytes"), Format: AudioFormatMP3_44100_128},
				Usage:   SpeechUsage{InputCharacters: len(req.Text), OutputBytes: 11},
			}, nil
		},
		streamSynthesize: func(_ context.Context, req *SpeechRequest) (*SpeechStream, error) {
			ch := make(chan AudioChunk, 3)
			errCh := make(chan error, 1)
			finalCh := make(chan *SpeechResponse, 1)
			ch <- AudioChunk{Data: []byte("chunk1"), Sequence: 0}
			ch <- AudioChunk{Data: []byte("chunk2"), Sequence: 1}
			ch <- AudioChunk{Sequence: 2, IsFinal: true}
			close(ch)
			finalCh <- &SpeechResponse{Usage: SpeechUsage{InputCharacters: 20, OutputBytes: 12}}
			close(finalCh)
			close(errCh)
			return &SpeechStream{Ch: ch, Err: errCh, Final: finalCh}, nil
		},
		transcribe: func(_ context.Context, _ *TranscriptionRequest) (*TranscriptionResponse, error) {
			return &TranscriptionResponse{
				Text:     "SECRET_TRANSCRIPT_NEVER_IN_METRICS",
				Duration: 2 * time.Second,
				IsFinal:  true,
			}, nil
		},
	}

	client := NewClient(p, WithVoiceTelemetry(hook))

	// 1. Synthesize
	if _, err := client.Synthesize(context.Background(), &SpeechRequest{
		Model:   "eleven_flash_v2_5",
		VoiceID: "v1",
		Text:    "SECRET_SYNTH_TEXT",
	}); err != nil {
		t.Fatalf("Synthesize() err = %v", err)
	}

	// 2. StreamSynthesize
	stream, err := client.StreamSynthesize(context.Background(), &SpeechRequest{
		Model:   "eleven_flash_v2_5",
		VoiceID: "v1",
		Text:    "SECRET_STREAM_TEXT",
	})
	if err != nil {
		t.Fatalf("StreamSynthesize() err = %v", err)
	}
	if _, err := DrainSpeechStream(context.Background(), stream); err != nil {
		t.Fatalf("DrainSpeechStream() err = %v", err)
	}

	// 3. Transcribe
	if _, err := client.Transcribe(context.Background(), &TranscriptionRequest{
		Model: "scribe_v2",
		Audio: &AudioData{Data: []byte("raw-audio-input"), Format: AudioFormatMP3_44100_128},
	}); err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}

	// Wait briefly for stream telemetry goroutine completion.
	deadline := time.After(time.Second)
	for {
		hook.mu.Lock()
		n := len(hook.reqEnds)
		hook.mu.Unlock()
		if n == 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for 3 VoiceRequestEndEvent callbacks, got %d", n)
		case <-time.After(5 * time.Millisecond):
		}
	}

	hook.mu.Lock()
	defer hook.mu.Unlock()
	if len(hook.reqStarts) != 3 || len(hook.firstChunks) != 1 || len(hook.reqEnds) != 3 {
		t.Fatalf("counts mismatch: starts=%d firstChunks=%d ends=%d", len(hook.reqStarts), len(hook.firstChunks), len(hook.reqEnds))
	}
	if hook.firstChunks[0].ChunkBytes != 6 || hook.firstChunks[0].FirstChunkLatency() < 0 {
		t.Errorf("unexpected firstChunk event: %+v", hook.firstChunks[0])
	}

	var sawSynth, sawStream, sawTranscribe bool
	for _, end := range hook.reqEnds {
		switch end.Operation {
		case VoiceOperationSynthesize:
			sawSynth = true
			if end.OutputBytes != 11 || end.InputCharacters != len("SECRET_SYNTH_TEXT") || end.Duration() < 0 {
				t.Errorf("unexpected synth end event: %+v", end)
			}
		case VoiceOperationStreamSynthesize:
			sawStream = true
			if end.Chunks != 2 || end.OutputBytes != 12 || end.InputCharacters != 20 {
				t.Errorf("unexpected stream end event: %+v", end)
			}
		case VoiceOperationTranscribe:
			sawTranscribe = true
			if end.InputBytes != int64(len("raw-audio-input")) || end.AudioDuration != 2*time.Second {
				t.Errorf("unexpected transcribe end event: %+v", end)
			}
		}
	}
	if !sawSynth || !sawStream || !sawTranscribe {
		t.Errorf("missing operations: synth=%t stream=%t transcribe=%t", sawSynth, sawStream, sawTranscribe)
	}
}

func TestObserveVoiceSessionMetricsInterruptionsAndFullBufferClose(t *testing.T) {
	hook := &recordingVoiceTelemetryHook{}

	bus := NewVoiceEventEmitter("vs-obs", 8)
	inner := &noopVoiceSession{id: "vs-obs", state: VoiceSessionStateActive, bus: bus}

	// Noop / nil pass-through.
	if ObserveVoiceSession(context.Background(), inner, NoopVoiceTelemetryHook{}, VoiceSessionObserverMeta{}) != inner {
		t.Error("expected NoopVoiceTelemetryHook to return inner unchanged")
	}

	p := &voiceSessionTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
		start: func(_ context.Context, _ VoiceSessionConfig) (VoiceSession, error) {
			return inner, nil
		},
	}

	client := NewClient(p, WithVoiceTelemetry(hook))
	sess, err := client.StartVoiceSession(context.Background(), VoiceSessionConfig{
		Model:   "gpt-5",
		VoiceID: "v1",
	})
	if err != nil {
		t.Fatalf("StartVoiceSession() err = %v", err)
	}

	if err := sess.SendAudio(context.Background(), AudioChunk{Data: []byte("123456")}); err != nil {
		t.Fatalf("SendAudio() err = %v", err)
	}

	// Turn 1: completed with audio + tool call.
	_ = bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t1"})
	_ = bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventToolCall, TurnID: "t1", ToolCall: &ToolCall{ID: "c1", Name: "fn"}})
	_ = bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventAudioOutput, TurnID: "t1", Audio: &AudioChunk{Data: []byte("abcd")}})
	_ = bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventAudioOutput, TurnID: "t1", Audio: &AudioChunk{Data: []byte("ef")}})
	_ = bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventTurnCompleted, TurnID: "t1"})

	// Turn 2: interrupted by barge-in.
	_ = bus.Emit(context.Background(), VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t2"})
	_ = bus.Emit(context.Background(), VoiceEvent{
		Type:         VoiceEventTurnInterrupted,
		TurnID:       "t2",
		Interruption: &VoiceInterruption{CancelledTurnID: "t2", Reason: VoiceInterruptionBargeIn},
	})

	boom := errors.New("session disconnected")
	bus.Fail(boom)

	for range sess.Events() {
	}
	if err := <-sess.Err(); !errors.Is(err, boom) {
		t.Fatalf("Err() = %v, want %v", err, boom)
	}
	_ = sess.Close()

	hook.mu.Lock()
	if len(hook.sessStarts) != 1 || len(hook.sessEnds) != 1 || len(hook.turnEnds) != 2 || len(hook.firstChunks) != 1 {
		hook.mu.Unlock()
		t.Fatalf("counts mismatch: sessStarts=%d sessEnds=%d turnEnds=%d firstChunks=%d",
			len(hook.sessStarts), len(hook.sessEnds), len(hook.turnEnds), len(hook.firstChunks))
	}

	end := hook.sessEnds[0]
	hook.mu.Unlock()
	if end.Turns != 2 || end.Interruptions != 1 || end.ToolCalls != 1 ||
		end.InputAudioBytes != 6 || end.OutputAudioBytes != 6 || end.ErrorCount != 1 || !errors.Is(end.Err, boom) {
		t.Errorf("unexpected VoiceSessionEndEvent: %+v", end)
	}

	// Verify full-buffer Close does not deadlock.
	bus2 := NewVoiceEventEmitter("vs-full", 1)
	inner2 := &noopVoiceSession{id: "vs-full", state: VoiceSessionStateActive, bus: bus2}
	obs2 := ObserveVoiceSession(context.Background(), inner2, hook, VoiceSessionObserverMeta{Provider: "test", Mode: "pipeline"})
	_ = bus2.Emit(context.Background(), VoiceEvent{Type: VoiceEventTurnStarted, TurnID: "t1"})
	_ = bus2.Emit(context.Background(), VoiceEvent{Type: VoiceEventTurnCompleted, TurnID: "t1"})

	done := make(chan struct{})
	go func() {
		_ = obs2.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ObserveVoiceSession.Close() deadlocked on full unconsumed Events() buffer")
	}
}
