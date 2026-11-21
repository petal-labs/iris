package elevenlabs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petal-labs/iris/v2/core"
)

type captureTelemetryHook struct {
	mu     sync.Mutex
	starts []core.RequestStartEvent
	ends   []core.RequestEndEvent
}

func (h *captureTelemetryHook) OnRequestStart(e core.RequestStartEvent) {
	h.mu.Lock()
	h.starts = append(h.starts, e)
	h.mu.Unlock()
}

func (h *captureTelemetryHook) OnRequestEnd(e core.RequestEndEvent) {
	h.mu.Lock()
	h.ends = append(h.ends, e)
	h.mu.Unlock()
}

func TestPrivacyNoSecretsAudioOrTranscriptsInDefaultLogsAndTelemetry(t *testing.T) {
	const (
		secretAPIKey     = "xi_live_super_secret_key_DO_NOT_LOG"
		secretHeaderVal  = "proxy_secret_token_DO_NOT_LOG"
		secretText       = "PRIVATE_SYNTHESIS_TEXT_DO_NOT_LOG"
		secretPrompt     = "PRIVATE_STT_PROMPT_DO_NOT_LOG"
		secretAudioRaw   = "RAW_BINARY_AUDIO_BYTES_DO_NOT_LOG"
		secretTranscript = "PRIVATE_TRANSCRIPT_OUTPUT_DO_NOT_LOG"
	)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	hook := &captureTelemetryHook{}

	// First: happy path across Synthesize, StreamSynthesize, and Transcribe.
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/stream"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"audio/mpeg"}, "Request-Id": []string{"req-stream"}},
					Body:       io.NopCloser(strings.NewReader(secretAudioRaw)),
				}, nil
			case strings.Contains(req.URL.Path, "/v1/text-to-speech/"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"audio/mpeg"}, "Request-Id": []string{"req-tts"}},
					Body:       io.NopCloser(strings.NewReader(secretAudioRaw)),
				}, nil
			default:
				_, _ = io.Copy(io.Discard, req.Body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Request-Id": []string{"req-stt"}},
					Body:       io.NopCloser(strings.NewReader(`{"text":"` + secretTranscript + `","language_code":"en"}`)),
				}, nil
			}
		}),
	}

	p := New(secretAPIKey, WithHTTPClient(client), WithHeader("X-Proxy-Secret", secretHeaderVal))
	sdk := core.NewClient(p, WithTelemetryForTest(hook)...)

	ttsReq := &core.SpeechRequest{
		Model:   ModelElevenFlashV25,
		VoiceID: "v1",
		Text:    secretText,
		Format:  core.AudioFormatMP3_44100_128,
	}
	ttsResp, err := sdk.Synthesize(context.Background(), ttsReq)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	stream, err := sdk.StreamSynthesize(context.Background(), ttsReq)
	if err != nil {
		t.Fatalf("StreamSynthesize() error = %v", err)
	}
	var lastChunk core.AudioChunk
	for chunk := range stream.Ch {
		if len(chunk.Data) > 0 {
			lastChunk = chunk
		}
	}
	<-stream.Err
	<-stream.Final

	sttReq := &core.TranscriptionRequest{
		Model:  ModelScribeV2,
		Audio:  &core.AudioData{Data: []byte(secretAudioRaw), Format: core.AudioFormatMP3_44100_128},
		Prompt: secretPrompt,
	}
	sttResp, err := sdk.Transcribe(context.Background(), sttReq)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	// Second: hostile server echoing the API key back in a 401 error body.
	hostileClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Request-Id": []string{"req-hostile"}},
				Body:       io.NopCloser(strings.NewReader(`{"detail":{"status":"invalid_api_key","message":"Rejected key ` + secretAPIKey + `"}}`)),
			}, nil
		}),
	}
	pHostile := New(secretAPIKey, WithHTTPClient(hostileClient))
	_, hostileErr := pHostile.Synthesize(context.Background(), ttsReq)
	if !errors.Is(hostileErr, core.ErrUnauthorized) {
		t.Fatalf("hostileErr = %v, want ErrUnauthorized", hostileErr)
	}

	// Log every object through default slog/fmt formatting.
	logger.Info("speech_audit",
		"provider", p,
		"provider_go", fmt.Sprintf("%#v", p),
		"config", p.config,
		"tts_req", ttsReq,
		"tts_resp", ttsResp,
		"audio_chunk", lastChunk,
		"stt_req", sttReq,
		"stt_resp", sttResp,
		"hostile_err", hostileErr,
		"telemetry_starts", fmt.Sprintf("%+v", hook.starts),
		"telemetry_ends", fmt.Sprintf("%+v", hook.ends),
	)

	var pe *core.ProviderError
	if errors.As(hostileErr, &pe) {
		logger.Error("provider_error_body", "body", pe.Body, "message", pe.Message)
	}

	logged := logBuf.String()
	forbidden := []string{
		secretAPIKey,
		secretHeaderVal,
		secretText,
		secretPrompt,
		secretAudioRaw,
		secretTranscript,
	}
	for _, needle := range forbidden {
		if strings.Contains(logged, needle) {
			t.Errorf("default log/telemetry/error output leaked sensitive value %q:\n%s", needle, logged)
		}
	}
}

