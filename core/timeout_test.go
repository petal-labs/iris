package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewClientDefaultTimeout(t *testing.T) {
	c := NewClient(&mockProvider{})
	if c.timeout != DefaultTimeout {
		t.Errorf("default timeout = %v, want %v", c.timeout, DefaultTimeout)
	}
}

func TestWithTimeoutOverridesDefault(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(5*time.Second))
	if c.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", c.timeout)
	}
}

func TestWithTimeoutZeroDisables(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(0))
	if c.timeout != 0 {
		t.Errorf("timeout = %v, want 0 (disabled)", c.timeout)
	}
}

func TestEffectiveTimeoutPrecedence(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(30*time.Second))

	// Caller ctx has a deadline -> 0 (caller wins).
	ctxWithDeadline, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b1 := c.Chat("m")
	if got := b1.effectiveTimeout(ctxWithDeadline); got != 0 {
		t.Errorf("with caller deadline = %v, want 0", got)
	}

	// Builder timeout set -> builder wins over client default.
	b2 := c.Chat("m")
	b2.timeout = 10 * time.Second
	if got := b2.effectiveTimeout(context.Background()); got != 10*time.Second {
		t.Errorf("builder override = %v, want 10s", got)
	}

	// Neither -> client default.
	b3 := c.Chat("m")
	if got := b3.effectiveTimeout(context.Background()); got != 30*time.Second {
		t.Errorf("client default = %v, want 30s", got)
	}
}

// noRetryPolicy is a retry policy that never retries.
type noRetryPolicy struct{}

func (noRetryPolicy) NextDelay(attempt int, err error) (time.Duration, bool) {
	return 0, false
}

// blockingProvider blocks Chat until ctx is cancelled, then returns ctx.Err().
type blockingProvider struct{}

func (blockingProvider) ID() string                    { return "blocking" }
func (blockingProvider) Models() []ModelInfo           { return nil }
func (blockingProvider) Supports(feature Feature) bool { return false }
func (blockingProvider) Chat(ctx context.Context, _ *ChatRequest) (*ChatResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (blockingProvider) StreamChat(ctx context.Context, _ *ChatRequest) (*ChatStream, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestGetResponseTimesOutWithErrTimeout(t *testing.T) {
	c := NewClient(blockingProvider{}, WithTimeout(50*time.Millisecond),
		WithRetryPolicy(noRetryPolicy{}))

	_, err := c.Chat("m").User("hi").GetResponse(context.Background())

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap DeadlineExceeded", err)
	}
}

func TestGetResponseCallerDeadlineNotRemapped(t *testing.T) {
	c := NewClient(blockingProvider{}, WithRetryPolicy(noRetryPolicy{}))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Chat("m").User("hi").GetResponse(ctx)

	if errors.Is(err, ErrTimeout) {
		t.Errorf("caller deadline was remapped to ErrTimeout; want raw context error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestGetResponseDisabledTimeoutDoesNotFire(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(0))
	// mockProvider returns promptly; assert no ErrTimeout on the happy path.
	_, err := c.Chat("m").User("hi").GetResponse(context.Background())
	if errors.Is(err, ErrTimeout) {
		t.Errorf("unexpected ErrTimeout with timeout disabled: %v", err)
	}
}

// fastStreamProvider emits one chunk then closes immediately.
type fastStreamProvider struct{}

func (fastStreamProvider) ID() string                    { return "faststream" }
func (fastStreamProvider) Models() []ModelInfo           { return nil }
func (fastStreamProvider) Supports(feature Feature) bool { return false }
func (fastStreamProvider) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotSupported
}
func (fastStreamProvider) StreamChat(_ context.Context, _ *ChatRequest) (*ChatStream, error) {
	ch := make(chan ChatChunk, 1)
	errc := make(chan error)
	final := make(chan *ChatResponse, 1)
	ch <- ChatChunk{Delta: "hello"}
	close(ch)
	final <- &ChatResponse{Output: "hello"}
	close(final)
	close(errc)
	return &ChatStream{Ch: ch, Err: errc, Final: final}, nil
}

func TestStreamFastCompletesNotCancelled(t *testing.T) {
	c := NewClient(fastStreamProvider{}, WithTimeout(50*time.Millisecond))
	stream, err := c.Chat("m").User("hi").Stream(context.Background())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	resp, err := DrainStream(context.Background(), stream)
	if err != nil {
		t.Fatalf("DrainStream error = %v", err)
	}
	if resp.Output != "hello" {
		t.Errorf("Output = %q, want %q", resp.Output, "hello")
	}
}

func TestStreamStalledTimesOut(t *testing.T) {
	// blockingProvider.StreamChat blocks until ctx cancels, then returns ctx.Err()
	// as a setup error (StreamChat returns error, not a stream).
	c := NewClient(blockingProvider{}, WithTimeout(50*time.Millisecond))
	_, err := c.Chat("m").User("hi").Stream(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Stream() err = %v, want ErrTimeout", err)
	}
}

// midStreamTimeoutProvider establishes a real stream (StreamChat returns a
// nil setup error), then blocks until ctx's deadline fires and reports
// ctx.Err() on Err — unlike blockingProvider, which fails synchronously
// during StreamChat setup. This exercises the mid-stream path: the deadline
// error is received inside wrapStreamWithTelemetry's goroutine (off
// stream.Err), passed through mapErr, and onDone fires cancel() when the
// goroutine exits.
type midStreamTimeoutProvider struct{}

func (midStreamTimeoutProvider) ID() string                    { return "midstreamtimeout" }
func (midStreamTimeoutProvider) Models() []ModelInfo           { return nil }
func (midStreamTimeoutProvider) Supports(feature Feature) bool { return false }
func (midStreamTimeoutProvider) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotSupported
}
func (midStreamTimeoutProvider) StreamChat(ctx context.Context, _ *ChatRequest) (*ChatStream, error) {
	ch := make(chan ChatChunk)
	errc := make(chan error, 1)
	final := make(chan *ChatResponse)
	go func() {
		<-ctx.Done()
		errc <- ctx.Err()
		// Give the telemetry-wrapping goroutine a moment to drain and
		// forward this error before closing Ch. DrainStream watches Ch
		// directly (it is passed through unwrapped) and treats its closure
		// as the signal to stop looping; closing it prematurely relative to
		// the wrapper's forwarding of Err would make the test's outcome
		// depend on unrelated goroutine scheduling.
		time.Sleep(5 * time.Millisecond)
		close(ch)
		close(errc)
		close(final)
	}()
	return &ChatStream{Ch: ch, Err: errc, Final: final}, nil
}

func TestStreamMidStreamTimeout(t *testing.T) {
	c := NewClient(midStreamTimeoutProvider{}, WithTimeout(50*time.Millisecond))
	stream, err := c.Chat("m").User("hi").Stream(context.Background())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	_, err = DrainStream(context.Background(), stream)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("DrainStream err = %v, want ErrTimeout", err)
	}
}
