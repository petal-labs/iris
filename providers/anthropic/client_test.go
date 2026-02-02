package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestDoChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Path = %s, want /v1/messages", r.URL.Path)
		}

		// Verify headers
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

		if req.Model != "claude-sonnet-4-5" {
			t.Errorf("Model = %s, want claude-sonnet-4-5", req.Model)
		}
		if req.MaxTokens != defaultMaxTokens {
			t.Errorf("MaxTokens = %d, want %d", req.MaxTokens, defaultMaxTokens)
		}
		if len(req.Messages) != 1 {
			t.Errorf("Messages count = %d, want 1", len(req.Messages))
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_123")
		w.WriteHeader(http.StatusOK)

		resp := anthropicResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-5",
			Content: []anthropicResponseContent{
				{Type: "text", Text: "Hello! How can I help you?"},
			},
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  10,
				OutputTokens: 8,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	resp, err := p.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.ID != "msg_123" {
		t.Errorf("ID = %q, want 'msg_123'", resp.ID)
	}

	if resp.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want 'claude-sonnet-4-5'", resp.Model)
	}

	if resp.Output != "Hello! How can I help you?" {
		t.Errorf("Output = %q, want 'Hello! How can I help you?'", resp.Output)
	}

	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}

	if resp.Usage.CompletionTokens != 8 {
		t.Errorf("CompletionTokens = %d, want 8", resp.Usage.CompletionTokens)
	}

	if resp.Usage.TotalTokens != 18 {
		t.Errorf("TotalTokens = %d, want 18", resp.Usage.TotalTokens)
	}
}

func TestDoChatWithSystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var req anthropicRequest
		json.Unmarshal(body, &req)

		if req.System != "You are helpful" {
			t.Errorf("System = %q, want 'You are helpful'", req.System)
		}

		if len(req.Messages) != 1 {
			t.Errorf("Messages count = %d, want 1 (system extracted)", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(anthropicResponse{
			ID:         "msg_sys",
			Model:      "claude-sonnet-4-5",
			Content:    []anthropicResponseContent{{Type: "text", Text: "Hi!"}},
			StopReason: "end_turn",
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleSystem, Content: "You are helpful"},
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	resp, err := p.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output != "Hi!" {
		t.Errorf("Output = %q, want 'Hi!'", resp.Output)
	}
}

func TestDoChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_tool")
		w.WriteHeader(http.StatusOK)

		resp := anthropicResponse{
			ID:    "msg_tool",
			Model: "claude-sonnet-4-5",
			Content: []anthropicResponseContent{
				{Type: "text", Text: "Let me check the weather."},
				{
					Type:  "tool_use",
					ID:    "tool_xyz",
					Name:  "get_weather",
					Input: json.RawMessage(`{"location":"NYC","unit":"fahrenheit"}`),
				},
			},
			StopReason: "tool_use",
			Usage: anthropicUsage{
				InputTokens:  15,
				OutputTokens: 20,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather in NYC?"},
		},
	}

	resp, err := p.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output != "Let me check the weather." {
		t.Errorf("Output = %q, want 'Let me check the weather.'", resp.Output)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "tool_xyz" {
		t.Errorf("ToolCall ID = %q, want 'tool_xyz'", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want 'get_weather'", tc.Name)
	}

	expectedArgs := `{"location":"NYC","unit":"fahrenheit"}`
	if string(tc.Arguments) != expectedArgs {
		t.Errorf("ToolCall Arguments = %s, want %s", tc.Arguments, expectedArgs)
	}
}

func TestDoChatUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_auth")
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

	_, err := p.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("Chat() should return error")
	}

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("error = %v, want ErrUnauthorized", err)
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("error should be ProviderError")
	}

	if provErr.Status != 401 {
		t.Errorf("Status = %d, want 401", provErr.Status)
	}

	if provErr.RequestID != "req_auth" {
		t.Errorf("RequestID = %q, want 'req_auth'", provErr.RequestID)
	}

	if provErr.Message != "Invalid API key" {
		t.Errorf("Message = %q, want 'Invalid API key'", provErr.Message)
	}
}

func TestDoChatRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Too many requests"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	_, err := p.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("Chat() should return error")
	}

	if !errors.Is(err, core.ErrRateLimited) {
		t.Errorf("error = %v, want ErrRateLimited", err)
	}
}

func TestDoChatServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"Internal server error"}}`))
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	_, err := p.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("Chat() should return error")
	}

	if !errors.Is(err, core.ErrServer) {
		t.Errorf("error = %v, want ErrServer", err)
	}
}

func TestDoChatWithTemperature(t *testing.T) {
	temp := float32(0.5)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var req anthropicRequest
		json.Unmarshal(body, &req)

		if req.Temperature == nil || *req.Temperature != 0.5 {
			t.Errorf("Temperature = %v, want 0.5", req.Temperature)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(anthropicResponse{
			ID:      "msg_temp",
			Model:   "claude-sonnet-4-5",
			Content: []anthropicResponseContent{{Type: "text", Text: "Response"}},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model:       "claude-sonnet-4-5",
		Messages:    []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		Temperature: &temp,
	}

	_, err := p.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestDoChatWithMaxTokens(t *testing.T) {
	maxTokens := 500

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var req anthropicRequest
		json.Unmarshal(body, &req)

		if req.MaxTokens != 500 {
			t.Errorf("MaxTokens = %d, want 500", req.MaxTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(anthropicResponse{
			ID:      "msg_max",
			Model:   "claude-sonnet-4-5",
			Content: []anthropicResponseContent{{Type: "text", Text: "Response"}},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	req := &core.ChatRequest{
		Model:     "claude-sonnet-4-5",
		Messages:  []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		MaxTokens: &maxTokens,
	}

	_, err := p.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}
