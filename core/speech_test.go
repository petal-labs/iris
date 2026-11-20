package core

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type speechTestProvider struct {
	*capabilityBaseProvider
	models           []ModelInfo
	features         map[Feature]bool
	synthesize       func(context.Context, *SpeechRequest) (*SpeechResponse, error)
	streamSynthesize func(context.Context, *SpeechRequest) (*SpeechStream, error)
	transcribe       func(context.Context, *TranscriptionRequest) (*TranscriptionResponse, error)
	synthCalls       int
	streamCalls      int
	transcribeCalls  int
}

func (p *speechTestProvider) Models() []ModelInfo { return p.models }

func (p *speechTestProvider) Supports(f Feature) bool { return p.features[f] }

func (p *speechTestProvider) Synthesize(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error) {
	p.synthCalls++
	if p.synthesize == nil {
		return &SpeechResponse{}, nil
	}
	return p.synthesize(ctx, req)
}

func (p *speechTestProvider) StreamSynthesize(ctx context.Context, req *SpeechRequest) (*SpeechStream, error) {
	p.streamCalls++
	if p.streamSynthesize == nil {
		return nil, ErrNotSupported
	}
	return p.streamSynthesize(ctx, req)
}

func (p *speechTestProvider) Transcribe(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResponse, error) {
	p.transcribeCalls++
	if p.transcribe == nil {
		return &TranscriptionResponse{}, nil
	}
	return p.transcribe(ctx, req)
}

type speechTelemetryHook struct {
	mu     sync.Mutex
	starts []RequestStartEvent
	ends   []RequestEndEvent
	doneCh chan struct{}
}

func newSpeechTelemetryHook() *speechTelemetryHook {
	return &speechTelemetryHook{doneCh: make(chan struct{}, 4)}
}

func (h *speechTelemetryHook) OnRequestStart(e RequestStartEvent) {
	h.mu.Lock()
	h.starts = append(h.starts, e)
	h.mu.Unlock()
}

func (h *speechTelemetryHook) OnRequestEnd(e RequestEndEvent) {
	h.mu.Lock()
	h.ends = append(h.ends, e)
	h.mu.Unlock()
	select {
	case h.doneCh <- struct{}{}:
	default:
	}
}

func (h *speechTelemetryHook) waitEnd(t *testing.T) RequestEndEvent {
	t.Helper()
	select {
	case <-h.doneCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for telemetry OnRequestEnd")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ends[len(h.ends)-1]
}

func (h *speechTelemetryHook) counts() (starts, ends int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.starts), len(h.ends)
}

type speechContextKey struct{}

type contextualSpeechTelemetryHook struct {
	*speechTelemetryHook
	mu            sync.Mutex
	endSawContext bool
}

func (h *contextualSpeechTelemetryHook) OnRequestStartWithContext(ctx context.Context, e RequestStartEvent) context.Context {
	h.OnRequestStart(e)
	return context.WithValue(ctx, speechContextKey{}, "speech-trace")
}

func (h *contextualSpeechTelemetryHook) OnRequestEndWithContext(ctx context.Context, e RequestEndEvent) {
	h.mu.Lock()
	h.endSawContext = ctx.Value(speechContextKey{}) == "speech-trace"
	h.mu.Unlock()
	h.OnRequestEnd(e)
}

type speechNoRetryPolicy struct{}

func (speechNoRetryPolicy) NextDelay(int, error) (time.Duration, bool) { return 0, false }

type speechRateLimiter struct {
	wait func(context.Context) error
}

func (l speechRateLimiter) Allow() bool                    { return true }
func (l speechRateLimiter) Wait(ctx context.Context) error { return l.wait(ctx) }
func (l speechRateLimiter) Reset()                         {}

func validSpeechRequest() *SpeechRequest {
	return &SpeechRequest{
		Model:   "eleven_flash_v2_5",
		VoiceID: "voice_1",
		Text:    "Hello world",
		Format:  AudioFormatMP3_44100_128,
	}
}

