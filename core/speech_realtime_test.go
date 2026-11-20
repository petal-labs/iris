package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
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

type realtimeTTSTestProvider struct {
	*capabilityBaseProvider
	start func(context.Context, RealtimeSynthesisConfig) (RealtimeSynthesisSession, error)
}

func (p *realtimeTTSTestProvider) StartRealtimeSynthesis(ctx context.Context, cfg RealtimeSynthesisConfig) (RealtimeSynthesisSession, error) {
	if p.start == nil {
		return nil, ErrNotSupported
	}
	return p.start(ctx, cfg)
}

type noopRealtimeTTSSession struct {
	audio chan AudioChunk
	errCh chan error
}

func (s *noopRealtimeTTSSession) SendText(context.Context, string) error { return nil }
func (s *noopRealtimeTTSSession) Flush(context.Context) error            { return nil }
func (s *noopRealtimeTTSSession) Finish(context.Context) error           { return nil }
func (s *noopRealtimeTTSSession) Audio() <-chan AudioChunk               { return s.audio }
func (s *noopRealtimeTTSSession) Err() <-chan error                      { return s.errCh }
func (s *noopRealtimeTTSSession) Close() error                           { return nil }

func TestRealtimeSynthesisConfigValidationAndString(t *testing.T) {
	stab := 0.5
	badStab := 1.5
	valid := RealtimeSynthesisConfig{
		Model:               "eleven_flash_v2_5",
		VoiceID:             "voice-123",
		Format:              AudioFormatPCM16000,
		Language:            "en",
		VoiceSettings:       &VoiceSettings{Stability: &stab},
		ChunkLengthSchedule: []int{120, 160, 250, 290},
		AutoMode:            true,
		InactivityTimeout:   30 * time.Second,
		SyncAlignment:       true,
		BufferSize:          16,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid.Validate() = %v", err)
	}
	if !strings.Contains(valid.String(), "voice-123") || !strings.Contains(fmt.Sprintf("%#v", valid), "core.RealtimeSynthesisConfig") {
		t.Errorf("unexpected String/GoString: %q / %#v", valid.String(), valid)
	}

	// Zero format is allowed (provider default).
	if err := (RealtimeSynthesisConfig{VoiceID: "v1"}).Validate(); err != nil {
		t.Fatalf("minimal config Validate() = %v", err)
	}

	invalid := []struct {
		name string
		cfg  RealtimeSynthesisConfig
	}{
		{"missing voice id", RealtimeSynthesisConfig{}},
		{"blank voice id", RealtimeSynthesisConfig{VoiceID: "   "}},
		{"invalid format", RealtimeSynthesisConfig{VoiceID: "v1", Format: AudioFormat{Encoding: "bogus"}}},
		{"invalid voice settings", RealtimeSynthesisConfig{VoiceID: "v1", VoiceSettings: &VoiceSettings{Stability: &badStab}}},
		{"non-positive chunk schedule", RealtimeSynthesisConfig{VoiceID: "v1", ChunkLengthSchedule: []int{120, 0}}},
		{"negative inactivity timeout", RealtimeSynthesisConfig{VoiceID: "v1", InactivityTimeout: -time.Second}},
		{"negative buffer size", RealtimeSynthesisConfig{VoiceID: "v1", BufferSize: -1}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); !errors.Is(err, ErrInvalidSpeechRequest) {
				t.Errorf("Validate() = %v, want ErrInvalidSpeechRequest", err)
			}
		})
	}
}

func TestAsRealtimeSpeechSynthesizerAndClient(t *testing.T) {
	sess := &noopRealtimeTTSSession{
		audio: make(chan AudioChunk),
		errCh: make(chan error),
	}
	p := &realtimeTTSTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
		start: func(_ context.Context, cfg RealtimeSynthesisConfig) (RealtimeSynthesisSession, error) {
			return sess, nil
		},
	}
	wrapped := &unwrapTestWrapper{Provider: p}

	if got, ok := AsRealtimeSpeechSynthesizer(p); !ok || got != p {
		t.Fatalf("AsRealtimeSpeechSynthesizer(direct) = (%v, %v)", got, ok)
	}
	if got, ok := AsRealtimeSpeechSynthesizer(wrapped); !ok || got != p {
		t.Fatalf("AsRealtimeSpeechSynthesizer(wrapped) = (%v, %v)", got, ok)
	}
	if got, ok := AsRealtimeSpeechSynthesizer(&capabilityBaseProvider{id: "base"}); ok || got != nil {
		t.Fatalf("AsRealtimeSpeechSynthesizer(base) = (%v, %v), want nil, false", got, ok)
	}

	gotSess, err := NewClient(p).StartRealtimeSynthesis(context.Background(), RealtimeSynthesisConfig{
		VoiceID: "voice-1",
	})
	if err != nil || gotSess != sess {
		t.Fatalf("StartRealtimeSynthesis() = (%v, %v), want sess", gotSess, err)
	}

	// Nil ctx, invalid cfg, unsupported provider, rate limiter error
	//nolint:staticcheck // intentionally testing nil context guard
	if _, err := NewClient(p).StartRealtimeSynthesis(nil, RealtimeSynthesisConfig{VoiceID: "voice-1"}); err == nil {
		t.Error("expected error for nil ctx")
	}
	if _, err := NewClient(p).StartRealtimeSynthesis(context.Background(), RealtimeSynthesisConfig{}); !errors.Is(err, ErrInvalidSpeechRequest) {
		t.Errorf("invalid cfg err = %v, want ErrInvalidSpeechRequest", err)
	}
	if _, err := NewClient(&capabilityBaseProvider{id: "base"}).StartRealtimeSynthesis(context.Background(), RealtimeSynthesisConfig{VoiceID: "voice-1"}); !errors.Is(err, ErrNotSupported) {
		t.Errorf("unsupported err = %v, want ErrNotSupported", err)
	}

	limiterErr := errors.New("rate limited")
	if _, err := NewClient(p, WithRateLimiter(speechRateLimiter{wait: func(context.Context) error { return limiterErr }})).
		StartRealtimeSynthesis(context.Background(), RealtimeSynthesisConfig{VoiceID: "voice-1"}); !errors.Is(err, limiterErr) {
		t.Errorf("rate limiter err = %v, want %v", err, limiterErr)
	}
}
