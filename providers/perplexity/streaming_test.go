package perplexity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestToolCallAssembler(t *testing.T) {
	t.Run("single tool call", func(t *testing.T) {
		asm := newToolCallAssembler()

		// Simulate streaming fragments
		asm.addFragment(perplexityStreamToolCall{
			Index: 0,
			ID:    "call_1",
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Name: "get_weather",
			},
		})
		asm.addFragment(perplexityStreamToolCall{
			Index: 0,
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Arguments: `{"city":`,
			},
		})
		asm.addFragment(perplexityStreamToolCall{
			Index: 0,
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Arguments: `"Tokyo"}`,
			},
		})

		calls, err := asm.finalize()
		if err != nil {
			t.Fatalf("finalize() error = %v", err)
		}

		if len(calls) != 1 {
			t.Fatalf("calls count = %d, want 1", len(calls))
		}

		if calls[0].ID != "call_1" {
			t.Errorf("ID = %q, want %q", calls[0].ID, "call_1")
		}
		if calls[0].Name != "get_weather" {
			t.Errorf("Name = %q, want %q", calls[0].Name, "get_weather")
		}

		var args map[string]string
		if err := json.Unmarshal(calls[0].Arguments, &args); err != nil {
			t.Fatalf("Failed to unmarshal arguments: %v", err)
		}
		if args["city"] != "Tokyo" {
			t.Errorf("args[city] = %q, want %q", args["city"], "Tokyo")
		}
	})

	t.Run("multiple tool calls", func(t *testing.T) {
		asm := newToolCallAssembler()

		// Tool call 0
		asm.addFragment(perplexityStreamToolCall{
			Index: 0,
			ID:    "call_1",
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Name:      "get_weather",
				Arguments: `{"city":"NYC"}`,
			},
		})

		// Tool call 1
		asm.addFragment(perplexityStreamToolCall{
			Index: 1,
			ID:    "call_2",
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Name:      "search",
				Arguments: `{"query":"news"}`,
			},
		})

		calls, err := asm.finalize()
		if err != nil {
			t.Fatalf("finalize() error = %v", err)
		}

		if len(calls) != 2 {
			t.Fatalf("calls count = %d, want 2", len(calls))
		}

		if calls[0].Name != "get_weather" {
			t.Errorf("calls[0].Name = %q, want %q", calls[0].Name, "get_weather")
		}
		if calls[1].Name != "search" {
			t.Errorf("calls[1].Name = %q, want %q", calls[1].Name, "search")
		}
	})

	t.Run("empty assembler", func(t *testing.T) {
		asm := newToolCallAssembler()
		calls, err := asm.finalize()

		if err != nil {
			t.Fatalf("finalize() error = %v", err)
		}
		if calls != nil {
			t.Errorf("calls should be nil, got %v", calls)
		}
	})

	t.Run("invalid JSON arguments", func(t *testing.T) {
		asm := newToolCallAssembler()

		asm.addFragment(perplexityStreamToolCall{
			Index: 0,
			ID:    "call_1",
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Name:      "test",
				Arguments: `{invalid`,
			},
		})

		_, err := asm.finalize()
		if err != ErrToolArgsInvalidJSON {
			t.Errorf("finalize() error = %v, want ErrToolArgsInvalidJSON", err)
		}
	})

	t.Run("sparse indices", func(t *testing.T) {
		asm := newToolCallAssembler()

		// Only index 0 and 2, no 1
		asm.addFragment(perplexityStreamToolCall{
			Index: 0,
			ID:    "call_0",
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Name:      "func0",
				Arguments: `{}`,
			},
		})
		asm.addFragment(perplexityStreamToolCall{
			Index: 2,
			ID:    "call_2",
			Function: struct {
				Name      string `json:"name,omitempty"`
				Arguments string `json:"arguments,omitempty"`
			}{
				Name:      "func2",
				Arguments: `{}`,
			},
		})

		calls, err := asm.finalize()
		if err != nil {
			t.Fatalf("finalize() error = %v", err)
		}

		if len(calls) != 2 {
			t.Fatalf("calls count = %d, want 2", len(calls))
		}
	})
}