func validAudioTranscriptionRequest() *TranscriptionRequest {
	return &TranscriptionRequest{
		Model: "scribe_v2",
		Audio: &AudioData{
			Data:   []byte("wav-bytes"),
			Format: AudioFormatWAV16000,
		},
	}
}

func TestSpeechFeatureConstants(t *testing.T) {
	tests := []struct {
		got  Feature
		want Feature
	}{
		{FeatureSpeechSynthesis, "speech_synthesis"},
		{FeatureSpeechSynthesisStreaming, "speech_synthesis_streaming"},
		{FeatureSpeechRecognition, "speech_recognition"},
		{FeatureSpeechRecognitionRealtime, "speech_recognition_realtime"},
		{FeatureVoiceSession, "voice_session"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("Feature = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestSpeechCapabilityDiscovery(t *testing.T) {
	p := &speechTestProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"}}
	wrapped := &unwrapTestWrapper{Provider: &unwrapTestWrapper{Provider: p}}
	base := &capabilityBaseProvider{id: "chat-only"}

	if got, ok := AsSpeechSynthesizer(p); !ok || got != p {
		t.Fatalf("AsSpeechSynthesizer(direct) = (%v, %v), want provider, true", got, ok)
	}
	if got, ok := AsSpeechSynthesizer(wrapped); !ok || got != p {
		t.Fatalf("AsSpeechSynthesizer(wrapped) = (%v, %v), want underlying provider, true", got, ok)
	}
	if got, ok := AsSpeechSynthesizer(base); ok || got != nil {
		t.Fatalf("AsSpeechSynthesizer(base) = (%v, %v), want nil, false", got, ok)
	}
	if got, ok := AsSpeechSynthesizer(nil); ok || got != nil {
		t.Fatalf("AsSpeechSynthesizer(nil) = (%v, %v), want nil, false", got, ok)
	}
	if got, ok := AsSpeechSynthesizer(&cyclicUnwrapTestWrapper{}); ok || got != nil {
		t.Fatalf("AsSpeechSynthesizer(cycle) = (%v, %v), want nil, false", got, ok)
	}

	if got, ok := AsSpeechRecognizer(p); !ok || got != p {
		t.Fatalf("AsSpeechRecognizer(direct) = (%v, %v), want provider, true", got, ok)
	}
	if got, ok := AsSpeechRecognizer(wrapped); !ok || got != p {
		t.Fatalf("AsSpeechRecognizer(wrapped) = (%v, %v), want underlying provider, true", got, ok)
	}
	if got, ok := AsSpeechRecognizer(base); ok || got != nil {
		t.Fatalf("AsSpeechRecognizer(base) = (%v, %v), want nil, false", got, ok)
	}
	if got, ok := AsSpeechRecognizer(nil); ok || got != nil {
		t.Fatalf("AsSpeechRecognizer(nil) = (%v, %v), want nil, false", got, ok)
	}
	if got, ok := AsSpeechRecognizer(&cyclicUnwrapTestWrapper{}); ok || got != nil {
		t.Fatalf("AsSpeechRecognizer(cycle) = (%v, %v), want nil, false", got, ok)
	}
}

func TestModelCapabilityDiscoveryHelpers(t *testing.T) {
	p := &speechTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
		models: []ModelInfo{
			{
				ID:           "eleven_flash_v2_5",
				DisplayName:  "Eleven Flash v2.5",
				Capabilities: []Feature{FeatureSpeechSynthesis, FeatureSpeechSynthesisStreaming},
			},
			{
				ID:           "scribe_v2",
				DisplayName:  "Scribe v2",
				Capabilities: []Feature{FeatureSpeechRecognition},
			},
		},
	}

	if got := ModelsWithCapability(nil, FeatureSpeechSynthesis); got != nil {
		t.Fatalf("ModelsWithCapability(nil) = %v, want nil", got)
	}
	ttsModels := ModelsWithCapability(p, FeatureSpeechSynthesis)
	if len(ttsModels) != 1 || ttsModels[0].ID != "eleven_flash_v2_5" {
		t.Fatalf("ModelsWithCapability(TTS) = %+v, want [eleven_flash_v2_5]", ttsModels)
	}
	if got := ModelsWithCapability(p, FeatureVoiceSession); len(got) != 0 {
		t.Fatalf("ModelsWithCapability(VoiceSession) = %+v, want empty", got)
	}

	if !ProviderSupportsModelFeature(p, "eleven_flash_v2_5", FeatureSpeechSynthesisStreaming) {
		t.Error("expected eleven_flash_v2_5 to support FeatureSpeechSynthesisStreaming")
	}
	if ProviderSupportsModelFeature(p, "eleven_flash_v2_5", FeatureSpeechRecognition) {
		t.Error("expected eleven_flash_v2_5 NOT to support FeatureSpeechRecognition")
	}
	if ProviderSupportsModelFeature(p, "unverified_model", FeatureSpeechSynthesis) {
		t.Error("unverified model must not be reported as supported")
	}
	if ProviderSupportsModelFeature(p, "", FeatureSpeechSynthesis) {
		t.Error("empty model ID must return false")
	}
	if ProviderSupportsModelFeature(nil, "scribe_v2", FeatureSpeechRecognition) {
		t.Error("nil provider must return false")
	}
}

func TestClientSynthesizeSuccessAndTelemetry(t *testing.T) {
	req := validSpeechRequest()
	want := &SpeechResponse{
		ID:      "gen-1",
		Model:   req.Model,
		VoiceID: req.VoiceID,
		Audio:   AudioData{Data: []byte("mp3-data"), Format: AudioFormatMP3_44100_128},
	}
	hook := &contextualSpeechTelemetryHook{speechTelemetryHook: newSpeechTelemetryHook()}
	p := &speechTestProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
		synthesize: func(ctx context.Context, got *SpeechRequest) (*SpeechResponse, error) {
			if got != req {
				t.Error("Synthesize did not forward original request")
			}
			if ctx.Value(speechContextKey{}) != "speech-trace" {
				t.Error("Synthesize did not receive contextual telemetry context")
			}
			return want, nil
		},
	}

	resp, err := NewClient(p, WithTelemetry(hook)).Synthesize(context.Background(), req)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if resp != want || p.synthCalls != 1 {
		t.Fatalf("Synthesize() = (%v, calls=%d), want response and 1 call", resp, p.synthCalls)
	}
	starts, ends := hook.counts()
	if starts != 1 || ends != 1 {
		t.Fatalf("telemetry counts = (%d, %d), want (1, 1)", starts, ends)
	}
	if !hook.endSawContext {
		t.Error("contextual telemetry end hook did not receive context")
	}
}

func TestClientSynthesizeValidationAndUnsupported(t *testing.T) {
	hook := newSpeechTelemetryHook()

	// Nil context
	//nolint:staticcheck // intentionally testing nil context guard
	if _, err := NewClient(&capabilityBaseProvider{id: "base"}, WithTelemetry(hook)).Synthesize(nil, validSpeechRequest()); err == nil {
		t.Fatal("expected error for nil context")
	}

	// Nil / invalid request
	if _, err := NewClient(&capabilityBaseProvider{id: "base"}, WithTelemetry(hook)).Synthesize(context.Background(), nil); !errors.Is(err, ErrInvalidSpeechRequest) {
		t.Fatalf("nil request err = %v, want ErrInvalidSpeechRequest", err)
	}
	if _, err := NewClient(&capabilityBaseProvider{id: "base"}, WithTelemetry(hook)).Synthesize(context.Background(), &SpeechRequest{Text: "hi"}); !errors.Is(err, ErrInvalidSpeechRequest) {
		t.Fatalf("missing voice err = %v, want ErrInvalidSpeechRequest", err)
	}

	// Unsupported provider
	if _, err := NewClient(&capabilityBaseProvider{id: "chat-only"}, WithTelemetry(hook)).Synthesize(context.Background(), validSpeechRequest()); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("unsupported provider err = %v, want ErrNotSupported", err)
	}

	// Nil provider
	if _, err := NewClient(nil, WithTelemetry(hook)).Synthesize(context.Background(), validSpeechRequest()); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("nil provider err = %v, want ErrNotSupported", err)
	}

	if starts, ends := hook.counts(); starts != 0 || ends != 0 {
		t.Fatalf("telemetry should not fire on pre-dispatch errors, got (%d, %d)", starts, ends)
	}
}

