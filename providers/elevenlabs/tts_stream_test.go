package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

type trackingReadCloser struct {
	io.ReadCloser
	closed atomic.Bool
	onRead func()
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	if t.onRead != nil {
		t.onRead()
	}
	return t.ReadCloser.Read(p)
}

func (t *trackingReadCloser) Close() error {
	t.closed.Store(true)
	return t.ReadCloser.Close()
}

type partialFailReader struct {
	chunks [][]byte
	idx    int
	err    error
	closed atomic.Bool
}

func (r *partialFailReader) Read(p []byte) (int, error) {
	if r.idx < len(r.chunks) {
		n := copy(p, r.chunks[r.idx])
		r.idx++
		return n, nil
	}
	return 0, r.err
}

func (r *partialFailReader) Close() error {
	r.closed.Store(true)
	return nil
}

func TestStreamSynthesizeIncrementalChunksAndFinalMetadata(t *testing.T) {
	var capturedPath, capturedQuery, capturedKey string
	var capturedBody ttsRequestBody

	pr, pw := io.Pipe()
	body := &trackingReadCloser{ReadCloser: pr}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedPath = req.URL.Path
			capturedQuery = req.URL.RawQuery
			capturedKey = req.Header.Get("xi-api-key")
			if err := json.NewDecoder(req.Body).Decode(&capturedBody); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type":   []string{"audio/L16; rate=16000"},
					"Request-Id":     []string{"req_stream_42"},
					"Character-Cost": []string{"24"},
				},
				Body: body,
			}, nil
		}),
	}

	p := New(
		"sk_stream_key",
		WithHTTPClient(client),
		WithStreamChunkSize(8),
		WithStreamBufferSize(4),
		WithStreamChunkSize(-1), // ignored
		WithStreamBufferSize(0), // ignored
	)

	stream, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{
		Model:   ModelElevenFlashV25,
		VoiceID: "voice_stream_1",
		Text:    "Streaming audio from Iris",
		Format:  core.AudioFormatPCM16000,
	})
	if err != nil {
		t.Fatalf("StreamSynthesize() error = %v", err)
	}

	if cap(stream.Ch) != 4 {
		t.Errorf("cap(stream.Ch) = %d, want 4", cap(stream.Ch))
	}

	// Write chunk 1 and verify it arrives BEFORE chunk 2 is written (true incremental streaming).
	go func() {
		_, _ = pw.Write([]byte("chunk-01"))
	}()

	first, ok := <-stream.Ch
	if !ok {
		t.Fatal("stream.Ch closed before receiving first chunk")
	}
	if string(first.Data) != "chunk-01" || first.Sequence != 0 || first.IsFinal {
		t.Fatalf("first chunk = %+v, want seq=0 data=chunk-01", first)
	}
	if first.Format.Encoding != core.AudioEncodingPCM || first.Format.SampleRate != 16000 || first.Format.MIMEType != "audio/l16" {
		t.Errorf("first chunk format = %+v (mime=%q)", first.Format, first.Format.MIMEType)
	}

	// Mutate first chunk buffer to verify no aliasing with subsequent chunks.
	first.Data[0] = 'X'

	go func() {
		_, _ = pw.Write([]byte("chunk-02"))
		_, _ = pw.Write([]byte("end"))
		_ = pw.Close()
	}()

	var rest []core.AudioChunk
	for c := range stream.Ch {
		rest = append(rest, c)
	}
	if len(rest) != 3 {
		t.Fatalf("remaining chunks = %d, want 3 (2 data + 1 final marker)", len(rest))
	}
	if string(rest[0].Data) != "chunk-02" || rest[0].Sequence != 1 || rest[0].IsFinal {
		t.Errorf("chunk 1 = %+v", rest[0])
	}
	if string(rest[1].Data) != "end" || rest[1].Sequence != 2 || rest[1].IsFinal {
		t.Errorf("chunk 2 = %+v", rest[1])
	}
	if !rest[2].IsFinal || len(rest[2].Data) != 0 || rest[2].Sequence != 3 {
		t.Errorf("terminal marker chunk = %+v", rest[2])
	}

	if streamErr, ok := <-stream.Err; ok && streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
	finalResp, ok := <-stream.Final
	if !ok || finalResp == nil {
		t.Fatal("expected final SpeechResponse on Final channel")
	}
	if _, stillOpen := <-stream.Final; stillOpen {
		t.Error("Final channel should be closed after final response")
	}

	if finalResp.ID != "req_stream_42" || finalResp.Model != ModelElevenFlashV25 || finalResp.VoiceID != "voice_stream_1" {
		t.Errorf("finalResp metadata = %+v", finalResp)
	}
	if len(finalResp.Audio.Data) != 0 {
		t.Errorf("finalResp.Audio.Data should remain empty for bounded streaming memory, got %d bytes", len(finalResp.Audio.Data))
	}
	if finalResp.Usage.InputCharacters != 24 || finalResp.Usage.OutputBytes != int64(len("chunk-01chunk-02end")) {
		t.Errorf("finalResp.Usage = %+v", finalResp.Usage)
	}
	if !body.closed.Load() {
		t.Error("response body was not closed on stream completion")
	}
	if capturedPath != "/v1/text-to-speech/voice_stream_1/stream" || capturedQuery != "output_format=pcm_16000" || capturedKey != "sk_stream_key" {
		t.Errorf("request mismatch: path=%q query=%q key=%q", capturedPath, capturedQuery, capturedKey)
	}
	if capturedBody.ModelID != ModelElevenFlashV25 {
		t.Errorf("capturedBody.ModelID = %q", capturedBody.ModelID)
	}
}

