package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stallingStreamProvider sends exactly one chunk, then stalls forever
// without sending more chunks and without closing any channel — until its
// ctx is cancelled, at which point it closes Ch, Err, and Final promptly, as
// the ChatStream contract in streaming.go requires. This models a real
// provider whose connection has gone silent mid-stream: the idle watchdog's
// cancel() call is what unblocks it.
type stallingStreamProvider struct{}

func (stallingStreamProvider) ID() string                    { return "stalling" }
func (stallingStreamProvider) Models() []ModelInfo           { return nil }
func (stallingStreamProvider) Supports(feature Feature) bool { return false }
func (stallingStreamProvider) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotSupported
}
func (stallingStreamProvider) StreamChat(ctx context.Context, _ *ChatRequest) (*ChatStream, error) {
	ch := make(chan ChatChunk, 1)
	errc := make(chan error, 1)
	final := make(chan *ChatResponse, 1)

	ch <- ChatChunk{Delta: "hello"}

	go func() {
		// Stall until the watchdog cancels ctx, then shut down promptly.
		<-ctx.Done()
		close(ch)
		close(errc)
		close(final)
	}()

	return &ChatStream{Ch: ch, Err: errc, Final: final}, nil
}

func TestStreamIdleTimeoutFiresOnStall(t *testing.T) {
	c := NewClient(stallingStreamProvider{}, WithStreamIdleTimeout(50*time.Millisecond))

	stream, err := c.Chat("m").User("hi").Stream(context.Background())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	done := make(chan struct{})
	var drainErr error
	go func() {
		_, drainErr = DrainStream(context.Background(), stream)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DrainStream did not return within 2s of the idle window elapsing")
	}

	if !errors.Is(drainErr, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", drainErr)
	}
	if !errors.Is(drainErr, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", drainErr)
	}
}

// tickingStreamProvider sends n chunks roughly every interval, then closes
// cleanly with a Final response. Used to prove a steady trickle of chunks
// keeps resetting the idle timer so the stream is never cut short.
type tickingStreamProvider struct {
	n        int
	interval time.Duration
}

func (tickingStreamProvider) ID() string                    { return "ticking" }
func (tickingStreamProvider) Models() []ModelInfo           { return nil }
func (tickingStreamProvider) Supports(feature Feature) bool { return false }
func (tickingStreamProvider) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotSupported
}
func (p tickingStreamProvider) StreamChat(ctx context.Context, _ *ChatRequest) (*ChatStream, error) {
	ch := make(chan ChatChunk)
	errc := make(chan error, 1)
	final := make(chan *ChatResponse, 1)

	go func() {
		defer close(ch)
		defer close(errc)
		defer close(final)
		for i := 0; i < p.n; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(p.interval):
			}
			select {
			case ch <- ChatChunk{Delta: "x"}:
			case <-ctx.Done():
				return
			}
		}
		final <- &ChatResponse{Output: "xxxxx"}
	}()

	return &ChatStream{Ch: ch, Err: errc, Final: final}, nil
}

func TestStreamIdleTimeoutDoesNotFireWithSteadyChunks(t *testing.T) {
	c := NewClient(tickingStreamProvider{n: 5, interval: 10 * time.Millisecond},
		WithStreamIdleTimeout(50*time.Millisecond))

	stream, err := c.Chat("m").User("hi").Stream(context.Background())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	done := make(chan struct{})
	var resp *ChatResponse
	var drainErr error
	go func() {
		resp, drainErr = DrainStream(context.Background(), stream)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DrainStream did not complete within 2s")
	}

	if drainErr != nil {
		t.Fatalf("unexpected error: %v", drainErr)
	}
	if resp.Output != "xxxxx" {
		t.Errorf("Output = %q, want %q", resp.Output, "xxxxx")
	}
}

func TestWithStreamIdleTimeoutOption(t *testing.T) {
	c := NewClient(&mockProvider{}, WithStreamIdleTimeout(30*time.Second))
	if c.streamIdleTimeout != 30*time.Second {
		t.Errorf("streamIdleTimeout = %v, want 30s", c.streamIdleTimeout)
	}
}

func TestStreamIdleTimeoutDisabledByDefault(t *testing.T) {
	c := NewClient(&mockProvider{})
	if c.streamIdleTimeout != 0 {
		t.Errorf("streamIdleTimeout = %v, want 0 (disabled)", c.streamIdleTimeout)
	}
}