func TestClientSynthesizeRetryTimeoutRateLimitAndCancel(t *testing.T) {
	t.Run("retries transient error", func(t *testing.T) {
		p := &speechTestProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"}}
		p.synthesize = func(context.Context, *SpeechRequest) (*SpeechResponse, error) {
			if p.synthCalls == 1 {
				return nil, ErrRateLimited
			}
			return &SpeechResponse{ID: "ok"}, nil
		}
		retry := NewRetryPolicy(RetryConfig{MaxRetries: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Microsecond})
		resp, err := NewClient(p, WithRetryPolicy(retry)).Synthesize(context.Background(), validSpeechRequest())
		if err != nil || resp.ID != "ok" || p.synthCalls != 2 {
			t.Fatalf("got (%v, %v, calls=%d), want ok after 2 calls", resp, err, p.synthCalls)
		}
	})

	t.Run("cancellation during retry backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			synthesize: func(context.Context, *SpeechRequest) (*SpeechResponse, error) {
				cancel()
				return nil, ErrNetwork
			},
		}
		retry := NewRetryPolicy(RetryConfig{MaxRetries: 3, BaseDelay: time.Second, MaxDelay: time.Second})
		_, err := NewClient(p, WithRetryPolicy(retry)).Synthesize(ctx, validSpeechRequest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("client timeout remaps to ErrTimeout", func(t *testing.T) {
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			synthesize: func(ctx context.Context, _ *SpeechRequest) (*SpeechResponse, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		_, err := NewClient(p, WithTimeout(15*time.Millisecond), WithRetryPolicy(speechNoRetryPolicy{})).
			Synthesize(context.Background(), validSpeechRequest())
		if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want ErrTimeout wrapping DeadlineExceeded", err)
		}
	})

	t.Run("caller deadline preserved", func(t *testing.T) {
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			synthesize: func(ctx context.Context, _ *SpeechRequest) (*SpeechResponse, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
		defer cancel()
		_, err := NewClient(p, WithTimeout(time.Second), WithRetryPolicy(speechNoRetryPolicy{})).
			Synthesize(ctx, validSpeechRequest())
		if errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want plain context.DeadlineExceeded", err)
		}
	})

	t.Run("rate limiter error stops call", func(t *testing.T) {
		limiterErr := errors.New("rate limit wait aborted")
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			synthesize: func(context.Context, *SpeechRequest) (*SpeechResponse, error) {
				t.Fatal("provider should not be called when rate limiter fails")
				return nil, nil
			},
		}
		_, err := NewClient(p, WithRateLimiter(speechRateLimiter{wait: func(context.Context) error { return limiterErr }}), WithRetryPolicy(speechNoRetryPolicy{})).
			Synthesize(context.Background(), validSpeechRequest())
		if !errors.Is(err, limiterErr) {
			t.Fatalf("err = %v, want %v", err, limiterErr)
		}
	})
}

