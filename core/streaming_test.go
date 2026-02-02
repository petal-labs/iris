package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDrainStreamAccumulatesDeltas(t *testing.T) {
	ch := make(chan ChatChunk, 3)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		ch <- ChatChunk{Delta: "Hello"}
		ch <- ChatChunk{Delta: " "}
		ch <- ChatChunk{Delta: "World"}
		close(ch)
		finalCh <- &ChatResponse{
			ID:    "resp-1",
			Model: "gpt-4",
			Usage: TokenUsage{TotalTokens: 10},
		}
		close(finalCh)
		close(errCh)
	}()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	resp, err := DrainStream(context.Background(), stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Output != "Hello World" {
		t.Errorf("Output = %q, want %q", resp.Output, "Hello World")
	}
	if resp.Usage.TotalTokens != 10 {
		t.Errorf("Usage.TotalTokens = %d, want 10", resp.Usage.TotalTokens)
	}
}

func TestDrainStreamUseFinalOutput(t *testing.T) {
	ch := make(chan ChatChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		ch <- ChatChunk{Delta: "partial"}
		close(ch)
		// Final has complete output
		finalCh <- &ChatResponse{
			Output: "Complete response from provider",
			Usage:  TokenUsage{TotalTokens: 20},
		}
		close(finalCh)
		close(errCh)
	}()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	resp, err := DrainStream(context.Background(), stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use Final's output, not accumulated
	if resp.Output != "Complete response from provider" {
		t.Errorf("Output = %q, want complete response", resp.Output)
	}
}

func TestDrainStreamErrorPropagates(t *testing.T) {
	ch := make(chan ChatChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	expectedErr := errors.New("stream error")

	go func() {
		ch <- ChatChunk{Delta: "partial"}
		close(ch)
		errCh <- expectedErr
		close(errCh)
		close(finalCh)
	}()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	_, err := DrainStream(context.Background(), stream)

	if err != expectedErr {
		t.Errorf("err = %v, want %v", err, expectedErr)
	}
}

func TestDrainStreamContextCancellation(t *testing.T) {
	ch := make(chan ChatChunk)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	// Don't send anything - stream blocks

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	_, err := DrainStream(ctx, stream)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestDrainStreamEmptyStream(t *testing.T) {
	ch := make(chan ChatChunk)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		close(ch)
		finalCh <- &ChatResponse{
			ID:    "resp-1",
			Usage: TokenUsage{TotalTokens: 0},
		}
		close(finalCh)
		close(errCh)
	}()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	resp, err := DrainStream(context.Background(), stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Output != "" {
		t.Errorf("Output = %q, want empty string", resp.Output)
	}
}

func TestDrainStreamToolCallsPreserved(t *testing.T) {
	ch := make(chan ChatChunk)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	expectedToolCalls := []ToolCall{
		{ID: "call_1", Name: "get_weather", Arguments: []byte(`{"city":"NYC"}`)},
		{ID: "call_2", Name: "search", Arguments: []byte(`{"q":"test"}`)},
	}

	go func() {
		close(ch)
		finalCh <- &ChatResponse{
			ID:        "resp-1",
			ToolCalls: expectedToolCalls,
		}
		close(finalCh)
		close(errCh)
	}()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	resp, err := DrainStream(context.Background(), stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("len(ToolCalls) = %d, want 2", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %v, want get_weather", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[1].Name != "search" {
		t.Errorf("ToolCalls[1].Name = %v, want search", resp.ToolCalls[1].Name)
	}
}

func TestDrainStreamNilStream(t *testing.T) {
	_, err := DrainStream(context.Background(), nil)
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("err = %v, want ErrBadRequest", err)
	}
}

func TestDrainStreamNoFinalResponse(t *testing.T) {
	ch := make(chan ChatChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		ch <- ChatChunk{Delta: "Hello"}
		ch <- ChatChunk{Delta: " World"}
		close(ch)
		// No final response sent
		close(finalCh)
		close(errCh)
	}()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	resp, err := DrainStream(context.Background(), stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should create response from accumulated content
	if resp.Output != "Hello World" {
		t.Errorf("Output = %q, want %q", resp.Output, "Hello World")
	}
}

func TestDrainStreamWithTimeout(t *testing.T) {
	ch := make(chan ChatChunk)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	// Don't close channels - will timeout

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	stream := &ChatStream{Ch: ch, Err: errCh, Final: finalCh}
	_, err := DrainStream(ctx, stream)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestChatStreamTypeHasCorrectChannelDirections(t *testing.T) {
	// This is a compile-time check - if it compiles, the channel directions are correct
	ch := make(chan ChatChunk)
	errCh := make(chan error)
	finalCh := make(chan *ChatResponse)

	stream := &ChatStream{
		Ch:    ch,      // receive-only in struct
		Err:   errCh,   // receive-only in struct
		Final: finalCh, // receive-only in struct
	}

	if stream == nil {
		t.Fatal("stream should not be nil")
	}
}