func TestStreamSynthesizeViaClientAndDrain(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type":   []string{"audio/mpeg"},
					"Request-Id":     []string{"req_drain_1"},
					"Character-Cost": []string{"5"},
				},
				Body: io.NopCloser(strings.NewReader("0123456789")),
			}, nil
		}),
	}

	p := New("k", WithHTTPClient(client), WithStreamChunkSize(3), WithStreamBufferSize(2))
	sdkClient := core.NewClient(p)

	stream, err := sdkClient.StreamSynthesize(context.Background(), &core.SpeechRequest{
		VoiceID: "v1",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("Client.StreamSynthesize() error = %v", err)
	}
	resp, err := core.DrainSpeechStream(context.Background(), stream)
	if err != nil {
		t.Fatalf("DrainSpeechStream() error = %v", err)
	}
	if string(resp.Audio.Data) != "0123456789" {
		t.Errorf("Audio.Data = %q, want 0123456789", resp.Audio.Data)
	}
	if resp.ID != "req_drain_1" || resp.Model != DefaultTTSModel || resp.Usage.InputCharacters != 5 || resp.Usage.OutputBytes != 10 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestStreamSynthesizeCancellationAndCleanup(t *testing.T) {
	t.Run("cancellation while blocked in body Read closes body and channels", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		body := &trackingReadCloser{ReadCloser: pr}

		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       body,
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client), WithTimeout(time.Second))
		ctx, cancel := context.WithCancel(context.Background())

		stream, err := p.StreamSynthesize(ctx, &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if err != nil {
			t.Fatalf("StreamSynthesize() error = %v", err)
		}

		cancel()

		done := make(chan error, 1)
		go func() {
			_, derr := core.DrainSpeechStream(context.Background(), stream)
			done <- derr
		}()

		select {
		case derr := <-done:
			if !errors.Is(derr, context.Canceled) {
				t.Fatalf("DrainSpeechStream() err = %v, want context.Canceled", derr)
			}
		case <-time.After(time.Second):
			t.Fatal("stream did not terminate promptly on cancellation")
		}
		if !body.closed.Load() {
			t.Error("response body was not closed on cancellation")
		}
	})

	t.Run("cancellation while blocked on full bounded buffer exits cleanly", func(t *testing.T) {
		// Producer supplies many chunks; consumer intentionally never reads Ch.
		body := &trackingReadCloser{
			ReadCloser: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 1024))),
		}
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       body,
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client), WithStreamChunkSize(4), WithStreamBufferSize(1))
		ctx, cancel := context.WithCancel(context.Background())

		stream, err := p.StreamSynthesize(ctx, &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if err != nil {
			t.Fatalf("StreamSynthesize() error = %v", err)
		}

		// Allow producer to fill the 1-slot channel and block on send.
		time.Sleep(15 * time.Millisecond)
		cancel()

		select {
		case streamErr, ok := <-stream.Err:
			if !ok || !errors.Is(streamErr, context.Canceled) {
				t.Fatalf("stream.Err = (%v, %v), want context.Canceled", streamErr, ok)
			}
		case <-time.After(time.Second):
			t.Fatal("producer goroutine leaked when consumer abandoned full channel")
		}
		// Final must also be closed without value.
		if resp, ok := <-stream.Final; ok || resp != nil {
			t.Errorf("Final = (%v, %v), want closed nil", resp, ok)
		}
		if !body.closed.Load() {
			t.Error("response body was not closed on full-buffer cancellation")
		}
	})

	t.Run("provider timeout mid-stream reports DeadlineExceeded and cleans up", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		body := &trackingReadCloser{ReadCloser: pr}

		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       body,
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client), WithTimeout(25*time.Millisecond))
		stream, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if err != nil {
			t.Fatalf("StreamSynthesize() setup error = %v", err)
		}

		_, derr := core.DrainSpeechStream(context.Background(), stream)
		if !errors.Is(derr, context.DeadlineExceeded) {
			t.Fatalf("DrainSpeechStream() err = %v, want context.DeadlineExceeded", derr)
		}
		if !body.closed.Load() {
			t.Error("response body was not closed on provider timeout")
		}
	})
}

