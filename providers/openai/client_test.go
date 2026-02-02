package openai

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
		json.NewEncoder(w).Encode(openAIResponse{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIRespMsg{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
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

	if resp.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", resp.Model, "gpt-4o")
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
		json.NewEncoder(w).Encode(openAIResponse{
			ID:    "chatcmpl-456",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Message: openAIRespMsg{
						Role:    "assistant",
						Content: "",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_abc123",
								Type: "function",
								Function: openAIFunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco","unit":"celsius"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     15,
				CompletionTokens: 20,
				TotalTokens:      35,
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
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

func TestChatInvalidToolCallJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIResponse{
			ID:    "chatcmpl-789",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Message: openAIRespMsg{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_invalid",
								Type: "function",
								Function: openAIFunctionCall{
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
		Model: "gpt-4o",
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
		Model: "gpt-4o",
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
		Model: "gpt-4o",
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
		Model: "gpt-4o",
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
		// This won't be reached if context is cancelled
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Chat(ctx, &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestChatWithOrgAndProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("OpenAI-Organization") != "my-org" {
			t.Errorf("OpenAI-Organization = %q, want %q", r.Header.Get("OpenAI-Organization"), "my-org")
		}
		if r.Header.Get("OpenAI-Project") != "my-project" {
			t.Errorf("OpenAI-Project = %q, want %q", r.Header.Get("OpenAI-Project"), "my-project")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIResponse{
			ID:      "chatcmpl-test",
			Model:   "gpt-4o",
			Choices: []openAIChoice{{Message: openAIRespMsg{Content: "OK"}}},
		})
	}))
	defer server.Close()

	p := New("test-key",
		WithBaseURL(server.URL),
		WithOrgID("my-org"),
		WithProjectID("my-project"),
	)

	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Test"},
		},
	})

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestMapResponseEmptyChoices(t *testing.T) {
	resp := &openAIResponse{
		ID:      "chatcmpl-empty",
		Model:   "gpt-4o",
		Choices: []openAIChoice{},
		Usage:   openAIUsage{TotalTokens: 5},
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
	calls := []openAIToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: openAIFunctionCall{
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
	calls := []openAIToolCall{
		{
			ID:   "call_bad",
			Type: "function",
			Function: openAIFunctionCall{
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