func TestDoStreamChat(t *testing.T) {
	t.Run("successful stream", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("x-request-id", "stream-req-123")

			// Send SSE events
			chunks := []string{
				`data: {"id":"resp-1","model":"sonar","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
				`data: {"id":"resp-1","model":"sonar","choices":[{"index":0,"delta":{"content":" world!"}}]}`,
				`data: {"id":"resp-1","model":"sonar","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
				`data: [DONE]`,
			}

			for _, chunk := range chunks {
				fmt.Fprintln(w, chunk)
				fmt.Fprintln(w, "")
			}
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		stream, err := p.doStreamChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err != nil {
			t.Fatalf("doStreamChat() error = %v", err)
		}

		// Collect chunks
		var content string
		for chunk := range stream.Ch {
			content += chunk.Delta
		}

		if content != "Hello world!" {
			t.Errorf("content = %q, want %q", content, "Hello world!")
		}

		// Check final response
		finalResp := <-stream.Final
		if finalResp == nil {
			t.Fatal("Final response should not be nil")
		}
		if finalResp.ID != "resp-1" {
			t.Errorf("ID = %q, want %q", finalResp.ID, "resp-1")
		}
		if finalResp.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", finalResp.Usage.TotalTokens)
		}

		// Check no errors
		select {
		case err := <-stream.Err:
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		default:
		}
	})

	t.Run("stream with tool calls", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")

			chunks := []string{
				`data: {"id":"resp-2","model":"sonar","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather"}}]}}]}`,
				`data: {"id":"resp-2","model":"sonar","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}`,
				`data: {"id":"resp-2","model":"sonar","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Tokyo\"}"}}]}}]}`,
				`data: {"id":"resp-2","model":"sonar","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
				`data: [DONE]`,
			}

			for _, chunk := range chunks {
				fmt.Fprintln(w, chunk)
				fmt.Fprintln(w, "")
			}
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		stream, err := p.doStreamChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Weather?"}},
		})

		if err != nil {
			t.Fatalf("doStreamChat() error = %v", err)
		}

		// Drain content channel
		for range stream.Ch {
		}

		// Check final response has tool calls
		finalResp := <-stream.Final
		if finalResp == nil {
			t.Fatal("Final response should not be nil")
		}
		if len(finalResp.ToolCalls) != 1 {
			t.Fatalf("ToolCalls count = %d, want 1", len(finalResp.ToolCalls))
		}
		if finalResp.ToolCalls[0].Name != "get_weather" {
			t.Errorf("ToolCall.Name = %q, want %q", finalResp.ToolCalls[0].Name, "get_weather")
		}
	})

	t.Run("error response before stream", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-request-id", "err-req")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "Invalid key",
					"type":    "auth_error",
				},
			})
		}))
		defer server.Close()

		p := New("bad-key", WithBaseURL(server.URL))
		_, err := p.doStreamChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err == nil {
			t.Fatal("doStreamChat() should return error")
		}

		var provErr *core.ProviderError
		if !errors.As(err, &provErr) {
			t.Fatal("err should be *core.ProviderError")
		}
		if provErr.Status != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", provErr.Status, http.StatusUnauthorized)
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New("test-key", WithBaseURL("http://localhost:0"))
		_, err := p.doStreamChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if !errors.Is(err, core.ErrNetwork) {
			t.Error("err should wrap core.ErrNetwork")
		}
	})

	t.Run("skip empty lines and comments", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")

			lines := []string{
				"",            // empty line
				":keep-alive", // comment
				`data: {"id":"resp-3","model":"sonar","choices":[{"index":0,"delta":{"content":"Hi"}}]}`, // actual data
				"",
				`:another comment`,
				`data: [DONE]`,
			}

			for _, line := range lines {
				fmt.Fprintln(w, line)
			}
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		stream, err := p.doStreamChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err != nil {
			t.Fatalf("doStreamChat() error = %v", err)
		}

		var content string
		for chunk := range stream.Ch {
			content += chunk.Delta
		}

		if content != "Hi" {
			t.Errorf("content = %q, want %q", content, "Hi")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			// Block until context is cancelled
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		p := New("test-key", WithBaseURL(server.URL))
		_, err := p.doStreamChat(ctx, &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err == nil {
			t.Fatal("doStreamChat() should return error on cancelled context")
		}
	})
}

func TestNewToolCallAssembler(t *testing.T) {
	asm := newToolCallAssembler()
	if asm == nil {
		t.Fatal("newToolCallAssembler() returned nil")
	}
	if asm.asm == nil {
		t.Fatal("newToolCallAssembler().asm should not be nil")
	}
}
