package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type clientEmbeddingProvider struct {
	*capabilityBaseProvider
	createEmbeddings func(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error)
	calls            int
}

func (p *clientEmbeddingProvider) CreateEmbeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	p.calls++
	return p.createEmbeddings(ctx, req)
}

type embeddingTelemetryHook struct {
	starts []RequestStartEvent
	ends   []RequestEndEvent
}

func (h *embeddingTelemetryHook) OnRequestStart(event RequestStartEvent) {
	h.starts = append(h.starts, event)
}

func (h *embeddingTelemetryHook) OnRequestEnd(event RequestEndEvent) {
	h.ends = append(h.ends, event)
}

type embeddingContextKey struct{}

type contextualEmbeddingTelemetryHook struct {
	embeddingTelemetryHook
	endSawContext bool
}

func (h *contextualEmbeddingTelemetryHook) OnRequestStartWithContext(ctx context.Context, event RequestStartEvent) context.Context {
	h.OnRequestStart(event)
	return context.WithValue(ctx, embeddingContextKey{}, "telemetry")
}

func (h *contextualEmbeddingTelemetryHook) OnRequestEndWithContext(ctx context.Context, event RequestEndEvent) {
	h.endSawContext = ctx.Value(embeddingContextKey{}) == "telemetry"
	h.OnRequestEnd(event)
}

type embeddingNoRetryPolicy struct{}

func (embeddingNoRetryPolicy) NextDelay(int, error) (time.Duration, bool) {
	return 0, false
}

func TestClientEmbedDelegatesAndEmitsTelemetry(t *testing.T) {
	request := &EmbeddingRequest{
		Model: "embedding-model",
		Input: []EmbeddingInput{{Text: "hello"}},
	}
	want := &EmbeddingResponse{
		Model: "embedding-model",
		Usage: EmbeddingUsage{PromptTokens: 7, TotalTokens: 7},
	}
	provider := &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "embedding-provider"},
		createEmbeddings: func(_ context.Context, got *EmbeddingRequest) (*EmbeddingResponse, error) {
			if got != request {
				t.Error("Client.Embed() did not pass the original request")
			}
			return want, nil
		},
	}
	hook := &embeddingTelemetryHook{}
	client := NewClient(provider, WithTelemetry(hook))

	response, err := client.Embed(context.Background(), request)
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if response != want || provider.calls != 1 {
		t.Fatalf("Embed() = (%v, calls=%d), want response and one call", response, provider.calls)
	}
	assertEmbeddingTelemetry(t, hook, "embedding-provider", "embedding-model", 7, nil)
}

func TestClientEmbedRejectsUnsupportedProvider(t *testing.T) {
	hook := &embeddingTelemetryHook{}
	client := NewClient(&capabilityBaseProvider{id: "chat-only"}, WithTelemetry(hook))

	_, err := client.Embed(context.Background(), &EmbeddingRequest{Model: "model"})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Embed() error = %v, want ErrNotSupported", err)
	}
	if len(hook.starts) != 0 || len(hook.ends) != 0 {
		t.Fatal("unsupported capability should fail before telemetry")
	}
}

func TestClientEmbedRejectsNilRequest(t *testing.T) {
	provider := &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "embedding-provider"},
		createEmbeddings: func(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error) {
			t.Fatal("provider should not receive a nil request")
			return nil, nil
		},
	}

	_, err := NewClient(provider).Embed(context.Background(), nil)
	if err == nil {
		t.Fatal("Embed() should reject a nil request")
	}
}

func TestClientEmbedEmitsTelemetryOnProviderError(t *testing.T) {
	provider := &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "embedding-provider"},
		createEmbeddings: func(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error) {
			return nil, ErrBadRequest
		},
	}
	hook := &embeddingTelemetryHook{}
	client := NewClient(provider, WithTelemetry(hook), WithRetryPolicy(embeddingNoRetryPolicy{}))

	_, err := client.Embed(context.Background(), &EmbeddingRequest{Model: "embedding-model"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("Embed() error = %v, want ErrBadRequest", err)
	}
	assertEmbeddingTelemetry(t, hook, "embedding-provider", "embedding-model", 0, ErrBadRequest)
}

