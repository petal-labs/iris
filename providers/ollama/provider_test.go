package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

// TestNew tests the provider constructor.
func TestNew(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		p := New()
		if p.config.BaseURL != DefaultLocalURL {
			t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultLocalURL)
		}
		if p.config.APIKey != "" {
			t.Errorf("APIKey = %q, want empty", p.config.APIKey)
		}
		if p.config.HTTPClient != http.DefaultClient {
			t.Error("HTTPClient should be http.DefaultClient")
		}
	})

	t.Run("with options", func(t *testing.T) {
		client := &http.Client{Timeout: 30 * time.Second}
		headers := http.Header{"X-Custom": []string{"value"}}

		p := New(
			WithAPIKey("test-key"),
			WithBaseURL("http://custom:11434"),
			WithHTTPClient(client),
			WithHeaders(headers),
			WithTimeout(60*time.Second),
		)

		if p.config.APIKey != "test-key" {
			t.Errorf("APIKey = %q, want %q", p.config.APIKey, "test-key")
		}
		if p.config.BaseURL != "http://custom:11434" {
			t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, "http://custom:11434")
		}
		if p.config.HTTPClient != client {
			t.Error("HTTPClient not set correctly")
		}
		if p.config.Headers.Get("X-Custom") != "value" {
			t.Errorf("Headers[X-Custom] = %q, want %q", p.config.Headers.Get("X-Custom"), "value")
		}
		if p.config.Timeout != 60*time.Second {
			t.Errorf("Timeout = %v, want %v", p.config.Timeout, 60*time.Second)
		}
	})

	t.Run("with cloud", func(t *testing.T) {
		p := New(WithCloud(), WithAPIKey("cloud-key"))
		if p.config.BaseURL != DefaultCloudURL {
			t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultCloudURL)
		}
		if p.config.APIKey != "cloud-key" {
			t.Errorf("APIKey = %q, want %q", p.config.APIKey, "cloud-key")
		}
	})
}

// TestProviderID tests the ID method.
func TestProviderID(t *testing.T) {
	p := New()
	if id := p.ID(); id != "ollama" {
		t.Errorf("ID() = %q, want %q", id, "ollama")
	}
}

// TestProviderModels tests the Models method.
func TestProviderModels(t *testing.T) {
	p := New()
	models := p.Models()

	if len(models) == 0 {
		t.Error("Models() should return example models")
	}

	// Check that llama3.2 is in the list
	found := false
	for _, m := range models {
		if m.ID == "llama3.2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Models() should include llama3.2")
	}
}

// TestProviderSupports tests the Supports method.
func TestProviderSupports(t *testing.T) {
	p := New()

	tests := []struct {
		feature core.Feature
		want    bool
	}{
		{core.FeatureChat, true},
		{core.FeatureChatStreaming, true},
		{core.FeatureToolCalling, true},
		{core.FeatureReasoning, true},
		{core.Feature("unknown"), false},
	}

	for _, tt := range tests {
		if got := p.Supports(tt.feature); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.feature, got, tt.want)
		}
	}
}

// TestBuildHeaders tests header construction.
func TestBuildHeaders(t *testing.T) {
	t.Run("without API key", func(t *testing.T) {
		p := New()
		headers := p.buildHeaders()

		if headers.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", headers.Get("Content-Type"), "application/json")
		}
		if headers.Get("Authorization") != "" {
			t.Error("Authorization header should not be set without API key")
		}
	})

	t.Run("with API key", func(t *testing.T) {
		p := New(WithAPIKey("test-key"))
		headers := p.buildHeaders()

		if headers.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", headers.Get("Authorization"), "Bearer test-key")
		}
	})

	t.Run("with custom headers", func(t *testing.T) {
		customHeaders := http.Header{"X-Custom": []string{"value"}}
		p := New(WithHeaders(customHeaders))
		headers := p.buildHeaders()

		if headers.Get("X-Custom") != "value" {
			t.Errorf("X-Custom = %q, want %q", headers.Get("X-Custom"), "value")
		}
	})
}