func WithTelemetryForTest(h core.TelemetryHook) []core.ClientOption {
	return []core.ClientOption{core.WithTelemetry(h)}
}

func TestRetrySynthesize4295xxAndNonRetryable(t *testing.T) {
	t.Run("retries 429 with Retry-After and 503 then succeeds", func(t *testing.T) {
		var attempts atomic.Int32
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				n := attempts.Add(1)
				switch n {
				case 1:
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header:     http.Header{"Retry-After": []string{"10ms"}},
						Body:       io.NopCloser(strings.NewReader(`{"detail":"rate limited"}`)),
					}, nil
				case 2:
					return &http.Response{
						StatusCode: http.StatusServiceUnavailable,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"detail":"upstream unavailable"}`)),
					}, nil
				default:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"audio/mpeg"}},
						Body:       io.NopCloser(strings.NewReader("recovered-audio")),
					}, nil
				}
			}),
		}

		p := New("k", WithHTTPClient(client))
		retry := core.NewRetryPolicy(core.RetryConfig{
			MaxRetries: 3,
			BaseDelay:  time.Millisecond,
			MaxDelay:   50 * time.Millisecond,
			Jitter:     0,
		})
		sdk := core.NewClient(p, core.WithRetryPolicy(retry))

		start := time.Now()
		resp, err := sdk.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Synthesize() error = %v", err)
		}
		if string(resp.Audio.Data) != "recovered-audio" {
			t.Errorf("Audio.Data = %q, want recovered-audio", resp.Audio.Data)
		}
		if got := attempts.Load(); got != 3 {
			t.Errorf("attempts = %d, want 3", got)
		}
		if elapsed < 9*time.Millisecond {
			t.Errorf("elapsed = %s, expected Retry-After >= 10ms to be respected", elapsed)
		}
	})

	t.Run("retries are bounded by MaxRetries on persistent 500", func(t *testing.T) {
		var attempts atomic.Int32
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts.Add(1)
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"detail":"boom"}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		retry := core.NewRetryPolicy(core.RetryConfig{
			MaxRetries: 2,
			BaseDelay:  time.Nanosecond,
			MaxDelay:   time.Microsecond,
			Jitter:     0,
		})
		sdk := core.NewClient(p, core.WithRetryPolicy(retry))

		_, err := sdk.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if !errors.Is(err, core.ErrServer) {
			t.Fatalf("err = %v, want core.ErrServer", err)
		}
		if got := attempts.Load(); got != 3 {
			t.Errorf("attempts = %d, want 3 (1 initial + 2 retries)", got)
		}
	})

	t.Run("non-retryable 4xx statuses are never retried", func(t *testing.T) {
		for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity} {
			var attempts atomic.Int32
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					attempts.Add(1)
					return &http.Response{
						StatusCode: status,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"detail":"client error"}`)),
					}, nil
				}),
			}
			p := New("k", WithHTTPClient(client))
			retry := core.NewRetryPolicy(core.RetryConfig{MaxRetries: 3, BaseDelay: time.Nanosecond, MaxDelay: time.Microsecond})
			sdk := core.NewClient(p, core.WithRetryPolicy(retry))

			_, err := sdk.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
			if err == nil {
				t.Fatalf("status %d: expected error", status)
			}
			if got := attempts.Load(); got != 1 {
				t.Errorf("status %d: attempts = %d, want 1 (must not retry)", status, got)
			}
		}
	})

	t.Run("context cancellation aborts Retry-After wait promptly", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				go func() {
					time.Sleep(15 * time.Millisecond)
					cancel()
				}()
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"60"}},
					Body:       io.NopCloser(strings.NewReader(`{"detail":"slow down"}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		sdk := core.NewClient(p, core.WithRetryPolicy(core.DefaultRetryPolicy()))

		start := time.Now()
		_, err := sdk.Synthesize(ctx, &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("cancellation took %s, expected prompt exit during Retry-After backoff", elapsed)
		}
	})
}

func TestRetryTranscribeAudioDataVsReader(t *testing.T) {
	retry := core.NewRetryPolicy(core.RetryConfig{
		MaxRetries: 2,
		BaseDelay:  time.Nanosecond,
		MaxDelay:   time.Microsecond,
		Jitter:     0,
	})

	t.Run("AudioData retries 429 and 502 then succeeds", func(t *testing.T) {
		var attempts atomic.Int32
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, _ = io.Copy(io.Discard, req.Body)
				n := attempts.Add(1)
				if n == 1 {
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"detail":"rate limited"}`)),
					}, nil
				}
				if n == 2 {
					return &http.Response{
						StatusCode: http.StatusBadGateway,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`bad gateway`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"text":"ok","language_code":"en"}`)),
				}, nil
			}),
		}

		sdk := core.NewClient(New("k", WithHTTPClient(client)), core.WithRetryPolicy(retry))
		resp, err := sdk.Transcribe(context.Background(), &core.TranscriptionRequest{
			Audio: &core.AudioData{Data: []byte("mp3"), Format: core.AudioFormatMP3_44100_128},
		})
		if err != nil || resp.Text != "ok" || attempts.Load() != 3 {
			t.Fatalf("got (%v, %v, attempts=%d), want ok in 3 attempts", resp, err, attempts.Load())
		}
	})

	t.Run("Reader never retries 500", func(t *testing.T) {
		var attempts atomic.Int32
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, _ = io.Copy(io.Discard, req.Body)
				attempts.Add(1)
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"detail":"server err"}`)),
				}, nil
			}),
		}

		sdk := core.NewClient(New("k", WithHTTPClient(client)), core.WithRetryPolicy(retry))
		_, err := sdk.Transcribe(context.Background(), &core.TranscriptionRequest{
			Reader: bytes.NewReader([]byte("streamed")),
		})
		if !errors.Is(err, core.ErrServer) || attempts.Load() != 1 {
			t.Fatalf("got (%v, attempts=%d), want ErrServer in exactly 1 attempt", err, attempts.Load())
		}
	})
}

