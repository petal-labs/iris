package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

// Helper to create SSE response
func sseResponse(events ...string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString("data: ")
		sb.WriteString(e)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func TestStreamChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send chunks
		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`,
			`{"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`{"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"}}]}`,
			`{"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"!"}}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hi"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Collect deltas
	var deltas []string
	for chunk := range stream.Ch {
		deltas = append(deltas, chunk.Delta)
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	// Get final response
	final := <-stream.Final
	if final == nil {
		t.Fatal("Final response is nil")
	}

	// Verify deltas
	expected := []string{"Hello", " world", "!"}
	if len(deltas) != len(expected) {
		t.Errorf("len(deltas) = %d, want %d", len(deltas), len(expected))
	}
	for i, d := range deltas {
		if i < len(expected) && d != expected[i] {
			t.Errorf("deltas[%d] = %q, want %q", i, d, expected[i])
		}
	}

	// Verify final response
	if final.ID != "chatcmpl-123" {
		t.Errorf("ID = %q, want %q", final.ID, "chatcmpl-123")
	}
	if final.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", final.Model, "gpt-4o")
	}
	if final.Usage.TotalTokens != 13 {
		t.Errorf("Usage.TotalTokens = %d, want 13", final.Usage.TotalTokens)
	}
}

func TestStreamChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Tool call fragments
		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-456","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
			`{"id":"chatcmpl-456","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}`,
			`{"id":"chatcmpl-456","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":"}}]}}]}`,
			`{"id":"chatcmpl-456","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"NYC\"}"}}]}}]}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Weather?"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunks
	for range stream.Ch {
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	// Get final response
	final := <-stream.Final
	if final == nil {
		t.Fatal("Final response is nil")
	}

	if len(final.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(final.ToolCalls))
	}

	tc := final.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", tc.ID, "call_abc")
	}
	if tc.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", tc.Name, "get_weather")
	}
	if string(tc.Arguments) != `{"location":"NYC"}` {
		t.Errorf("ToolCalls[0].Arguments = %s, want %s", tc.Arguments, `{"location":"NYC"}`)
	}
}

func TestStreamChatMultipleToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-789","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"func1","arguments":"{}"}}]}}]}`,
			`{"id":"chatcmpl-789","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"func2","arguments":"{}"}}]}}]}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	for range stream.Ch {
	}

	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	final := <-stream.Final
	if len(final.ToolCalls) != 2 {
		t.Fatalf("len(ToolCalls) = %d, want 2", len(final.ToolCalls))
	}

	if final.ToolCalls[0].Name != "func1" {
		t.Errorf("ToolCalls[0].Name = %q, want func1", final.ToolCalls[0].Name)
	}
	if final.ToolCalls[1].Name != "func2" {
		t.Errorf("ToolCalls[1].Name = %q, want func2", final.ToolCalls[1].Name)
	}
}

func TestStreamChatInvalidToolCallJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-bad","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_bad","type":"function","function":{"name":"broken","arguments":"{invalid"}}]}}]}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	for range stream.Ch {
	}

	// Should get error
	streamErr := <-stream.Err
	if !errors.Is(streamErr, ErrToolArgsInvalidJSON) {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", streamErr)
	}
}

func TestStreamChatError400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Bad request"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestStreamChatError429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if !errors.Is(err, core.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestStreamChatContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send first chunk
		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-cancel","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		))
		w.(http.Flusher).Flush()

		// Wait to simulate slow stream
		time.Sleep(100 * time.Millisecond)

		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-cancel","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"}}]}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(ctx, &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Read first chunk then cancel
	<-stream.Ch
	cancel()

	// Channels should close
	timeout := time.After(500 * time.Millisecond)
	select {
	case <-timeout:
		t.Error("Channels did not close after context cancellation")
	case _, ok := <-stream.Ch:
		if ok {
			// Drain remaining
			for range stream.Ch {
			}
		}
	}
}

func TestStreamChatEmptyStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-empty","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// No content chunks expected
	count := 0
	for range stream.Ch {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 chunks, got %d", count)
	}

	select {
	case err := <-stream.Err:
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	default:
	}

	final := <-stream.Final
	if final == nil {
		t.Error("Final response is nil")
	}
}

func TestStreamChatChannelsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, sseResponse(
			`{"id":"chatcmpl-close","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hi"}}]}`,
			"[DONE]",
		))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "Test"}},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain Ch channel
	for range stream.Ch {
	}

	// Drain Err channel (may have 0 or 1 value)
	for range stream.Err {
	}

	// Drain Final channel (should have exactly 1 value)
	for range stream.Final {
	}

	// Verify all channels are now closed (second receive returns false)
	_, chOpen := <-stream.Ch
	_, errOpen := <-stream.Err
	_, finalOpen := <-stream.Final

	if chOpen {
		t.Error("Ch channel not closed")
	}
	if errOpen {
		t.Error("Err channel not closed")
	}
	if finalOpen {
		t.Error("Final channel not closed")
	}
}

func TestToolCallAssemblerEmpty(t *testing.T) {
	a := newToolCallAssembler()
	calls, err := a.finalize()

	if err != nil {
		t.Errorf("finalize() error = %v", err)
	}
	if calls != nil {
		t.Errorf("finalize() = %v, want nil", calls)
	}
}

func TestToolCallAssemblerSingleCall(t *testing.T) {
	a := newToolCallAssembler()

	a.addFragment(openAIStreamToolCall{
		Index: 0,
		ID:    "call_1",
		Function: openAIStreamFunction{
			Name:      "test",
			Arguments: `{"key":`,
		},
	})

	a.addFragment(openAIStreamToolCall{
		Index: 0,
		Function: openAIStreamFunction{
			Arguments: `"value"}`,
		},
	})

	calls, err := a.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}

	if calls[0].ID != "call_1" {
		t.Errorf("ID = %q, want call_1", calls[0].ID)
	}

	if string(calls[0].Arguments) != `{"key":"value"}` {
		t.Errorf("Arguments = %s, want %s", calls[0].Arguments, `{"key":"value"}`)
	}
}

func TestToolCallAssemblerInvalidJSON(t *testing.T) {
	a := newToolCallAssembler()

	a.addFragment(openAIStreamToolCall{
		Index: 0,
		ID:    "call_bad",
		Function: openAIStreamFunction{
			Name:      "broken",
			Arguments: `{invalid`,
		},
	})

	_, err := a.finalize()
	if !errors.Is(err, ErrToolArgsInvalidJSON) {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", err)
	}
}
