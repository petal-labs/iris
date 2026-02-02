package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestDoChat(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		expectedPath := "/v1beta/models/gemini-2.5-flash:generateContent"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}

		// Verify headers
		if r.Header.Get("x-goog-api-key") != "test-api-key" {
			t.Errorf("x-goog-api-key = %q, want 'test-api-key'", r.Header.Get("x-goog-api-key"))
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want 'application/json'", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var reqBody geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(reqBody.Contents) != 1 {
			t.Errorf("contents count = %d, want 1", len(reqBody.Contents))
		}

		if reqBody.Contents[0].Parts[0].Text != "Hello" {
			t.Errorf("message text = %q, want 'Hello'", reqBody.Contents[0].Parts[0].Text)
		}

		// Send response
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Role:  "model",
						Parts: []geminiPart{{Text: "Hello! How can I help you?"}},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsage{
				PromptTokenCount:     5,
				CandidatesTokenCount: 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with mock server
	provider := New("test-api-key", WithBaseURL(server.URL))

	// Create chat request
	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	// Execute request
	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	// Verify response
	if resp.Output != "Hello! How can I help you?" {
		t.Errorf("Output = %q, want 'Hello! How can I help you?'", resp.Output)
	}

	if resp.Model != "gemini-2.5-flash" {
		t.Errorf("Model = %q, want 'gemini-2.5-flash'", resp.Model)
	}

	if resp.Usage.PromptTokens != 5 {
		t.Errorf("PromptTokens = %d, want 5", resp.Usage.PromptTokens)
	}

	if resp.Usage.CompletionTokens != 10 {
		t.Errorf("CompletionTokens = %d, want 10", resp.Usage.CompletionTokens)
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestDoChatWithSystemMessage(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body
		var reqBody geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Verify system instruction is set
		if reqBody.SystemInstruction == nil {
			t.Fatal("SystemInstruction is nil")
		}

		if len(reqBody.SystemInstruction.Parts) != 1 {
			t.Fatalf("SystemInstruction.Parts count = %d, want 1", len(reqBody.SystemInstruction.Parts))
		}

		if reqBody.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
			t.Errorf("SystemInstruction text = %q, want 'You are a helpful assistant.'", reqBody.SystemInstruction.Parts[0].Text)
		}

		// Verify user message is in contents (not system)
		if len(reqBody.Contents) != 1 {
			t.Fatalf("Contents count = %d, want 1", len(reqBody.Contents))
		}

		if reqBody.Contents[0].Role != "user" {
			t.Errorf("Contents[0].Role = %q, want 'user'", reqBody.Contents[0].Role)
		}

		// Send response
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Role:  "model",
						Parts: []geminiPart{{Text: "I understand. How can I assist you?"}},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsage{
				PromptTokenCount:     15,
				CandidatesTokenCount: 8,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with mock server
	provider := New("test-api-key", WithBaseURL(server.URL))

	// Create chat request with system message
	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleSystem, Content: "You are a helpful assistant."},
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	// Execute request
	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	// Verify response
	if resp.Output != "I understand. How can I assist you?" {
		t.Errorf("Output = %q, want 'I understand. How can I assist you?'", resp.Output)
	}
}

func TestDoChatWithToolCall(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with tool call
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Role: "model",
						Parts: []geminiPart{
							{
								FunctionCall: &geminiFunctionCall{
									Name: "get_weather",
									Args: json.RawMessage(`{"location":"San Francisco","unit":"celsius"}`),
								},
							},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsage{
				PromptTokenCount:     20,
				CandidatesTokenCount: 15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with mock server
	provider := New("test-api-key", WithBaseURL(server.URL))

	// Create chat request
	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather in San Francisco?"},
		},
	}

	// Execute request
	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	// Verify tool calls
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "call_0" {
		t.Errorf("ToolCall ID = %q, want 'call_0'", tc.ID)
	}

	if tc.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want 'get_weather'", tc.Name)
	}

	expectedArgs := `{"location":"San Francisco","unit":"celsius"}`
	if string(tc.Arguments) != expectedArgs {
		t.Errorf("ToolCall Arguments = %q, want %q", string(tc.Arguments), expectedArgs)
	}
}

func TestDoChatError(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		body         string
		wantSentinel error
	}{
		{
			name:         "bad request",
			statusCode:   400,
			body:         `{"error":{"code":400,"message":"Invalid model name","status":"INVALID_ARGUMENT"}}`,
			wantSentinel: core.ErrBadRequest,
		},
		{
			name:         "unauthorized",
			statusCode:   401,
			body:         `{"error":{"code":401,"message":"API key not valid","status":"UNAUTHENTICATED"}}`,
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "rate limited",
			statusCode:   429,
			body:         `{"error":{"code":429,"message":"Resource exhausted","status":"RESOURCE_EXHAUSTED"}}`,
			wantSentinel: core.ErrRateLimited,
		},
		{
			name:         "server error",
			statusCode:   500,
			body:         `{"error":{"code":500,"message":"Internal error","status":"INTERNAL"}}`,
			wantSentinel: core.ErrServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns error
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			// Create provider with mock server
			provider := New("test-api-key", WithBaseURL(server.URL))

			// Create chat request
			req := &core.ChatRequest{
				Model: "gemini-2.5-flash",
				Messages: []core.Message{
					{Role: core.RoleUser, Content: "Hello"},
				},
			}

			// Execute request
			_, err := provider.Chat(context.Background(), req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// Verify error type
			var provErr *core.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error is not ProviderError: %v", err)
			}

			if !errors.Is(provErr.Err, tt.wantSentinel) {
				t.Errorf("sentinel = %v, want %v", provErr.Err, tt.wantSentinel)
			}

			if provErr.Provider != "gemini" {
				t.Errorf("Provider = %q, want 'gemini'", provErr.Provider)
			}

			if provErr.Status != tt.statusCode {
				t.Errorf("Status = %d, want %d", provErr.Status, tt.statusCode)
			}
		})
	}
}

func TestDoChatWithThinkingResponse(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with thinking parts
		thoughtTrue := true
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Role: "model",
						Parts: []geminiPart{
							{Text: "Let me think about this...", Thought: &thoughtTrue},
							{Text: "I'll consider the options.", Thought: &thoughtTrue},
							{Text: "The answer is 42."},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsage{
				PromptTokenCount:     10,
				CandidatesTokenCount: 25,
				ThoughtsTokenCount:   15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with mock server
	provider := New("test-api-key", WithBaseURL(server.URL))

	// Create chat request
	req := &core.ChatRequest{
		Model:           "gemini-2.5-flash",
		ReasoningEffort: core.ReasoningEffortMedium,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What is the meaning of life?"},
		},
	}

	// Execute request
	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	// Verify output excludes thoughts
	if resp.Output != "The answer is 42." {
		t.Errorf("Output = %q, want 'The answer is 42.'", resp.Output)
	}

	// Verify reasoning is populated
	if resp.Reasoning == nil {
		t.Fatal("Reasoning is nil")
	}

	if len(resp.Reasoning.Summary) != 2 {
		t.Fatalf("Reasoning.Summary count = %d, want 2", len(resp.Reasoning.Summary))
	}

	if resp.Reasoning.Summary[0] != "Let me think about this..." {
		t.Errorf("Reasoning.Summary[0] = %q, want 'Let me think about this...'", resp.Reasoning.Summary[0])
	}

	if resp.Reasoning.Summary[1] != "I'll consider the options." {
		t.Errorf("Reasoning.Summary[1] = %q, want 'I'll consider the options.'", resp.Reasoning.Summary[1])
	}
}

func TestDoChatContextCanceled(t *testing.T) {
	// Create mock server that hangs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait for context to be canceled
		<-r.Context().Done()
	}))
	defer server.Close()

	// Create provider with mock server
	provider := New("test-api-key", WithBaseURL(server.URL))

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create chat request
	req := &core.ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	// Execute request
	_, err := provider.Chat(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error is a network error wrapping context canceled
	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("error is not ProviderError: %v", err)
	}

	if !errors.Is(provErr.Err, core.ErrNetwork) {
		t.Errorf("sentinel = %v, want ErrNetwork", provErr.Err)
	}
}
