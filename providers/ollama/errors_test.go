package ollama

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func assertNormalizedError(t *testing.T, err, sentinel error) {
	t.Helper()

	var providerErr *core.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %v (%T), want *core.ProviderError", err, err)
	}
	if providerErr.Provider != "ollama" {
		t.Errorf("Provider = %q, want ollama", providerErr.Provider)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want sentinel %v", err, sentinel)
	}
}

func TestErrorConformance(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		sentinel error
	}{
		{name: "network", err: newNetworkError(errors.New("connection refused")), sentinel: core.ErrNetwork},
		{name: "decode", err: newDecodeError(errors.New("invalid JSON")), sentinel: core.ErrDecode},
		{name: "not found", err: mapOllamaError(http.StatusNotFound, "model not found"), sentinel: core.ErrNotFound},
		{name: "stream", err: newStreamError("model crashed"), sentinel: core.ErrServer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNormalizedError(t, tt.err, tt.sentinel)
		})
	}
}

func TestChatMalformedResponseIsNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not JSON"))
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL))
	_, err := provider.Chat(context.Background(), &core.ChatRequest{Model: "llama3.2"})
	assertNormalizedError(t, err, core.ErrDecode)
}

func TestStreamMalformedChunkIsNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("not JSON\n"))
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL))
	stream, err := provider.StreamChat(context.Background(), &core.ChatRequest{Model: "llama3.2"})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	_, err = core.DrainStream(context.Background(), stream)
	assertNormalizedError(t, err, core.ErrDecode)
}

func TestErrorResponseReadFailureIsNormalized(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(errorReader{}),
	}

	err := parseErrorResponse(response)
	assertNormalizedError(t, err, core.ErrNetwork)

	var providerErr *core.ProviderError
	errors.As(err, &providerErr)
	if providerErr.Status != http.StatusBadGateway {
		t.Errorf("Status = %d, want %d", providerErr.Status, http.StatusBadGateway)
	}
	if providerErr.Code != "read_error" {
		t.Errorf("Code = %q, want read_error", providerErr.Code)
	}
}

func TestRequestCreationFailuresAreNormalized(t *testing.T) {
	provider := New(WithBaseURL("://invalid"))
	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "chat",
			call: func() error {
				_, err := provider.Chat(context.Background(), &core.ChatRequest{Model: "llama3.2"})
				return err
			},
		},
		{
			name: "stream",
			call: func() error {
				_, err := provider.StreamChat(context.Background(), &core.ChatRequest{Model: "llama3.2"})
				return err
			},
		},
		{
			name: "embeddings",
			call: func() error {
				_, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{Model: "nomic-embed-text"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNormalizedError(t, tt.call(), core.ErrNetwork)
		})
	}
}

func TestStreamReadFailureIsNormalized(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(errorReader{}),
		}, nil
	})}
	provider := New(WithHTTPClient(client))

	stream, err := provider.StreamChat(context.Background(), &core.ChatRequest{Model: "llama3.2"})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	_, err = core.DrainStream(context.Background(), stream)
	assertNormalizedError(t, err, core.ErrNetwork)
}

func TestInlineErrorsAreNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/api/chat":
			_, _ = w.Write([]byte(`{"error":"chat failed"}`))
		case embedPath:
			_, _ = w.Write([]byte(`{"error":"embedding failed"}`))
		}
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL))
	_, chatErr := provider.Chat(context.Background(), &core.ChatRequest{Model: "llama3.2"})
	assertNormalizedError(t, chatErr, core.ErrServer)

	_, embeddingErr := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{Model: "nomic-embed-text"})
	assertNormalizedError(t, embeddingErr, core.ErrServer)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
