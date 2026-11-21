package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/petal-labs/iris/providers/elevenlabs"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRunVoiceDiscoverySuccessFailureAndCancellation(t *testing.T) {
	t.Run("lists voices with filters", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, "/v2/voices") {
					t.Errorf("unexpected path: %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"voices": [
							{"voice_id": "v1", "name": "George", "category": "premade"}
						],
						"has_more": false
					}`)),
				}, nil
			}),
		}

		p := elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient))
		var out bytes.Buffer
		if err := runVoiceDiscovery(context.Background(), p, voiceDiscoveryConfig{
			Category: "premade",
			Limit:    5,
		}, &out); err != nil {
			t.Fatalf("runVoiceDiscovery() err = %v", err)
		}
		if !strings.Contains(out.String(), "George") || !strings.Contains(out.String(), "v1") {
			t.Errorf("unexpected output: %s", out.String())
		}
	})

	t.Run("inspects single voice and settings", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/voices/v1":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"voice_id": "v1", "name": "George", "category": "premade"}`)),
					}, nil
				case "/v1/voices/v1/settings":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"stability": 0.5, "similarity_boost": 0.75}`)),
					}, nil
				default:
					t.Fatalf("unexpected path: %s", req.URL.Path)
					return nil, nil
				}
			}),
		}

		p := elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient))
		var out bytes.Buffer
		if err := runVoiceDiscovery(context.Background(), p, voiceDiscoveryConfig{VoiceID: "v1"}, &out); err != nil {
			t.Fatalf("runVoiceDiscovery(single) err = %v", err)
		}
		if !strings.Contains(out.String(), "stability=0.50") {
			t.Errorf("expected settings output, got: %s", out.String())
		}
	})

	t.Run("nil provider fails fast", func(t *testing.T) {
		if err := runVoiceDiscovery(context.Background(), nil, voiceDiscoveryConfig{}, io.Discard); err == nil {
			t.Fatal("expected error for nil provider")
		}
	})

	t.Run("cancelled context returns context.Canceled", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		}
		p := elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := runVoiceDiscovery(ctx, p, voiceDiscoveryConfig{Limit: 5}, io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
