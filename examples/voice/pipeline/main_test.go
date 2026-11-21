package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/petal-labs/iris/core"
)

type mockLLM struct{}

func (m *mockLLM) ID() string               { return "mock-llm" }
func (m *mockLLM) Models() []core.ModelInfo { return nil }
func (m *mockLLM) Supports(f core.Feature) bool {
	return f == core.FeatureChat || f == core.FeatureChatStreaming
}
func (m *mockLLM) Chat(context.Context, *core.ChatRequest) (*core.ChatResponse, error) {
	return &core.ChatResponse{Output: "Go compiles fast."}, nil
}
func (m *mockLLM) StreamChat(ctx context.Context, _ *core.ChatRequest) (*core.ChatStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ch := make(chan core.ChatChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ChatResponse, 1)
	ch <- core.ChatChunk{Delta: "Go compiles "}
	ch <- core.ChatChunk{Delta: "fast."}
	finalCh <- &core.ChatResponse{Output: "Go compiles fast."}
	close(ch)
	close(errCh)
	close(finalCh)
	return &core.ChatStream{Ch: ch, Err: errCh, Final: finalCh}, nil
}

type mockRealtimeTTS struct{}

func (m *mockRealtimeTTS) StartRealtimeSynthesis(_ context.Context, cfg core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error) {
	return &mockTTSSession{
		format: cfg.Format,
		audio:  make(chan core.AudioChunk, 8),
		errCh:  make(chan error, 1),
	}, nil
}

type mockTTSSession struct {
	format    core.AudioFormat
	audio     chan core.AudioChunk
	errCh     chan error
	closeOnce sync.Once
}

func (s *mockTTSSession) SendText(_ context.Context, text string) error {
	s.audio <- core.AudioChunk{Data: []byte("pcm:" + text), Format: s.format}
	return nil
}
func (s *mockTTSSession) Flush(context.Context) error   { return nil }
func (s *mockTTSSession) Finish(context.Context) error  { return s.Close() }
func (s *mockTTSSession) Audio() <-chan core.AudioChunk { return s.audio }
func (s *mockTTSSession) Err() <-chan error             { return s.errCh }
func (s *mockTTSSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.audio)
		close(s.errCh)
	})
	return nil
}

func TestRunVoicePipelineSuccessFailureAndCancellation(t *testing.T) {
	t.Run("streams LLM tokens through realtime TTS and writes audio", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "reply.pcm")
		var stdout bytes.Buffer

		err := runVoicePipeline(context.Background(), nil, &mockLLM{}, &mockRealtimeTTS{}, pipelineExampleConfig{
			Prompt:   "Tell me about Go",
			OutPath:  outPath,
			VoiceID:  defaultVoiceID,
			LLMModel: "test-model",
		}, &stdout)
		if err != nil {
			t.Fatalf("runVoicePipeline() err = %v", err)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("ReadFile() err = %v", err)
		}
		if string(data) != "pcm:Go compiles pcm:fast." {
			t.Errorf("written audio = %q, want %q", data, "pcm:Go compiles pcm:fast.")
		}
		if !strings.Contains(stdout.String(), "Assistant: Go compiles fast.") {
			t.Errorf("stdout missing assistant output: %s", stdout.String())
		}
	})

	t.Run("missing prompt and pcm-in fails fast", func(t *testing.T) {
		err := runVoicePipeline(context.Background(), nil, &mockLLM{}, &mockRealtimeTTS{}, pipelineExampleConfig{
			OutPath:  filepath.Join(t.TempDir(), "reply.pcm"),
			VoiceID:  defaultVoiceID,
			LLMModel: "test-model",
		}, io.Discard)
		if err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("cancelled context returns context.Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := runVoicePipeline(ctx, nil, &mockLLM{}, &mockRealtimeTTS{}, pipelineExampleConfig{
			Prompt:   "Hello",
			OutPath:  filepath.Join(t.TempDir(), "reply.pcm"),
			VoiceID:  defaultVoiceID,
			LLMModel: "test-model",
		}, io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