func TestResourceCleanupBodiesAndGoroutines(t *testing.T) {
	t.Run("response bodies always closed across error and success statuses", func(t *testing.T) {
		var bodies []*trackingReadCloser
		var mu sync.Mutex
		nextStatus := []int{http.StatusOK, http.StatusBadRequest, http.StatusTooManyRequests, http.StatusInternalServerError}
		var idx atomic.Int32

		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, _ = io.Copy(io.Discard, req.Body)
				i := int(idx.Add(1) - 1)
				status := nextStatus[i%len(nextStatus)]
				payload := "audio-bytes"
				if status != http.StatusOK {
					payload = `{"detail":"error"}`
				}
				b := &trackingReadCloser{ReadCloser: io.NopCloser(strings.NewReader(payload))}
				mu.Lock()
				bodies = append(bodies, b)
				mu.Unlock()
				return &http.Response{
					StatusCode: status,
					Header:     make(http.Header),
					Body:       b,
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		for range len(nextStatus) {
			_, _ = p.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		}

		mu.Lock()
		defer mu.Unlock()
		for i, b := range bodies {
			if !b.closed.Load() {
				t.Errorf("response body[%d] was not closed", i)
			}
		}
	})

	t.Run("no goroutine leaks across repeated streaming and transcription cancellations", func(t *testing.T) {
		// Allow any background runtime goroutines to settle first.
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		baseline := runtime.NumGoroutine()

		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/stream") {
					pr, pw := io.Pipe()
					go func() {
						defer pw.Close()
						for range 50 {
							if _, err := pw.Write([]byte("chunk")); err != nil {
								return
							}
						}
					}()
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       pr,
					}, nil
				}
				_, _ = io.Copy(io.Discard, req.Body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"text":"ok"}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client), WithStreamChunkSize(4), WithStreamBufferSize(1))

		for range 10 {
			// Abandoned stream + cancel
			ctx, cancel := context.WithCancel(context.Background())
			s, err := p.StreamSynthesize(ctx, &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
			if err != nil {
				t.Fatalf("StreamSynthesize() error = %v", err)
			}
			cancel()
			_, _ = core.DrainSpeechStream(context.Background(), s)

			// Oversized streaming reader transcription
			_, _ = p.Transcribe(context.Background(), &core.TranscriptionRequest{
				Reader:   bytes.NewReader(bytes.Repeat([]byte("a"), 64)),
				MaxBytes: 8,
			})
		}

		// Wait briefly for goroutines to exit cleanly.
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			runtime.GC()
			if runtime.NumGoroutine() <= baseline+2 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("possible goroutine leak: baseline=%d, after=%d", baseline, runtime.NumGoroutine())
	})
}

