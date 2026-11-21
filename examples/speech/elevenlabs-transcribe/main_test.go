package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers/elevenlabs"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRunTranscribeSuccessFailureAndCancellation(t *testing.T) {
	audioPath := filepath.Join(t.TempDir(), "sample.mp3")
	if err := os.WriteFile(audioPath, []byte("fake-mp3-audio"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Run("success transcribes file with diarized words", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, _ = io.Copy(io.Discard, req.Body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"transcription_id": "tr-ex-1",
						"language_code": "eng",
						"text": "Hello Iris",
						"words": [
							{"text": "Hello", "start": 0.0, "end": 0.3, "type": "word", "speaker_id": "speaker_0"},
							{"text": " ", "start": 0.3, "end": 0.35, "type": "spacing", "speaker_id": "speaker_0"},
							{"text": "Iris", "start": 0.35, "end": 0.7, "type": "word", "speaker_id": "speaker_0"}
						]
					}`)),
				}, nil
			}),
		}

		var stdout bytes.Buffer
		client := core.NewClient(elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient)))

		err := runTranscribe(context.Background(), client, transcribeConfig{
			FilePath:   audioPath,
			Model:      elevenlabs.ModelScribeV2,
			Diarize:    true,
			Timestamps: core.TimestampGranularityWord,
		}, &stdout)
		if err != nil {
			t.Fatalf("runTranscribe() error = %v", err)
		}
		out := stdout.String()
		if !strings.Contains(out, "tr-ex-1") || !strings.Contains(out, "Hello Iris") || !strings.Contains(out, "speaker_0") {
			t.Errorf("unexpected stdout:\n%s", out)
		}
	})

	t.Run("missing file path and nonexistent file fail", func(t *testing.T) {
		client := core.NewClient(elevenlabs.New("test-key"))
		if err := runTranscribe(context.Background(), client, transcribeConfig{}, io.Discard); err == nil {
			t.Fatal("expected error when FilePath is empty")
		}
		if err := runTranscribe(context.Background(), client, transcribeConfig{FilePath: filepath.Join(t.TempDir(), "missing.mp3")}, io.Discard); err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("cancelled context returns context.Canceled", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		}
		client := core.NewClient(elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient)))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := runTranscribe(ctx, client, transcribeConfig{
			FilePath: audioPath,
			Model:    elevenlabs.ModelScribeV2,
		}, io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
