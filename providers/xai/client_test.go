package xai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Path = %q, want /chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header incorrect")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header incorrect")
		}

		w.Header().Set("x-request-id", "req-abc123")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(xaiResponse{
			ID:    "chatcmpl-123",
			Model: "grok-4",
			Choices: []xaiChoice{
				{
					Index: 0,
					Message: xaiRespMsg{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: xaiUsage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	})

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "chatcmpl-123")
	}

	if resp.Model != "grok-4" {
		t.Errorf("Model = %q, want %q", resp.Model, "grok-4")
	}

	if resp.Output != "Hello! How can I help you?" {
		t.Errorf("Output = %q, want %q", resp.Output, "Hello! How can I help you?")
	}

	if resp.Usage.PromptTokens != 10 {
		t.Errorf("Usage.PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}

	if resp.Usage.CompletionTokens != 8 {
		t.Errorf("Usage.CompletionTokens = %d, want 8", resp.Usage.CompletionTokens)
	}

	if resp.Usage.TotalTokens != 18 {
		t.Errorf("Usage.TotalTokens = %d, want 18", resp.Usage.TotalTokens)
	}
}

func TestChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(xaiResponse{
			ID:    "chatcmpl-456",
			Model: "grok-4",
			Choices: []xaiChoice{
				{
					Message: xaiRespMsg{
						Role:    "assistant",
						Content: "",
						ToolCalls: []xaiToolCall{
							{
								ID:   "call_abc123",
								Type: "function",
								Function: xaiFunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco","unit":"celsius"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: xaiUsage{
				PromptTokens:     15,
				CompletionTokens: 20,
				TotalTokens:      35,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather?"},
		},
	})

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", tc.ID, "call_abc123")
	}

	if tc.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", tc.Name, "get_weather")
	}

	expectedArgs := `{"location":"San Francisco","unit":"celsius"}`
	if string(tc.Arguments) != expectedArgs {
		t.Errorf("ToolCalls[0].Arguments = %s, want %s", tc.Arguments, expectedArgs)
	}
}

func TestChatWithReasoningContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(xaiResponse{
			ID:    "chatcmpl-reasoning",
			Model: "grok-3-mini",
			Choices: []xaiChoice{
				{
					Message: xaiRespMsg{
						Role:             "assistant",
						Content:          "The answer is 42.",
						ReasoningContent: "Let me think about this step by step...",
					},
					FinishReason: "stop",
				},
			},
			Usage: xaiUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
				ReasoningTokens:  15,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model:           "grok-3-mini",
		ReasoningEffort: core.ReasoningEffortHigh,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What is the meaning of life?"},
		},
	})

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output != "The answer is 42." {
		t.Errorf("Output = %q, want %q", resp.Output, "The answer is 42.")
	}

	if resp.Reasoning == nil {
		t.Fatal("Reasoning is nil")
	}

	if len(resp.Reasoning.Summary) != 1 {
		t.Fatalf("len(Reasoning.Summary) = %d, want 1", len(resp.Reasoning.Summary))
	}

	if resp.Reasoning.Summary[0] != "Let me think about this step by step..." {
		t.Errorf("Reasoning.Summary[0] = %q, want %q", resp.Reasoning.Summary[0], "Let me think about this step by step...")
	}
}

func TestChatInvalidToolCallJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(xaiResponse{
			ID:    "chatcmpl-789",
			Model: "grok-4",
			Choices: []xaiChoice{
				{
					Message: xaiRespMsg{
						Role: "assistant",
						ToolCalls: []xaiToolCall{
							{
								ID:   "call_invalid",
								Type: "function",
								Function: xaiFunctionCall{
									Name:      "broken",
									Arguments: `{not valid json`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if !errors.Is(err, ErrToolArgsInvalidJSON) {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", err)
	}
}

func TestChatError400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req-err-400")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Invalid model","type":"invalid_request_error"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "invalid-model",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}

	var pErr *core.ProviderError
	if errors.As(err, &pErr) {
		if pErr.RequestID != "req-err-400" {
			t.Errorf("RequestID = %q, want %q", pErr.RequestID, "req-err-400")
		}
	}
}

func TestChatError401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer server.Close()

	p := New("bad-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestChatError429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if !errors.Is(err, core.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestChatError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"Internal error"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if !errors.Is(err, core.ErrServer) {
		t.Errorf("expected ErrServer, got %v", err)
	}
}

func TestChatContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Chat(ctx, &core.ChatRequest{
		Model: "grok-4",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestMapResponseEmptyChoices(t *testing.T) {
	resp := &xaiResponse{
		ID:      "chatcmpl-empty",
		Model:   "grok-4",
		Choices: []xaiChoice{},
		Usage:   xaiUsage{TotalTokens: 5},
	}

	result, err := mapResponse(resp)
	if err != nil {
		t.Fatalf("mapResponse() error = %v", err)
	}

	if result.Output != "" {
		t.Errorf("Output = %q, want empty", result.Output)
	}
}

func TestMapToolCallsValidJSON(t *testing.T) {
	calls := []xaiToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: xaiFunctionCall{
				Name:      "func1",
				Arguments: `{"key": "value"}`,
			},
		},
	}

	result, err := mapToolCalls(calls)
	if err != nil {
		t.Fatalf("mapToolCalls() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].ID != "call_1" {
		t.Errorf("ID = %q, want %q", result[0].ID, "call_1")
	}

	if result[0].Name != "func1" {
		t.Errorf("Name = %q, want %q", result[0].Name, "func1")
	}
}

func TestMapToolCallsInvalidJSON(t *testing.T) {
	calls := []xaiToolCall{
		{
			ID:   "call_bad",
			Type: "function",
			Function: xaiFunctionCall{
				Name:      "broken",
				Arguments: `{invalid`,
			},
		},
	}

	_, err := mapToolCalls(calls)
	if !errors.Is(err, ErrToolArgsInvalidJSON) {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", err)
	}
}
