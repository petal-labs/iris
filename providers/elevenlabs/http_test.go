package elevenlabs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/v2/core"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDoRequestEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hit = true
			return nil, errors.New("should not be called")
		}),
	}

	p := New("   ", WithHTTPClient(client))
	_, err := p.doRequest(context.Background(), http.MethodGet, "/v1/voices", "", nil)
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want core.ErrUnauthorized", err)
	}
	if hit {
		t.Error("HTTP transport must not be called when API key is empty")
	}
}

func TestDoRequestSuccessHeadersAndBodyReadAfterReturn(t *testing.T) {
	var capturedURL, capturedKey, capturedCT, capturedCustom string
	var bodyCtxAlive bool

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			capturedKey = req.Header.Get("xi-api-key")
			capturedCT = req.Header.Get("Content-Type")
			capturedCustom = req.Header.Get("X-Trace-ID")

			pr, pw := io.Pipe()
			go func() {
				// Wait briefly so we confirm doRequest did not cancel req.Context() on return.
				time.Sleep(10 * time.Millisecond)
				bodyCtxAlive = req.Context().Err() == nil
				_, _ = pw.Write([]byte("audio-payload"))
				_ = pw.Close()
			}()

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Request-Id": []string{"req-ok-1"}},
				Body:       pr,
			}, nil
		}),
	}

	p := New(
		"el_key_123",
		WithBaseURL("https://custom.elevenlabs.test/api/"),
		WithHTTPClient(client),
		WithHeader("X-Trace-ID", "trace-42"),
		WithTimeout(time.Second),
	)

	resp, err := p.doRequest(context.Background(), http.MethodPost, "v1/text-to-speech/voice1", "application/json", strings.NewReader(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body error = %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing body error = %v", err)
	}

	if string(data) != "audio-payload" {
		t.Errorf("body = %q, want audio-payload", data)
	}
	if !bodyCtxAlive {
		t.Error("request context was cancelled before response body was read")
	}
	if capturedURL != "https://custom.elevenlabs.test/api/v1/text-to-speech/voice1" {
		t.Errorf("URL = %q", capturedURL)
	}
	if capturedKey != "el_key_123" || capturedCT != "application/json" || capturedCustom != "trace-42" {
		t.Errorf("headers = (key=%q, ct=%q, custom=%q)", capturedKey, capturedCT, capturedCustom)
	}
}

func TestDoRequestErrorResponsesAndNetworkFailure(t *testing.T) {
	t.Run("typed HTTP error with x-request-id and Retry-After", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header: http.Header{
						"X-Request-Id": []string{"xreq-99"},
						"Retry-After":  []string{"3"},
					},
					Body: io.NopCloser(strings.NewReader(`{"detail":{"code":"rate_limit_exceeded","message":"slow down"}}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		_, err := p.doRequest(context.Background(), http.MethodGet, "/v1/voices", "", nil)
		if !errors.Is(err, core.ErrRateLimited) {
			t.Fatalf("err = %v, want core.ErrRateLimited", err)
		}
		var pe *core.ProviderError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *core.ProviderError, got %T", err)
		}
		if pe.RequestID != "xreq-99" || pe.Code != "rate_limit_exceeded" || pe.RetryAfter != 3*time.Second {
			t.Errorf("unexpected ProviderError: %+v", pe)
		}
	})

	t.Run("network error wrapped as core.ErrNetwork", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("dial tcp: connection refused")
			}),
		}

		p := New("k", WithHTTPClient(client))
		_, err := p.doRequest(context.Background(), http.MethodGet, "/v1/voices", "", nil)
		if !errors.Is(err, core.ErrNetwork) {
			t.Fatalf("err = %v, want core.ErrNetwork", err)
		}
	})
}

func TestDoRequestTimeoutsAndCancellation(t *testing.T) {
	blockingClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}),
	}

	t.Run("provider timeout bounds request", func(t *testing.T) {
		p := New("k", WithHTTPClient(blockingClient), WithTimeout(20*time.Millisecond))
		_, err := p.doRequest(context.Background(), http.MethodGet, "/v1/voices", "", nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("caller deadline takes precedence over large provider timeout", func(t *testing.T) {
		p := New("k", WithHTTPClient(blockingClient), WithTimeout(time.Hour))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := p.doRequest(ctx, http.MethodGet, "/v1/voices", "", nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("caller cancellation interrupts request", func(t *testing.T) {
		p := New("k", WithHTTPClient(blockingClient), WithTimeout(time.Second))
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()

		_, err := p.doRequest(ctx, http.MethodGet, "/v1/voices", "", nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("WithTimeout(0) adds no deadline and nil context is safe", func(t *testing.T) {
		var sawDeadline bool
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, sawDeadline = req.Context().Deadline()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client), WithTimeout(0))
		//nolint:staticcheck // intentionally testing nil context fallback
		resp, err := p.doRequest(nil, http.MethodGet, "/v1/voices", "", nil)
		if err != nil {
			t.Fatalf("doRequest() error = %v", err)
		}
		_ = resp.Body.Close()
		if sawDeadline {
			t.Error("WithTimeout(0) should not attach a context deadline")
		}
	})
}
