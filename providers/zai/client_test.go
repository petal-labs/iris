package zai

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
		if r.Header.Get("Accept-Language") != "en-US,en" {
			t.Errorf("Accept-Language header incorrect")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(zaiResponse{
			ID:        "task-123",
			RequestID: "req-abc123",
			Model:     "glm-4.7",
			Choices: []zaiChoice{
				{
					Index: 0,
					Message: zaiRespMsg{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: zaiUsage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	})

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.ID != "task-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "task-123")
	}

	if resp.Model != "glm-4.7" {
		t.Errorf("Model = %q, want %q", resp.Model, "glm-4.7")
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
		json.NewEncoder(w).Encode(zaiResponse{
			ID:    "task-456",
			Model: "glm-4.7",
			Choices: []zaiChoice{
				{
					Message: zaiRespMsg{
						Role:    "assistant",
						Content: "",
						ToolCalls: []zaiToolCall{
							{
								ID:   "call_abc123",
								Type: "function",
								Function: zaiFunctionCall{
									Name:      "get_weather",
									Arguments: json.RawMessage(`{"location":"San Francisco","unit":"celsius"}`),
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: zaiUsage{
				PromptTokens:     15,
				CompletionTokens: 20,
				TotalTokens:      35,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
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
		json.NewEncoder(w).Encode(zaiResponse{
			ID:    "task-reasoning",
			Model: "glm-4.7",
			Choices: []zaiChoice{
				{
					Message: zaiRespMsg{
						Role:             "assistant",
						Content:          "The answer is 42.",
						ReasoningContent: "Let me think about this step by step...",
					},
					FinishReason: "stop",
				},
			},
			Usage: zaiUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model:           "glm-4.7",
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
	// Z.ai returns arguments as JSON objects, so we test the mapToolCalls function
	// directly with invalid JSON since the HTTP response wouldn't contain invalid JSON
	calls := []zaiToolCall{
		{
			ID:   "call_invalid",
			Type: "function",
			Function: zaiFunctionCall{
				Name:      "broken",
				Arguments: json.RawMessage(`{not valid json`),
			},
		},
	}

	_, err := mapToolCalls(calls)
	if !errors.Is(err, ErrToolArgsInvalidJSON) {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", err)
	}
}

func TestChatError400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"1214","message":"Invalid model"}}`))
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
		if pErr.Code != "1214" {
			t.Errorf("Code = %q, want %q", pErr.Code, "1214")
		}
	}
}

func TestChatError401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"1002","message":"Invalid API key"}}`))
	}))
	defer server.Close()

	p := New("bad-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
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
		w.Write([]byte(`{"error":{"code":"1302","message":"Rate limit exceeded"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
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
		w.Write([]byte(`{"error":{"code":"500","message":"Internal error"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "glm-4.7",
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
		Model: "glm-4.7",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestMapResponseEmptyChoices(t *testing.T) {
	resp := &zaiResponse{
		ID:      "task-empty",
		Model:   "glm-4.7",
		Choices: []zaiChoice{},
		Usage:   zaiUsage{TotalTokens: 5},
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
	calls := []zaiToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: zaiFunctionCall{
				Name:      "func1",
				Arguments: json.RawMessage(`{"key": "value"}`),
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
	calls := []zaiToolCall{
		{
			ID:   "call_bad",
			Type: "function",
			Function: zaiFunctionCall{
				Name:      "broken",
				Arguments: json.RawMessage(`{invalid`),
			},
		},
	}

	_, err := mapToolCalls(calls)
	if !errors.Is(err, ErrToolArgsInvalidJSON) {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", err)
	}
}