// TestChat tests non-streaming chat.
func TestChat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/api/chat" {
				t.Errorf("Path = %q, want /api/chat", r.URL.Path)
			}

			var req ollamaRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if req.Model != "llama3.2" {
				t.Errorf("Model = %q, want llama3.2", req.Model)
			}
			if req.Stream {
				t.Error("Stream should be false")
			}

			// Send response
			resp := ollamaResponse{
				Model:     "llama3.2",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you?",
				},
				Done:            true,
				DoneReason:      "stop",
				PromptEvalCount: 10,
				EvalCount:       20,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		p := New(WithBaseURL(server.URL))
		resp, err := p.Chat(context.Background(), &core.ChatRequest{
			Model: "llama3.2",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Chat() error = %v", err)
		}
		if resp.Output != "Hello! How can I help you?" {
			t.Errorf("Output = %q, want %q", resp.Output, "Hello! How can I help you?")
		}
		if resp.Model != "llama3.2" {
			t.Errorf("Model = %q, want llama3.2", resp.Model)
		}
		if resp.Usage.PromptTokens != 10 {
			t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
		}
		if resp.Usage.CompletionTokens != 20 {
			t.Errorf("CompletionTokens = %d, want 20", resp.Usage.CompletionTokens)
		}
	})

	t.Run("with tools", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if len(req.Tools) != 1 {
				t.Errorf("Tools count = %d, want 1", len(req.Tools))
			}

			// Send tool call response
			resp := ollamaResponse{
				Model:     "llama3.2",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []ollamaToolCall{
						{
							Function: ollamaFunctionCall{
								Name: "get_weather",
								Arguments: map[string]interface{}{
									"city": "Tokyo",
								},
							},
						},
					},
				},
				Done:       true,
				DoneReason: "tool_calls",
			}

			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		p := New(WithBaseURL(server.URL))
		resp, err := p.Chat(context.Background(), &core.ChatRequest{
			Model: "llama3.2",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "What's the weather in Tokyo?"},
			},
			Tools: []core.Tool{&mockTool{name: "get_weather", description: "Get weather"}},
		})

		if err != nil {
			t.Fatalf("Chat() error = %v", err)
		}
		if len(resp.ToolCalls) != 1 {
			t.Fatalf("ToolCalls count = %d, want 1", len(resp.ToolCalls))
		}
		if resp.ToolCalls[0].Name != "get_weather" {
			t.Errorf("ToolCall.Name = %q, want get_weather", resp.ToolCalls[0].Name)
		}
	})

	t.Run("with thinking", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if req.Think == nil || !*req.Think {
				t.Error("Think should be true")
			}

			resp := ollamaResponse{
				Model:     "qwen3",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: ollamaMessage{
					Role:     "assistant",
					Content:  "The answer is 36.",
					Thinking: "Let me calculate: 15% of 240 = 0.15 * 240 = 36",
				},
				Done: true,
			}

			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		p := New(WithBaseURL(server.URL))
		resp, err := p.Chat(context.Background(), &core.ChatRequest{
			Model: "qwen3",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "What is 15% of 240?"},
			},
			ReasoningEffort: core.ReasoningEffortHigh,
		})

		if err != nil {
			t.Fatalf("Chat() error = %v", err)
		}
		if resp.Reasoning == nil {
			t.Fatal("Reasoning should not be nil")
		}
		if len(resp.Reasoning.Summary) != 1 {
			t.Fatalf("Reasoning.Summary length = %d, want 1", len(resp.Reasoning.Summary))
		}
		if !strings.Contains(resp.Reasoning.Summary[0], "calculate") {
			t.Errorf("Reasoning.Summary[0] = %q, should contain 'calculate'", resp.Reasoning.Summary[0])
		}
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ollamaErrorResponse{
				Error: "model 'nonexistent' not found",
			})
		}))
		defer server.Close()

		p := New(WithBaseURL(server.URL))
		_, err := p.Chat(context.Background(), &core.ChatRequest{
			Model: "nonexistent",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		})

		if err == nil {
			t.Fatal("Chat() should return error")
		}

		provErr, ok := err.(*core.ProviderError)
		if !ok {
			t.Fatalf("Error should be *core.ProviderError, got %T", err)
		}
		if provErr.Code != "model_not_found" {
			t.Errorf("Error code = %q, want model_not_found", provErr.Code)
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New(WithBaseURL("http://localhost:99999"))
		_, err := p.Chat(context.Background(), &core.ChatRequest{
			Model: "llama3.2",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		})

		if err == nil {
			t.Fatal("Chat() should return error")
		}

		provErr, ok := err.(*core.ProviderError)
		if !ok {
			t.Fatalf("Error should be *core.ProviderError, got %T", err)
		}
		if provErr.Code != "network_error" {
			t.Errorf("Error code = %q, want network_error", provErr.Code)
		}
	})
}

