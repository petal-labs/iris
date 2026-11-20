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

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRunStreamSuccessFailureAndCancellation(t *testing.T) {
	t.Run("success streams chunks incrementally to file", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"audio/mpeg"},
						"Request-Id":   []string{"req-stream-ex"},
					},
					Body: io.NopCloser(strings.NewReader("chunk1-chunk2-chunk3")),
				}, nil
			}),
		}

		outFile := filepath.Join(t.TempDir(), "stream.mp3")
		var stdout bytes.Buffer
		p := elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient), elevenlabs.WithStreamChunkSize(7))
		client := core.NewClient(p)

		err := runStream(context.Background(), client, streamConfig{
			Text:    "Streaming test",
			VoiceID: defaultVoiceID,
			Model:   elevenlabs.ModelElevenFlashV25,
			OutPath: outFile,
		}, &stdout)
		if err != nil {
			t.Fatalf("runStream() error = %v", err)
		}

		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != "chunk1-chunk2-chunk3" {
			t.Errorf("streamed bytes = %q, want chunk1-chunk2-chunk3", data)
		}
		if !strings.Contains(stdout.String(), "req-stream-ex") {
			t.Errorf("stdout missing metadata: %s", stdout.String())
		}
	})

	t.Run("upstream HTTP failure returns error", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"detail":"invalid api key"}`)),
				}, nil
			}),
		}
		client := core.NewClient(elevenlabs.New("bad-key", elevenlabs.WithHTTPClient(httpClient)))

		err := runStream(context.Background(), client, streamConfig{
			Text:    "Hi",
			VoiceID: defaultVoiceID,
			OutPath: filepath.Join(t.TempDir(), "stream.mp3"),
		}, io.Discard)
		if !errors.Is(err, core.ErrUnauthorized) {
			t.Fatalf("err = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("mid-stream cancellation returns context.Canceled", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()

		httpClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       pr,
				}, nil
			}),
		}
		client := core.NewClient(elevenlabs.New("test-key", elevenlabs.WithHTTPClient(httpClient)))
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			_, _ = pw.Write([]byte("partial"))
			cancel()
		}()

		err := runStream(ctx, client, streamConfig{
			Text:    "Cancel me",
			VoiceID: defaultVoiceID,
			OutPath: filepath.Join(t.TempDir(), "stream.mp3"),
		}, io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
