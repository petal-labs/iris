package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestToolCallAssembler(t *testing.T) {
	a := newToolCallAssembler()

	// Start a tool use
	a.startToolUse(0, "tool_123", "get_weather")

	// Add fragments
	a.addFragment(0, `{"location":`)
	a.addFragment(0, `"NYC"}`)

	// Finalize
	calls, err := a.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("calls count = %d, want 1", len(calls))
	}

	if calls[0].ID != "tool_123" {
		t.Errorf("ID = %q, want 'tool_123'", calls[0].ID)
	}

	if calls[0].Name != "get_weather" {
		t.Errorf("Name = %q, want 'get_weather'", calls[0].Name)
	}

	if string(calls[0].Arguments) != `{"location":"NYC"}` {
		t.Errorf("Arguments = %s, want '{\"location\":\"NYC\"}'", calls[0].Arguments)
	}
}

func TestToolCallAssemblerEmpty(t *testing.T) {
	a := newToolCallAssembler()

	calls, err := a.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if len(calls) != 0 {
		t.Errorf("calls count = %d, want 0", len(calls))
	}
}

func TestToolCallAssemblerEmptyArgs(t *testing.T) {
	a := newToolCallAssembler()
	a.startToolUse(0, "tool_123", "no_args_tool")

	calls, err := a.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("calls count = %d, want 1", len(calls))
	}

	// Empty args should default to {}
	if string(calls[0].Arguments) != "{}" {
		t.Errorf("Arguments = %s, want '{}'", calls[0].Arguments)
	}
}

func TestToolCallAssemblerInvalidJSON(t *testing.T) {
	a := newToolCallAssembler()
	a.startToolUse(0, "tool_123", "get_weather")
	a.addFragment(0, `{invalid json`)

	_, err := a.finalize()
	if err == nil {
		t.Fatal("finalize() should return error for invalid JSON")
	}

	if err != ErrToolArgsInvalidJSON {
		t.Errorf("error = %v, want ErrToolArgsInvalidJSON", err)
	}
}

func TestToolCallAssemblerMultipleTools(t *testing.T) {
	a := newToolCallAssembler()

	a.startToolUse(0, "tool_1", "weather")
	a.startToolUse(1, "tool_2", "time")

	a.addFragment(0, `{"city":"NYC"}`)
	a.addFragment(1, `{"timezone":"EST"}`)

	calls, err := a.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("calls count = %d, want 2", len(calls))
	}

	if calls[0].Name != "weather" {
		t.Errorf("first call Name = %q, want 'weather'", calls[0].Name)
	}

	if calls[1].Name != "time" {
		t.Errorf("second call Name = %q, want 'time'", calls[1].Name)
	}
}

func TestDoStreamChat(t *testing.T) {
	// Create a mock server that returns a streaming response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q, want 'test-key'", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != DefaultVersion {
			t.Errorf("anthropic-version = %q, want %q", r.Header.Get("anthropic-version"), DefaultVersion)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want 'application/json'", r.Header.Get("Content-Type"))
		}

		// Verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading body: %v", err)
		}

		var req anthropicRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("unmarshaling request: %v", err)
		}

		if !req.Stream {
			t.Error("Stream should be true")
		}

		// Write SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("request-id", "req_stream_123")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_stream","model":"claude-sonnet-4-5","usage":{"input_tokens":10,"output_tokens":0}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world!"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}

		for _, line := range events {
			w.Write([]byte(line + "\n"))
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
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

	if finalResp.ID != "msg_stream" {
		t.Errorf("ID = %q, want 'msg_stream'", finalResp.ID)
	}

	if finalResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", finalResp.Usage.PromptTokens)
	}

	if finalResp.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", finalResp.Usage.CompletionTokens)
	}
}

func TestDoStreamChatWithToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_tool","model":"claude-sonnet-4-5","usage":{"input_tokens":20,"output_tokens":0}}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool_abc","name":"get_weather"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"NYC\"}"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}

		for _, line := range events {
			w.Write([]byte(line + "\n"))
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather?"},
		},
	}

	stream, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunks (there shouldn't be any text)
	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	if len(chunks) != 0 {
		t.Errorf("chunks count = %d, want 0 (tool use only)", len(chunks))
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

	if finalResp == nil {
		t.Fatal("finalResp is nil")
	}

	if len(finalResp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(finalResp.ToolCalls))
	}

	tc := finalResp.ToolCalls[0]
	if tc.ID != "tool_abc" {
		t.Errorf("ToolCall ID = %q, want 'tool_abc'", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want 'get_weather'", tc.Name)
	}
	if string(tc.Arguments) != `{"location":"NYC"}` {
		t.Errorf("ToolCall Arguments = %s, want '{\"location\":\"NYC\"}'", tc.Arguments)
	}
}

func TestDoStreamChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`))
	}))
	defer server.Close()

	p := New("bad-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
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
