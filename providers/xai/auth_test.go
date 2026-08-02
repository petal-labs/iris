package xai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestChatEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	p := New("", WithBaseURL(srv.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model:    ModelGrok43,
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}

func TestStreamChatEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	p := New("", WithBaseURL(srv.URL))
	_, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    ModelGrok43,
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}