func TestClientEmbedPropagatesContextualTelemetryContext(t *testing.T) {
	hook := &contextualEmbeddingTelemetryHook{}
	provider := &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "embedding-provider"},
		createEmbeddings: func(ctx context.Context, _ *EmbeddingRequest) (*EmbeddingResponse, error) {
			if ctx.Value(embeddingContextKey{}) != "telemetry" {
				t.Error("provider did not receive telemetry context")
			}
			return &EmbeddingResponse{}, nil
		},
	}
	client := NewClient(provider, WithTelemetry(hook))

	_, err := client.Embed(context.Background(), &EmbeddingRequest{Model: "embedding-model"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if !hook.endSawContext {
		t.Error("contextual telemetry end hook did not receive telemetry context")
	}
}

func TestClientEmbedRetriesTransientErrors(t *testing.T) {
	provider := &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "embedding-provider"},
	}
	provider.createEmbeddings = func(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error) {
		if provider.calls == 1 {
			return nil, ErrNetwork
		}
		return &EmbeddingResponse{}, nil
	}
	retry := NewRetryPolicy(RetryConfig{MaxRetries: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Microsecond})
	client := NewClient(provider, WithRetryPolicy(retry))

	_, err := client.Embed(context.Background(), &EmbeddingRequest{Model: "embedding-model"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
}

func TestClientEmbedCanDisableClientTimeout(t *testing.T) {
	provider := &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "embedding-provider"},
		createEmbeddings: func(ctx context.Context, _ *EmbeddingRequest) (*EmbeddingResponse, error) {
			if _, hasDeadline := ctx.Deadline(); hasDeadline {
				t.Error("WithTimeout(0) should not add a deadline")
			}
			return &EmbeddingResponse{}, nil
		},
	}
	client := NewClient(provider, WithTimeout(0))

	_, err := client.Embed(context.Background(), &EmbeddingRequest{Model: "embedding-model"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
}

func TestClientEmbedAppliesClientTimeout(t *testing.T) {
	provider := blockingEmbeddingProvider()
	client := NewClient(provider, WithTimeout(20*time.Millisecond), WithRetryPolicy(embeddingNoRetryPolicy{}))

	_, err := client.Embed(context.Background(), &EmbeddingRequest{Model: "embedding-model"})
	if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Embed() error = %v, want ErrTimeout wrapping DeadlineExceeded", err)
	}
}

func TestClientEmbedPreservesCallerDeadline(t *testing.T) {
	provider := blockingEmbeddingProvider()
	client := NewClient(provider, WithTimeout(time.Second), WithRetryPolicy(embeddingNoRetryPolicy{}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.Embed(ctx, &EmbeddingRequest{Model: "embedding-model"})
	if errors.Is(err, ErrTimeout) {
		t.Fatalf("Embed() remapped caller deadline to ErrTimeout: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Embed() error = %v, want context.DeadlineExceeded", err)
	}
}

func blockingEmbeddingProvider() *clientEmbeddingProvider {
	return &clientEmbeddingProvider{
		capabilityBaseProvider: &capabilityBaseProvider{id: "blocking-embedding-provider"},
		createEmbeddings: func(ctx context.Context, _ *EmbeddingRequest) (*EmbeddingResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
}

func assertEmbeddingTelemetry(t *testing.T, hook *embeddingTelemetryHook, provider string, model ModelID, tokens int, wantErr error) {
	t.Helper()
	if len(hook.starts) != 1 || len(hook.ends) != 1 {
		t.Fatalf("telemetry events = (%d starts, %d ends), want one each", len(hook.starts), len(hook.ends))
	}
	if hook.starts[0].Provider != provider || hook.starts[0].Model != model {
		t.Errorf("start event = %+v, want provider=%s model=%s", hook.starts[0], provider, model)
	}
	end := hook.ends[0]
	if end.Provider != provider || end.Model != model || end.Usage.PromptTokens != tokens || end.Usage.TotalTokens != tokens {
		t.Errorf("end event = %+v, want provider=%s model=%s tokens=%d", end, provider, model, tokens)
	}
	if !errors.Is(end.Err, wantErr) {
		t.Errorf("end error = %v, want %v", end.Err, wantErr)
	}
}