// TestStreamChat tests streaming chat.
func TestStreamChat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if !req.Stream {
				t.Error("Stream should be true")
			}

			w.Header().Set("Content-Type", "application/x-ndjson")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("ResponseWriter doesn't support Flusher")
			}

			// Send chunks
			chunks := []ollamaResponse{
				{Model: "llama3.2", Message: ollamaMessage{Content: "Hello"}, Done: false},
				{Model: "llama3.2", Message: ollamaMessage{Content: " "}, Done: false},
				{Model: "llama3.2", Message: ollamaMessage{Content: "World"}, Done: false},
				{Model: "llama3.2", Message: ollamaMessage{Content: ""}, Done: true, PromptEvalCount: 5, EvalCount: 3},
			}

			for _, chunk := range chunks {
				data, _ := json.Marshal(chunk)
				w.Write(data)
				w.Write([]byte("\n"))
				flusher.Flush()
			}
		}))
		defer server.Close()

		p := New(WithBaseURL(server.URL))
		stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
			Model: "llama3.2",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("StreamChat() error = %v", err)
		}

		var content strings.Builder
		for chunk := range stream.Ch {
			content.WriteString(chunk.Delta)
		}

		if content.String() != "Hello World" {
			t.Errorf("Content = %q, want %q", content.String(), "Hello World")
		}

		// Check final response
		select {
		case resp := <-stream.Final:
			if resp.Usage.PromptTokens != 5 {
				t.Errorf("PromptTokens = %d, want 5", resp.Usage.PromptTokens)
			}
			if resp.Usage.CompletionTokens != 3 {
				t.Errorf("CompletionTokens = %d, want 3", resp.Usage.CompletionTokens)
			}
		default:
			t.Error("Final response not received")
		}

		// Check no error
		select {
		case err := <-stream.Err:
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		default:
		}
	})

	t.Run("stream error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			flusher, _ := w.(http.Flusher)

			// Send one chunk then error
			chunk := ollamaResponse{Model: "llama3.2", Message: ollamaMessage{Content: "Hello"}, Done: false}
			data, _ := json.Marshal(chunk)
			w.Write(data)
			w.Write([]byte("\n"))
			flusher.Flush()

			// Send error
			errChunk := ollamaResponse{Error: "model crashed"}
			data, _ = json.Marshal(errChunk)
			w.Write(data)
			w.Write([]byte("\n"))
			flusher.Flush()
		}))
		defer server.Close()

		p := New(WithBaseURL(server.URL))
		stream, err := p.StreamChat(context.Background(), &core.ChatRequest{
			Model: "llama3.2",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("StreamChat() error = %v", err)
		}

		// Drain chunks
		for range stream.Ch {
		}

		// Check for error
		select {
		case err := <-stream.Err:
			if err == nil {
				t.Error("Expected error")
			}
			if !strings.Contains(err.Error(), "model crashed") {
				t.Errorf("Error = %v, should contain 'model crashed'", err)
			}
		default:
			t.Error("Error channel should have error")
		}
	})
}

