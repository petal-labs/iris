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

func TestRunSynthesizeSuccessFailureAndCancellation(t *testing.T) {
	t.Run("success writes audio file and logs metadata", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type":   []string{"audio/mpeg"},
						"Request-Id":     []string{"req-ex-1"},
						"Character-Cost": []string{"12"},
					},
					Body: io.NopCloser(strings.NewReader("mp3-example-bytes")),
				}, nil
			}),
		}

		outFile := filepath.Join(t.TempDir(), "out.mp3")
		var stdout bytes.Buffer
		client := core.NewClient(elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient)))

		err := runSynthesize(context.Background(), client, synthesizeConfig{
			Text:    "Hello world!",
			VoiceID: defaultVoiceID,
			Model:   elevenlabs.ModelElevenFlashV25,
			OutPath: outFile,
		}, &stdout)
		if err != nil {
			t.Fatalf("runSynthesize() error = %v", err)
		}

		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != "mp3-example-bytes" {
			t.Errorf("written audio = %q, want mp3-example-bytes", data)
		}
		if !strings.Contains(stdout.String(), "req-ex-1") {
			t.Errorf("stdout missing request metadata: %s", stdout.String())
		}
	})

	t.Run("validation failure returns error", func(t *testing.T) {
		client := core.NewClient(elevenlabs.New("test-key"))
		err := runSynthesize(context.Background(), client, synthesizeConfig{
			Text:    "",
			VoiceID: defaultVoiceID,
			OutPath: filepath.Join(t.TempDir(), "out.mp3"),
		}, io.Discard)
		if !errors.Is(err, core.ErrInvalidSpeechRequest) {
			t.Fatalf("err = %v, want ErrInvalidSpeechRequest", err)
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

		err := runSynthesize(ctx, client, synthesizeConfig{
			Text:    "Hello",
			VoiceID: defaultVoiceID,
			OutPath: filepath.Join(t.TempDir(), "out.mp3"),
		}, io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