func TestClientStreamSynthesizeSuccessFailureAndCancellation(t *testing.T) {
	t.Run("success streams chunks and emits telemetry", func(t *testing.T) {
		hook := &contextualSpeechTelemetryHook{speechTelemetryHook: newSpeechTelemetryHook()}
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			streamSynthesize: func(ctx context.Context, _ *SpeechRequest) (*SpeechStream, error) {
				if ctx.Value(speechContextKey{}) != "speech-trace" {
					t.Error("StreamSynthesize did not receive contextual telemetry context")
				}
				ch := make(chan AudioChunk, 2)
				errCh := make(chan error, 1)
				finalCh := make(chan *SpeechResponse, 1)
				go func() {
					ch <- AudioChunk{Data: []byte("abc"), Format: AudioFormatPCM16000, Sequence: 0}
					ch <- AudioChunk{Data: []byte("def"), Sequence: 1, IsFinal: true}
					close(ch)
					finalCh <- &SpeechResponse{ID: "stream-1", Model: "eleven_flash_v2_5"}
					close(finalCh)
					close(errCh)
				}()
				return &SpeechStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		stream, err := NewClient(p, WithTelemetry(hook)).StreamSynthesize(context.Background(), validSpeechRequest())
		if err != nil {
			t.Fatalf("StreamSynthesize() error = %v", err)
		}
		resp, err := DrainSpeechStream(context.Background(), stream)
		if err != nil {
			t.Fatalf("DrainSpeechStream() error = %v", err)
		}
		if string(resp.Audio.Data) != "abcdef" || resp.ID != "stream-1" {
			t.Fatalf("unexpected drained response: %+v", resp)
		}
		endEvent := hook.waitEnd(t)
		if endEvent.Err != nil || endEvent.Provider != "elevenlabs" {
			t.Fatalf("unexpected telemetry end event: %+v", endEvent)
		}
		hook.mu.Lock()
		sawCtx := hook.endSawContext
		hook.mu.Unlock()
		if !sawCtx {
			t.Error("contextual telemetry end hook did not receive context")
		}
	})

	t.Run("validation, nil ctx, unsupported, and nil stream guard", func(t *testing.T) {
		//nolint:staticcheck // intentionally testing nil context guard
		if _, err := NewClient(&capabilityBaseProvider{id: "base"}).StreamSynthesize(nil, validSpeechRequest()); err == nil {
			t.Fatal("expected error for nil ctx")
		}
		if _, err := NewClient(&capabilityBaseProvider{id: "base"}).StreamSynthesize(context.Background(), &SpeechRequest{}); !errors.Is(err, ErrInvalidSpeechRequest) {
			t.Fatalf("invalid request err = %v, want ErrInvalidSpeechRequest", err)
		}
		if _, err := NewClient(&capabilityBaseProvider{id: "base"}).StreamSynthesize(context.Background(), validSpeechRequest()); !errors.Is(err, ErrNotSupported) {
			t.Fatalf("unsupported err = %v, want ErrNotSupported", err)
		}

		hook := newSpeechTelemetryHook()
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			streamSynthesize: func(context.Context, *SpeechRequest) (*SpeechStream, error) {
				return nil, nil
			},
		}
		if _, err := NewClient(p, WithTelemetry(hook)).StreamSynthesize(context.Background(), validSpeechRequest()); !errors.Is(err, ErrNilSpeechStream) {
			t.Fatalf("nil stream err = %v, want ErrNilSpeechStream", err)
		}
		if end := hook.waitEnd(t); !errors.Is(end.Err, ErrNilSpeechStream) {
			t.Fatalf("telemetry end err = %v, want ErrNilSpeechStream", end.Err)
		}
	})

	t.Run("setup error and rate limit error", func(t *testing.T) {
		hook := newSpeechTelemetryHook()
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			// default streamSynthesize returns ErrNotSupported (sync-only synthesizer)
		}
		if _, err := NewClient(p, WithTelemetry(hook)).StreamSynthesize(context.Background(), validSpeechRequest()); !errors.Is(err, ErrNotSupported) {
			t.Fatalf("sync-only synthesizer err = %v, want ErrNotSupported", err)
		}
		if end := hook.waitEnd(t); !errors.Is(end.Err, ErrNotSupported) {
			t.Fatalf("telemetry end err = %v, want ErrNotSupported", end.Err)
		}

		// Rate limiter timeout under client timeout maps to ErrTimeout.
		hook2 := newSpeechTelemetryHook()
		limiter := speechRateLimiter{wait: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}
		if _, err := NewClient(p, WithTelemetry(hook2), WithTimeout(15*time.Millisecond), WithRateLimiter(limiter)).
			StreamSynthesize(context.Background(), validSpeechRequest()); !errors.Is(err, ErrTimeout) {
			t.Fatalf("rate limiter timeout err = %v, want ErrTimeout", err)
		}
	})

	t.Run("mid-stream client timeout remaps to ErrTimeout", func(t *testing.T) {
		hook := newSpeechTelemetryHook()
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			streamSynthesize: func(ctx context.Context, _ *SpeechRequest) (*SpeechStream, error) {
				ch := make(chan AudioChunk)
				errCh := make(chan error, 1)
				finalCh := make(chan *SpeechResponse, 1)
				go func() {
					<-ctx.Done()
					close(ch)
					errCh <- ctx.Err()
					close(errCh)
					close(finalCh)
				}()
				return &SpeechStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		stream, err := NewClient(p, WithTelemetry(hook), WithTimeout(20*time.Millisecond)).
			StreamSynthesize(context.Background(), validSpeechRequest())
		if err != nil {
			t.Fatalf("StreamSynthesize() setup error = %v", err)
		}
		_, err = DrainSpeechStream(context.Background(), stream)
		if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("DrainSpeechStream() err = %v, want ErrTimeout wrapping DeadlineExceeded", err)
		}
		if end := hook.waitEnd(t); !errors.Is(end.Err, ErrTimeout) {
			t.Fatalf("telemetry end err = %v, want ErrTimeout", end.Err)
		}
	})

	t.Run("mid-stream caller cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			streamSynthesize: func(ctx context.Context, _ *SpeechRequest) (*SpeechStream, error) {
				ch := make(chan AudioChunk)
				errCh := make(chan error, 1)
				finalCh := make(chan *SpeechResponse, 1)
				go func() {
					<-ctx.Done()
					close(ch)
					errCh <- ctx.Err()
					close(errCh)
					close(finalCh)
				}()
				return &SpeechStream{Ch: ch, Err: errCh, Final: finalCh}, nil
			},
		}

		stream, err := NewClient(p, WithTimeout(time.Second)).StreamSynthesize(ctx, validSpeechRequest())
		if err != nil {
			t.Fatalf("StreamSynthesize() setup error = %v", err)
		}
		cancel()
		_, err = DrainSpeechStream(context.Background(), stream)
		if errors.Is(err, ErrTimeout) || !errors.Is(err, context.Canceled) {
			t.Fatalf("DrainSpeechStream() err = %v, want context.Canceled (not ErrTimeout)", err)
		}
	})
}