// TestMapRequest tests request mapping.
func TestMapRequest(t *testing.T) {
	t.Run("basic request", func(t *testing.T) {
		temp := float32(0.7)
		maxTokens := 100

		req := &core.ChatRequest{
			Model: "llama3.2",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		}

		ollamaReq := mapRequest(req, false)

		if ollamaReq.Model != "llama3.2" {
			t.Errorf("Model = %q, want llama3.2", ollamaReq.Model)
		}
		if ollamaReq.Stream {
			t.Error("Stream should be false")
		}
		if len(ollamaReq.Messages) != 1 {
			t.Fatalf("Messages count = %d, want 1", len(ollamaReq.Messages))
		}
		if ollamaReq.Options == nil {
			t.Fatal("Options should not be nil")
		}
		if ollamaReq.Options.Temperature != 0.7 {
			t.Errorf("Temperature = %v, want 0.7", ollamaReq.Options.Temperature)
		}
		if ollamaReq.Options.NumPredict != 100 {
			t.Errorf("NumPredict = %d, want 100", ollamaReq.Options.NumPredict)
		}
	})

	t.Run("with thinking", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:           "qwen3",
			Messages:        []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			ReasoningEffort: core.ReasoningEffortHigh,
		}

		ollamaReq := mapRequest(req, false)

		if ollamaReq.Think == nil || !*ollamaReq.Think {
			t.Error("Think should be true")
		}
	})

	t.Run("no thinking for none effort", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:           "llama3.2",
			Messages:        []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			ReasoningEffort: core.ReasoningEffortNone,
		}

		ollamaReq := mapRequest(req, false)

		if ollamaReq.Think != nil {
			t.Errorf("Think should be nil, got %v", *ollamaReq.Think)
		}
	})
}

// TestMapResponse tests response mapping.
func TestMapResponse(t *testing.T) {
	t.Run("basic response", func(t *testing.T) {
		resp := &ollamaResponse{
			Model:           "llama3.2",
			CreatedAt:       "2024-01-01T00:00:00Z",
			Message:         ollamaMessage{Role: "assistant", Content: "Hello!"},
			Done:            true,
			PromptEvalCount: 10,
			EvalCount:       5,
		}

		result := mapResponse(resp)

		if result.Output != "Hello!" {
			t.Errorf("Output = %q, want Hello!", result.Output)
		}
		if result.Model != "llama3.2" {
			t.Errorf("Model = %q, want llama3.2", result.Model)
		}
		if result.Usage.PromptTokens != 10 {
			t.Errorf("PromptTokens = %d, want 10", result.Usage.PromptTokens)
		}
		if result.Usage.CompletionTokens != 5 {
			t.Errorf("CompletionTokens = %d, want 5", result.Usage.CompletionTokens)
		}
		if result.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", result.Usage.TotalTokens)
		}
	})

	t.Run("with tool calls", func(t *testing.T) {
		resp := &ollamaResponse{
			Model: "llama3.2",
			Message: ollamaMessage{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{Function: ollamaFunctionCall{Name: "weather", Arguments: map[string]interface{}{"city": "NYC"}}},
				},
			},
			Done: true,
		}

		result := mapResponse(resp)

		if len(result.ToolCalls) != 1 {
			t.Fatalf("ToolCalls count = %d, want 1", len(result.ToolCalls))
		}
		if result.ToolCalls[0].Name != "weather" {
			t.Errorf("ToolCall.Name = %q, want weather", result.ToolCalls[0].Name)
		}
	})

	t.Run("with thinking", func(t *testing.T) {
		resp := &ollamaResponse{
			Model: "qwen3",
			Message: ollamaMessage{
				Role:     "assistant",
				Content:  "36",
				Thinking: "15% of 240 = 36",
			},
			Done: true,
		}

		result := mapResponse(resp)

		if result.Reasoning == nil {
			t.Fatal("Reasoning should not be nil")
		}
		if len(result.Reasoning.Summary) != 1 {
			t.Fatalf("Reasoning.Summary length = %d, want 1", len(result.Reasoning.Summary))
		}
		if result.Reasoning.Summary[0] != "15% of 240 = 36" {
			t.Errorf("Reasoning.Summary[0] = %q, want %q", result.Reasoning.Summary[0], "15% of 240 = 36")
		}
	})
}

