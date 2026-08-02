package voyageai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

// blockingServer returns an httptest server whose handler blocks until the
// request's context is cancelled, simulating a hung upstream. The server is
// closed asynchronously on test cleanup: for a POST with a body, the Go
// standard library does not always observe the client-side connection
// abandonment in time for the handler to unblock, so a synchronous
// server.Close() (which waits for outstanding handlers) can itself hang
// well past the timeout under test. The op-timeout guard is what is under
// test here, not the test server's teardown.
func blockingServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(func() {
		go server.Close()
	})
	return server
}

// validEmbeddingRequest returns a minimal valid embedding request for use in
// timeout tests.
func validEmbeddingRequest() *core.EmbeddingRequest {
	return &core.EmbeddingRequest{
		Model: "voyage-3-large",
		Input: []core.EmbeddingInput{{Text: "hello world"}},
	}
}

func TestCreateEmbeddings_Timeout_OpTimeoutApplies(t *testing.T) {
	server := blockingServer(t)

	provider := New("test-key", WithBaseURL(server.URL), WithTimeout(50*time.Millisecond))

	done := make(chan struct{})
	var err error
	go func() {
		_, err = provider.CreateEmbeddings(context.Background(), validEmbeddingRequest())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CreateEmbeddings did not return within 2s; op timeout was not applied")
	}

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected error chain to satisfy errors.Is(err, context.DeadlineExceeded), got: %v", err)
	}
}

func TestCreateEmbeddings_Timeout_CallerDeadlineHonoredOverLargeTimeout(t *testing.T) {
	server := blockingServer(t)

	// Provider timeout is large; caller supplies a short deadline that
	// should still bound the call.
	provider := New("test-key", WithBaseURL(server.URL), WithTimeout(time.Hour))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = provider.CreateEmbeddings(ctx, validEmbeddingRequest())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CreateEmbeddings did not return within 2s; caller deadline was not honored")
	}

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected error chain to satisfy errors.Is(err, context.DeadlineExceeded), got: %v", err)
	}
}

func TestCreateEmbeddings_Timeout_ZeroDisablesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageEmbeddingResponse{
			Object: "list",
			Data: []voyageEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2, 0.3}},
			},
			Model: "voyage-3-large",
			Usage: voyageEmbeddingUsage{TotalTokens: 2},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL), WithTimeout(0))

	resp, err := provider.CreateEmbeddings(context.Background(), validEmbeddingRequest())
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v, want nil", err)
	}
	if len(resp.Vectors) != 1 {
		t.Fatalf("len(Vectors) = %d, want 1", len(resp.Vectors))
	}
}
