package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type realtimeSTTTestProvider struct {
	*capabilityBaseProvider
	start func(context.Context, RealtimeTranscriptionConfig) (RealtimeTranscriptionSession, error)
}

func (p *realtimeSTTTestProvider) StartRealtimeTranscription(ctx context.Context, cfg RealtimeTranscriptionConfig) (RealtimeTranscriptionSession, error) {
	if p.start == nil {
		return nil, ErrNotSupported
	}
	return p.start(ctx, cfg)
}

type noopRealtimeSession struct {
	events chan RealtimeTranscriptEvent
	errCh  chan error
}

func (s *noopRealtimeSession) SendAudio(context.Context, AudioChunk) error { return nil }
func (s *noopRealtimeSession) Commit(context.Context) error                { return nil }
func (s *noopRealtimeSession) Events() <-chan RealtimeTranscriptEvent      { return s.events }
func (s *noopRealtimeSession) Err() <-chan error                           { return s.errCh }
func (s *noopRealtimeSession) Close() error                                { return nil }

func TestRealtimeTranscriptionConfigValidation(t *testing.T) {
	valid := RealtimeTranscriptionConfig{
		Model:             "scribe_v2_realtime",
		Format:            AudioFormatPCM16000,
		Language:          "en",
		CommitStrategy:    CommitStrategyVAD,
		IncludeTimestamps: true,
		MaxChunkBytes:     16384,
		EventBufferSize:   32,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid.Validate() = %v", err)
	}

	if !CommitStrategyDefault.IsValid() || !CommitStrategyManual.IsValid() || !CommitStrategyVAD.IsValid() {
		t.Error("expected default, manual, and vad CommitStrategy values to be valid")
	}
	if CommitStrategy("auto").IsValid() {
		t.Error("expected unknown CommitStrategy to be invalid")
	}

	invalid := []struct {
		name string
		cfg  RealtimeTranscriptionConfig
	}{
		{"missing format", RealtimeTranscriptionConfig{}},
		{"invalid commit strategy", RealtimeTranscriptionConfig{Format: AudioFormatPCM16000, CommitStrategy: "bad"}},
		{"negative max chunk bytes", RealtimeTranscriptionConfig{Format: AudioFormatPCM16000, MaxChunkBytes: -1}},
		{"negative event buffer size", RealtimeTranscriptionConfig{Format: AudioFormatPCM16000, EventBufferSize: -1}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); !errors.Is(err, ErrInvalidTranscriptionRequest) {
				t.Errorf("Validate() = %v, want ErrInvalidTranscriptionRequest", err)
			}
		})
	}
}

func TestRealtimeTranscriptEventPrivacy(t *testing.T) {
	evt := RealtimeTranscriptEvent{
		Type:      RealtimeTranscriptEventCommitted,
		SessionID: "sess-1",
		IsFinal:   true,
		Transcript: &TranscriptionResponse{
			Text:    "TOP_SECRET_REALTIME_UTTERANCE",
			IsFinal: true,
		},
	}
	for _, out := range []string{evt.String(), fmt.Sprintf("%#v", evt)} {
		if strings.Contains(out, "TOP_SECRET_REALTIME_UTTERANCE") {
			t.Errorf("RealtimeTranscriptEvent leaked transcript text: %s", out)
		}
	}

	emptyEvt := RealtimeTranscriptEvent{Type: RealtimeTranscriptEventSessionStarted, SessionID: "sess-2"}
	if !strings.Contains(emptyEvt.String(), "sess-2") {
		t.Errorf("String() = %s, want session ID", emptyEvt.String())
	}
}

func TestAsRealtimeSpeechRecognizerAndClient(t *testing.T) {
	sess := &noopRealtimeSession{
		events: make(chan RealtimeTranscriptEvent),
		errCh:  make(chan error),
	}
	p := &realtimeSTTTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
		start: func(_ context.Context, cfg RealtimeTranscriptionConfig) (RealtimeTranscriptionSession, error) {
			return sess, nil
		},
	}
	wrapped := &unwrapTestWrapper{Provider: p}

	if got, ok := AsRealtimeSpeechRecognizer(p); !ok || got != p {
		t.Fatalf("AsRealtimeSpeechRecognizer(direct) = (%v, %v)", got, ok)
	}
	if got, ok := AsRealtimeSpeechRecognizer(wrapped); !ok || got != p {
		t.Fatalf("AsRealtimeSpeechRecognizer(wrapped) = (%v, %v)", got, ok)
	}
	if got, ok := AsRealtimeSpeechRecognizer(&capabilityBaseProvider{id: "base"}); ok || got != nil {
		t.Fatalf("AsRealtimeSpeechRecognizer(base) = (%v, %v), want nil, false", got, ok)
	}

	// Client success
	gotSess, err := NewClient(p).StartRealtimeTranscription(context.Background(), RealtimeTranscriptionConfig{
		Format: AudioFormatPCM16000,
	})
	if err != nil || gotSess != sess {
		t.Fatalf("StartRealtimeTranscription() = (%v, %v), want sess", gotSess, err)
	}

	// Nil ctx, invalid cfg, unsupported provider, rate limiter error
	//nolint:staticcheck // intentionally testing nil context guard
	if _, err := NewClient(p).StartRealtimeTranscription(nil, RealtimeTranscriptionConfig{Format: AudioFormatPCM16000}); err == nil {
		t.Error("expected error for nil ctx")
	}
	if _, err := NewClient(p).StartRealtimeTranscription(context.Background(), RealtimeTranscriptionConfig{}); !errors.Is(err, ErrInvalidTranscriptionRequest) {
		t.Errorf("invalid cfg err = %v, want ErrInvalidTranscriptionRequest", err)
	}
	if _, err := NewClient(&capabilityBaseProvider{id: "base"}).StartRealtimeTranscription(context.Background(), RealtimeTranscriptionConfig{Format: AudioFormatPCM16000}); !errors.Is(err, ErrNotSupported) {
		t.Errorf("unsupported err = %v, want ErrNotSupported", err)
	}

	limiterErr := errors.New("rate limited")
	if _, err := NewClient(p, WithRateLimiter(speechRateLimiter{wait: func(context.Context) error { return limiterErr }})).
		StartRealtimeTranscription(context.Background(), RealtimeTranscriptionConfig{Format: AudioFormatPCM16000}); !errors.Is(err, limiterErr) {
		t.Errorf("rate limiter err = %v, want %v", err, limiterErr)
	}
}
