package gemini

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestDoStreamChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL contains streaming endpoint
		if !strings.Contains(r.URL.Path, ":streamGenerateContent") {
			t.Errorf("Path should contain ':streamGenerateContent', got %q", r.URL.Path)
		}
		if r.URL.Query().Get("alt") != "sse" {
			t.Errorf("alt query param = %q, want 'sse'", r.URL.Query().Get("alt"))
		}

		// Write SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`,
			``,
			`data: {"candidates":[{"content":{"parts":[{"text":" world!"}]}}]}`,
			``,
			`data: {"candidates":[{"content":{"parts":[{"text":""}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`,
			``,
		}

		for _, line := range events {
			w.Write([]byte(line + "\n"))
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	stream, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Collect chunks
	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	// Check for errors
	var streamErr error
	select {
	case e := <-stream.Err:
		streamErr = e
	default:
	}
	if streamErr != nil {
		t.Errorf("stream error = %v", streamErr)
	}

	// Get final response
	var finalResp *core.ChatResponse
	select {
	case r := <-stream.Final:
		finalResp = r
	default:
	}

	// Verify chunks
	if len(chunks) != 2 {
		t.Errorf("chunks count = %d, want 2", len(chunks))
	}

	accumulated := strings.Join(chunks, "")
	if accumulated != "Hello world!" {
		t.Errorf("accumulated = %q, want 'Hello world!'", accumulated)
	}

	// Verify final response
	if finalResp == nil {
		t.Fatal("finalResp is nil")
	}

	if finalResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", finalResp.Usage.PromptTokens)
	}

	if finalResp.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", finalResp.Usage.CompletionTokens)
	}
}

func TestDoStreamChatWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"location":"NYC"}}}]}}]}`,
			``,
			`data: {"candidates":[{"content":{"parts":[]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10}}`,
			``,
		}

		for _, line := range events {
			w.Write([]byte(line + "\n"))
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather?"},
		},
	}

	stream, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunks (should be none for tool-only response)
	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	if len(chunks) != 0 {
		t.Errorf("chunks count = %d, want 0 (tool use only)", len(chunks))
	}

	// Get final response
	var finalResp *core.ChatResponse
	select {
	case r := <-stream.Final:
		finalResp = r
	default:
	}

	if finalResp == nil {
		t.Fatal("finalResp is nil")
	}

	if len(finalResp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(finalResp.ToolCalls))
	}

	tc := finalResp.ToolCalls[0]
	if tc.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want 'get_weather'", tc.Name)
	}
}

func TestDoStreamChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`))
	}))
	defer server.Close()

	p := New("bad-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	_, err := p.StreamChat(context.Background(), req)
	if err == nil {
		t.Fatal("StreamChat() should return error")
	}

	var provErr *core.ProviderError
	if errors.As(err, &provErr) {
		if provErr.Message != "Invalid API key" {
			t.Errorf("error message = %q, want 'Invalid API key'", provErr.Message)
		}
	}
}
