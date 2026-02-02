package zai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestStreamChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send SSE chunks
		chunks := []string{
			`{"id":"task-stream","model":"glm-4.7","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
			`{"id":"task-stream","model":"glm-4.7","choices":[{"index":0,"delta":{"content":" world"}}]}`,
			`{"id":"task-stream","model":"glm-4.7","choices":[{"index":0,"delta":{"content":"!"}}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`,
		}

		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Collect chunks
	var content strings.Builder
	for chunk := range stream.Ch {
		content.WriteString(chunk.Delta)
	}

	if content.String() != "Hello world!" {
		t.Errorf("content = %q, want %q", content.String(), "Hello world!")
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	default:
	}

	// Check final response
	select {
	case resp := <-stream.Final:
		if resp == nil {
			t.Fatal("Final response is nil")
		}
		if resp.ID != "task-stream" {
			t.Errorf("ID = %q, want %q", resp.ID, "task-stream")
		}
		if resp.Usage.TotalTokens != 8 {
			t.Errorf("Usage.TotalTokens = %d, want 8", resp.Usage.TotalTokens)
		}
	default:
		t.Error("No final response received")
	}
}

func TestStreamChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"id":"task-tools","model":"glm-4.7","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
			`{"id":"task-tools","model":"glm-4.7","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":"}}]}}]}`,
			`{"id":"task-tools","model":"glm-4.7","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Beijing\"}"}}]}}]}`,
		}

		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Weather?"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Drain content channel
	for range stream.Ch {
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	default:
	}

	// Check final response
	select {
	case resp := <-stream.Final:
		if resp == nil {
			t.Fatal("Final response is nil")
		}
		if len(resp.ToolCalls) != 1 {
			t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
		}
		if resp.ToolCalls[0].Name != "get_weather" {
			t.Errorf("ToolCalls[0].Name = %q, want get_weather", resp.ToolCalls[0].Name)
		}
		if string(resp.ToolCalls[0].Arguments) != `{"location":"Beijing"}` {
			t.Errorf("ToolCalls[0].Arguments = %s, want %s", resp.ToolCalls[0].Arguments, `{"location":"Beijing"}`)
		}
	default:
		t.Error("No final response received")
	}
}

func TestStreamChatWithReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"id":"task-reasoning","model":"glm-4.7","choices":[{"index":0,"delta":{"reasoning_content":"Let me think..."}}]}`,
			`{"id":"task-reasoning","model":"glm-4.7","choices":[{"index":0,"delta":{"content":"The answer is 42."}}]}`,
		}

		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model:           "glm-4.7",
		ReasoningEffort: core.ReasoningEffortHigh,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Question?"},
		},
	})

	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	// Collect content
	var content strings.Builder
	for chunk := range stream.Ch {
		content.WriteString(chunk.Delta)
	}

	if content.String() != "The answer is 42." {
		t.Errorf("content = %q, want %q", content.String(), "The answer is 42.")
	}

	// Check final response
	select {
	case resp := <-stream.Final:
		if resp == nil {
			t.Fatal("Final response is nil")
		}
		if resp.Reasoning == nil {
			t.Fatal("Reasoning is nil")
		}
		if len(resp.Reasoning.Summary) != 1 {
			t.Fatalf("len(Reasoning.Summary) = %d, want 1", len(resp.Reasoning.Summary))
		}
		if resp.Reasoning.Summary[0] != "Let me think..." {
			t.Errorf("Reasoning.Summary[0] = %q, want %q", resp.Reasoning.Summary[0], "Let me think...")
		}
	default:
		t.Error("No final response received")
	}
}

func TestStreamChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"1002","message":"Invalid API key"}}`))
	}))
	defer server.Close()

	p := New("bad-key", WithBaseURL(server.URL))
	_, err := p.StreamChat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err == nil {
		t.Error("expected error for unauthorized request")
	}
}

func TestToolCallAssembler(t *testing.T) {
	assembler := newToolCallAssembler()

	// Add fragments
	assembler.addFragment(zaiStreamToolCall{
		Index: 0,
		ID:    "call_1",
		Type:  "function",
		Function: zaiStreamFunctionCall{
			Name:      "test_func",
			Arguments: `{"key":`,
		},
	})

	assembler.addFragment(zaiStreamToolCall{
		Index: 0,
		Function: zaiStreamFunctionCall{
			Arguments: `"value"}`,
		},
	})

	calls, err := assembler.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}

	if calls[0].ID != "call_1" {
		t.Errorf("ID = %q, want %q", calls[0].ID, "call_1")
	}

	if calls[0].Name != "test_func" {
		t.Errorf("Name = %q, want %q", calls[0].Name, "test_func")
	}

	if string(calls[0].Arguments) != `{"key":"value"}` {
		t.Errorf("Arguments = %s, want %s", calls[0].Arguments, `{"key":"value"}`)
	}
}

func TestToolCallAssemblerInvalidJSON(t *testing.T) {
	assembler := newToolCallAssembler()

	assembler.addFragment(zaiStreamToolCall{
		Index: 0,
		ID:    "call_bad",
		Function: zaiStreamFunctionCall{
			Name:      "broken",
			Arguments: `{invalid`,
		},
	})

	_, err := assembler.finalize()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestToolCallAssemblerEmpty(t *testing.T) {
	assembler := newToolCallAssembler()

	calls, err := assembler.finalize()
	if err != nil {
		t.Fatalf("finalize() error = %v", err)
	}

	if calls != nil {
		t.Errorf("calls = %v, want nil", calls)
	}
}