func TestStreamSynthesizeErrorPropagation(t *testing.T) {
	t.Run("preflight validation, unsupported format, and empty key fail before HTTP", func(t *testing.T) {
		hit := false
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				hit = true
				return nil, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		if _, err := p.StreamSynthesize(context.Background(), nil); !errors.Is(err, core.ErrInvalidSpeechRequest) {
			t.Errorf("nil req err = %v, want ErrInvalidSpeechRequest", err)
		}
		if _, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi", Format: core.AudioFormatWAV16000}); !errors.Is(err, core.ErrInvalidAudioFormat) {
			t.Errorf("unsupported wav err = %v, want ErrInvalidAudioFormat", err)
		}

		pEmpty := New("", WithHTTPClient(client))
		if _, err := pEmpty.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"}); !errors.Is(err, core.ErrUnauthorized) {
			t.Errorf("empty key err = %v, want ErrUnauthorized", err)
		}
		if hit {
			t.Error("preflight failures must not issue HTTP calls")
		}
	})

	t.Run("HTTP setup error returns typed ProviderError", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"4"}, "Request-Id": []string{"req-429"}},
					Body:       io.NopCloser(strings.NewReader(`{"detail":"too many requests"}`)),
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		_, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if !errors.Is(err, core.ErrRateLimited) {
			t.Fatalf("setup 429 err = %v, want ErrRateLimited", err)
		}
	})

	t.Run("empty 200 stream emits ErrDecode", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		stream, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if err != nil {
			t.Fatalf("StreamSynthesize() setup error = %v", err)
		}
		_, derr := core.DrainSpeechStream(context.Background(), stream)
		if !errors.Is(derr, core.ErrDecode) {
			t.Fatalf("empty stream err = %v, want ErrDecode", derr)
		}
	})

	t.Run("mid-stream read error propagates ErrNetwork and closes body", func(t *testing.T) {
		reader := &partialFailReader{
			chunks: [][]byte{[]byte("part1")},
			err:    errors.New("connection reset by peer"),
		}
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       reader,
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		stream, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if err != nil {
			t.Fatalf("StreamSynthesize() setup error = %v", err)
		}
		_, derr := core.DrainSpeechStream(context.Background(), stream)
		if !errors.Is(derr, core.ErrNetwork) {
			t.Fatalf("mid-stream read err = %v, want ErrNetwork", derr)
		}
		if !reader.closed.Load() {
			t.Error("response body was not closed after mid-stream error")
		}
	})
}
