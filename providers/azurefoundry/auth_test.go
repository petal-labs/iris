package azurefoundry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

// TestBuildHeadersNoAuth verifies that buildHeaders returns ErrNoAuth when
// neither a token credential nor an API key is configured, and that the
// returned error also satisfies errors.Is(err, core.ErrUnauthorized) so
// callers can use the shared sentinel across all providers.
func TestBuildHeadersNoAuth(t *testing.T) {
	p := New("https://test.openai.azure.com", "")

	_, err := p.buildHeaders(context.Background())
	if err == nil {
		t.Fatal("buildHeaders() error = nil, want error")
	}

	if !errors.Is(err, ErrNoAuth) {
		t.Errorf("expected error to wrap ErrNoAuth, got %v", err)
	}

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("expected error to wrap core.ErrUnauthorized, got %v", err)
	}
}

// TestChatNoAuthSatisfiesErrUnauthorized verifies the no-auth error surfaces
// through the public Chat path (empty API key, no token credential) and
// still satisfies both errors.Is(err, ErrNoAuth) and
// errors.Is(err, core.ErrUnauthorized). The request must never reach the
// server since buildHeaders fails first.
func TestChatNoAuthSatisfiesErrUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when no auth is configured")
	}))
	defer server.Close()

	p := New(server.URL, "")

	req := &core.ChatRequest{
		Model: "test-model",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "hello"},
		},
	}

	_, err := p.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("Chat() error = nil, want error")
	}

	if !errors.Is(err, ErrNoAuth) {
		t.Errorf("expected error to wrap ErrNoAuth, got %v", err)
	}

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("expected error to wrap core.ErrUnauthorized, got %v", err)
	}
}
