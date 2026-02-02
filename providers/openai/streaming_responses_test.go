package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestResponsesAPIStreamChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("Path = %q, want /responses", r.URL.Path)
		}

		// Verify stream is requested using raw JSON
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		stream, ok := reqBody["stream"].(bool)
		if !ok || !stream {
			t.Error("Expected stream=true in request")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send streaming events
		events := []string{
			`event: response.created` + "\n" + `data: {"type":"response.created","response":{"id":"resp-stream-123","model":"gpt-5.2","status":"in_progress"}}`,
			`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":{"type":"text","text":"Hello"}}`,
			`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":{"type":"text","text":" World"}}`,
			`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":{"type":"text","text":"!"}}`,
			`event: response.completed` + "\n" + `data: {"type":"response.completed","response":{"id":"resp-stream-123","model":"gpt-5.2","status":"completed","usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}}`,
			`data: [DONE]`,
		}

		flusher := w.(http.Flusher)
		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: ModelGPT52,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Say hello"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Collect chunks
	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	// Check final response
	var finalResp *core.ChatResponse
	select {
	case resp := <-stream.Final:
		finalResp = resp
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for final response")
	}

	if finalResp == nil {
		t.Fatal("Expected final response")
	}

	if finalResp.ID != "resp-stream-123" {
		t.Errorf("Final.ID = %q, want %q", finalResp.ID, "resp-stream-123")
	}

	if finalResp.Status != "completed" {
		t.Errorf("Final.Status = %q, want %q", finalResp.Status, "completed")
	}

	// Verify chunks
	expected := []string{"Hello", " World", "!"}
	if len(chunks) != len(expected) {
		t.Fatalf("len(chunks) = %d, want %d", len(chunks), len(expected))
	}

	for i, chunk := range chunks {
		if chunk != expected[i] {
			t.Errorf("chunks[%d] = %q, want %q", i, chunk, expected[i])
		}
	}
}

func TestResponsesAPIStreamChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`event: response.created` + "\n" + `data: {"type":"response.created","response":{"id":"resp-tool-stream","model":"gpt-5.2","status":"in_progress"}}`,
			`event: response.output_item.added` + "\n" + `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call"}}`,
			`event: response.function_call_arguments.delta` + "\n" + `data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":{"arguments":"{\"location\":"}}`,
			`event: response.function_call_arguments.delta` + "\n" + `data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":{"arguments":"\"SF\"}"}}`,
			`event: response.output_item.done` + "\n" + `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","call_id":"call_123","name":"get_weather","arguments":"{\"location\":\"SF\"}"}}`,
			`event: response.completed` + "\n" + `data: {"type":"response.completed","response":{"id":"resp-tool-stream","model":"gpt-5.2","status":"completed","usage":{"input_tokens":10,"output_tokens":15,"total_tokens":25}}}`,
			`data: [DONE]`,
		}

		flusher := w.(http.Flusher)
		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: ModelGPT52,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Weather?"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunk channel
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

	// Check final response
	var finalResp *core.ChatResponse
	select {
	case resp := <-stream.Final:
		finalResp = resp
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for final response")
	}

	if finalResp == nil {
		t.Fatal("Expected final response")
	}

	if len(finalResp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(finalResp.ToolCalls))
	}

	tc := finalResp.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", tc.ID, "call_123")
	}

	if tc.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", tc.Name, "get_weather")
	}
}

func TestResponsesAPIStreamChatWithReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`event: response.created` + "\n" + `data: {"type":"response.created","response":{"id":"resp-reason-stream","model":"gpt-5.2","status":"in_progress"}}`,
			`event: response.output_item.done` + "\n" + `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"rs_123","summary":[{"type":"text","text":"Thinking..."}]}}`,
			`event: response.output_text.delta` + "\n" + `data: {"type":"response.output_text.delta","delta":{"type":"text","text":"Answer"}}`,
			`event: response.completed` + "\n" + `data: {"type":"response.completed","response":{"id":"resp-reason-stream","model":"gpt-5.2","status":"completed","usage":{"input_tokens":5,"output_tokens":10,"total_tokens":15,"reasoning_tokens":5}}}`,
			`data: [DONE]`,
		}

		flusher := w.(http.Flusher)
		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:           ModelGPT52,
		ReasoningEffort: core.ReasoningEffortHigh,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Think hard"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain chunk channel
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

	// Check final response
	var finalResp *core.ChatResponse
	select {
	case resp := <-stream.Final:
		finalResp = resp
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for final response")
	}

	if finalResp == nil {
		t.Fatal("Expected final response")
	}

	if finalResp.Reasoning == nil {
		t.Fatal("Expected reasoning output")
	}

	if len(finalResp.Reasoning.Summary) != 1 {
		t.Fatalf("len(Reasoning.Summary) = %d, want 1", len(finalResp.Reasoning.Summary))
	}

	if finalResp.Reasoning.Summary[0] != "Thinking..." {
		t.Errorf("Reasoning.Summary[0] = %q, want %q", finalResp.Reasoning.Summary[0], "Thinking...")
	}
}

func TestResponsesAPIStreamChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req-stream-err")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"Server error"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: ModelGPT52,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestOlderModelStreamUsesCompletionsAPI(t *testing.T) {
	var calledPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`data: {"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
			`data: {"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hi"}}]}`,
			`data: {"id":"chatcmpl-123","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`,
			`data: [DONE]`,
		}

		flusher := w.(http.Flusher)
		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: ModelGPT4o, // Uses Chat Completions API
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain channels
	for range stream.Ch {
	}

	if calledPath != "/chat/completions" {
		t.Errorf("Called path = %q, want /chat/completions", calledPath)
	}
}