func TestClientTranscribeSuccessFailureRetryAndCancellation(t *testing.T) {
	t.Run("success with AudioData and contextual telemetry", func(t *testing.T) {
		req := validAudioTranscriptionRequest()
		want := &TranscriptionResponse{ID: "tr-1", Text: "hello", Language: "en", IsFinal: true}
		hook := &contextualSpeechTelemetryHook{speechTelemetryHook: newSpeechTelemetryHook()}
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			transcribe: func(ctx context.Context, got *TranscriptionRequest) (*TranscriptionResponse, error) {
				if got != req {
					t.Error("Transcribe did not forward original request")
				}
				if ctx.Value(speechContextKey{}) != "speech-trace" {
					t.Error("Transcribe did not receive contextual telemetry context")
				}
				return want, nil
			},
		}

		resp, err := NewClient(p, WithTelemetry(hook)).Transcribe(context.Background(), req)
		if err != nil || resp != want {
			t.Fatalf("Transcribe() = (%v, %v), want %v", resp, err, want)
		}
		if !hook.endSawContext {
			t.Error("contextual telemetry end hook did not receive context")
		}
	})

	t.Run("validation, nil ctx, and unsupported provider", func(t *testing.T) {
		hook := newSpeechTelemetryHook()
		//nolint:staticcheck // intentionally testing nil context guard
		if _, err := NewClient(&capabilityBaseProvider{id: "base"}, WithTelemetry(hook)).Transcribe(nil, validAudioTranscriptionRequest()); err == nil {
			t.Fatal("expected error for nil ctx")
		}
		if _, err := NewClient(&capabilityBaseProvider{id: "base"}, WithTelemetry(hook)).Transcribe(context.Background(), nil); !errors.Is(err, ErrInvalidTranscriptionRequest) {
			t.Fatalf("nil req err = %v, want ErrInvalidTranscriptionRequest", err)
		}
		if _, err := NewClient(&capabilityBaseProvider{id: "base"}, WithTelemetry(hook)).Transcribe(context.Background(), validAudioTranscriptionRequest()); !errors.Is(err, ErrNotSupported) {
			t.Fatalf("unsupported err = %v, want ErrNotSupported", err)
		}
		if starts, ends := hook.counts(); starts != 0 || ends != 0 {
			t.Fatalf("telemetry should not fire on pre-dispatch errors, got (%d, %d)", starts, ends)
		}
	})

	t.Run("retries AudioData but never retries Reader", func(t *testing.T) {
		retry := NewRetryPolicy(RetryConfig{MaxRetries: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Microsecond})

		// AudioData retries and succeeds on 2nd attempt.
		pAudio := &speechTestProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"}}
		pAudio.transcribe = func(context.Context, *TranscriptionRequest) (*TranscriptionResponse, error) {
			if pAudio.transcribeCalls == 1 {
				return nil, ErrServer
			}
			return &TranscriptionResponse{Text: "recovered", IsFinal: true}, nil
		}
		resp, err := NewClient(pAudio, WithRetryPolicy(retry)).Transcribe(context.Background(), validAudioTranscriptionRequest())
		if err != nil || resp.Text != "recovered" || pAudio.transcribeCalls != 2 {
			t.Fatalf("AudioData retry = (%v, %v, calls=%d), want recovered in 2 calls", resp, err, pAudio.transcribeCalls)
		}

		// Reader-backed request must NOT retry.
		pReader := &speechTestProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"}}
		pReader.transcribe = func(context.Context, *TranscriptionRequest) (*TranscriptionResponse, error) {
			return nil, ErrServer
		}
		readerReq := &TranscriptionRequest{
			Model:    "scribe_v2",
			Reader:   bytes.NewReader([]byte("streamed")),
			Filename: "clip.wav",
		}
		_, err = NewClient(pReader, WithRetryPolicy(retry)).Transcribe(context.Background(), readerReq)
		if !errors.Is(err, ErrServer) || pReader.transcribeCalls != 1 {
			t.Fatalf("Reader request = (%v, calls=%d), want ErrServer after exactly 1 call", err, pReader.transcribeCalls)
		}
	})

	t.Run("timeout and cancellation", func(t *testing.T) {
		p := &speechTestProvider{
			capabilityBaseProvider: &capabilityBaseProvider{id: "elevenlabs"},
			transcribe: func(ctx context.Context, _ *TranscriptionRequest) (*TranscriptionResponse, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		_, err := NewClient(p, WithTimeout(15*time.Millisecond), WithRetryPolicy(speechNoRetryPolicy{})).
			Transcribe(context.Background(), validAudioTranscriptionRequest())
		if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout err = %v, want ErrTimeout wrapping DeadlineExceeded", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = NewClient(p, WithTimeout(time.Second), WithRetryPolicy(speechNoRetryPolicy{})).
			Transcribe(ctx, validAudioTranscriptionRequest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel err = %v, want context.Canceled", err)
		}
	})
}