func TestConcurrentMixedSpeechOperations(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/stream"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"audio/mpeg"}},
					Body:       io.NopCloser(strings.NewReader("stream-audio")),
				}, nil
			case strings.Contains(req.URL.Path, "/v1/text-to-speech/"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"audio/mpeg"}},
					Body:       io.NopCloser(strings.NewReader("sync-audio")),
				}, nil
			default:
				_, _ = io.Copy(io.Discard, req.Body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"text":"concurrent ok","language_code":"en"}`)),
				}, nil
			}
		}),
	}

	sdk := core.NewClient(New("k", WithHTTPClient(client)))

	const workers = 12
	var wg sync.WaitGroup
	errCh := make(chan error, workers*3)

	for range workers {
		wg.Add(3)
		go func() {
			defer wg.Done()
			resp, err := sdk.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hello"})
			if err != nil || string(resp.Audio.Data) != "sync-audio" {
				errCh <- fmt.Errorf("sync failed: resp=%v err=%v", resp, err)
			}
		}()
		go func() {
			defer wg.Done()
			s, err := sdk.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hello"})
			if err != nil {
				errCh <- fmt.Errorf("stream setup failed: %w", err)
				return
			}
			resp, err := core.DrainSpeechStream(context.Background(), s)
			if err != nil || string(resp.Audio.Data) != "stream-audio" {
				errCh <- fmt.Errorf("stream drain failed: resp=%v err=%v", resp, err)
			}
		}()
		go func() {
			defer wg.Done()
			resp, err := sdk.Transcribe(context.Background(), &core.TranscriptionRequest{
				Audio: &core.AudioData{Data: []byte("audio"), Format: core.AudioFormatMP3_44100_128},
			})
			if err != nil || resp.Text != "concurrent ok" {
				errCh <- fmt.Errorf("transcribe failed: resp=%v err=%v", resp, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}