// TestMapToolCalls tests tool call mapping.
func TestMapToolCalls(t *testing.T) {
	calls := []ollamaToolCall{
		{Function: ollamaFunctionCall{Name: "func1", Arguments: map[string]interface{}{"a": "1"}}},
		{Function: ollamaFunctionCall{Name: "func2", Arguments: map[string]interface{}{"b": "2"}}},
	}

	result := mapToolCalls(calls)

	if len(result) != 2 {
		t.Fatalf("Result length = %d, want 2", len(result))
	}

	// Check IDs are generated
	if result[0].ID != "call_0" {
		t.Errorf("ID[0] = %q, want call_0", result[0].ID)
	}
	if result[1].ID != "call_1" {
		t.Errorf("ID[1] = %q, want call_1", result[1].ID)
	}

	// Check names
	if result[0].Name != "func1" {
		t.Errorf("Name[0] = %q, want func1", result[0].Name)
	}
	if result[1].Name != "func2" {
		t.Errorf("Name[1] = %q, want func2", result[1].Name)
	}

	// Check arguments are JSON
	var args1 map[string]string
	if err := json.Unmarshal(result[0].Arguments, &args1); err != nil {
		t.Errorf("Failed to unmarshal Arguments[0]: %v", err)
	}
	if args1["a"] != "1" {
		t.Errorf("Arguments[0][a] = %q, want 1", args1["a"])
	}
}

// TestMapOllamaError tests error mapping.
func TestMapOllamaError(t *testing.T) {
	tests := []struct {
		status   int
		message  string
		wantCode string
	}{
		{400, "bad request", "bad_request"},
		{404, "model not found", "model_not_found"},
		{429, "rate limited", "rate_limited"},
		{500, "internal error", "internal_error"},
		{502, "gateway error", "gateway_error"},
		{401, "unauthorized", "unauthorized"},
		{403, "forbidden", "forbidden"},
		{418, "teapot", "unknown"},
	}

	for _, tt := range tests {
		err := mapOllamaError(tt.status, tt.message)
		provErr, ok := err.(*core.ProviderError)
		if !ok {
			t.Errorf("mapOllamaError(%d) should return *core.ProviderError", tt.status)
			continue
		}
		if provErr.Code != tt.wantCode {
			t.Errorf("mapOllamaError(%d).Code = %q, want %q", tt.status, provErr.Code, tt.wantCode)
		}
		if provErr.Message != tt.message {
			t.Errorf("mapOllamaError(%d).Message = %q, want %q", tt.status, provErr.Message, tt.message)
		}
		if provErr.Provider != "ollama" {
			t.Errorf("mapOllamaError(%d).Provider = %q, want ollama", tt.status, provErr.Provider)
		}
	}
}

// TestParseErrorResponse tests error response parsing.
func TestParseErrorResponse(t *testing.T) {
	t.Run("json error", func(t *testing.T) {
		body := `{"error": "model not found"}`
		resp := &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader(body)),
		}

		err := parseErrorResponse(resp)
		provErr, ok := err.(*core.ProviderError)
		if !ok {
			t.Fatalf("Error should be *core.ProviderError, got %T", err)
		}
		if provErr.Message != "model not found" {
			t.Errorf("Message = %q, want %q", provErr.Message, "model not found")
		}
	})

	t.Run("plain text error", func(t *testing.T) {
		body := "Something went wrong"
		resp := &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader(body)),
		}

		err := parseErrorResponse(resp)
		provErr, ok := err.(*core.ProviderError)
		if !ok {
			t.Fatalf("Error should be *core.ProviderError, got %T", err)
		}
		if provErr.Message != "Something went wrong" {
			t.Errorf("Message = %q, want %q", provErr.Message, "Something went wrong")
		}
	})
}

// mockTool is a simple tool implementation for testing.
type mockTool struct {
	name        string
	description string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }
