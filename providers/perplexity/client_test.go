package perplexity

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
	t.Run("successful response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", r.Method)
			}
			if r.URL.Path != "/chat/completions" {
				t.Errorf("Path = %q, want /chat/completions", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization = %q, want Bearer test-key", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
			}

			// Verify request body
			body, _ := io.ReadAll(r.Body)
			var req perplexityRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("Failed to unmarshal request: %v", err)
			}
			if req.Model != "sonar" {
				t.Errorf("Model = %q, want %q", req.Model, "sonar")
			}
			if req.Stream {
				t.Error("Stream should be false")
			}

			// Write response
			w.Header().Set("x-request-id", "req-123")
			json.NewEncoder(w).Encode(perplexityResponse{
				ID:    "resp-456",
				Model: "sonar",
				Choices: []perplexityChoice{
					{
						Index:        0,
						Message:      &perplexityRespMsg{Role: "assistant", Content: "Hello!"},
						FinishReason: "stop",
					},
				},
				Usage: &perplexityUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		resp, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err != nil {
			t.Fatalf("doChat() error = %v", err)
		}

		if resp.ID != "resp-456" {
			t.Errorf("ID = %q, want %q", resp.ID, "resp-456")
		}
		if resp.Model != "sonar" {
			t.Errorf("Model = %q, want %q", resp.Model, "sonar")
		}
		if resp.Output != "Hello!" {
			t.Errorf("Output = %q, want %q", resp.Output, "Hello!")
		}
		if resp.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
		}
	})

	t.Run("with tool calls", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(perplexityResponse{
				ID:    "resp-789",
				Model: "sonar",
				Choices: []perplexityChoice{
					{
						Index: 0,
						Message: &perplexityRespMsg{
							Role:    "assistant",
							Content: "",
							ToolCalls: []perplexityToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: perplexityFunctionCall{
										Name:      "get_weather",
										Arguments: `{"city":"Tokyo"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		resp, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "What's the weather?"}},
		})

		if err != nil {
			t.Fatalf("doChat() error = %v", err)
		}

		if len(resp.ToolCalls) != 1 {
			t.Fatalf("ToolCalls count = %d, want 1", len(resp.ToolCalls))
		}
		if resp.ToolCalls[0].Name != "get_weather" {
			t.Errorf("ToolCall.Name = %q, want %q", resp.ToolCalls[0].Name, "get_weather")
		}
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("x-request-id", "req-err")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "Invalid API key",
					"type":    "authentication_error",
				},
			})
		}))
		defer server.Close()

		p := New("bad-key", WithBaseURL(server.URL))
		_, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err == nil {
			t.Fatal("doChat() should return error")
		}

		var provErr *core.ProviderError
		if !errors.As(err, &provErr) {
			t.Fatal("err should be *core.ProviderError")
		}

		if provErr.Status != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", provErr.Status, http.StatusUnauthorized)
		}
		if provErr.RequestID != "req-err" {
			t.Errorf("RequestID = %q, want %q", provErr.RequestID, "req-err")
		}
		if !errors.Is(err, core.ErrUnauthorized) {
			t.Error("err should wrap core.ErrUnauthorized")
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		_, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if !errors.Is(err, core.ErrRateLimited) {
			t.Error("err should wrap core.ErrRateLimited")
		}
	})

	t.Run("invalid response JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		_, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if !errors.Is(err, core.ErrDecode) {
			t.Error("err should wrap core.ErrDecode")
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New("test-key", WithBaseURL("http://localhost:0"))
		_, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if !errors.Is(err, core.ErrNetwork) {
			t.Error("err should wrap core.ErrNetwork")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Slow response - but we'll cancel before it completes
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		p := New("test-key", WithBaseURL(server.URL))
		_, err := p.doChat(ctx, &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err == nil {
			t.Fatal("doChat() should return error on cancelled context")
		}
	})
}

func TestChatCompletionsPath(t *testing.T) {
	if chatCompletionsPath != "/chat/completions" {
		t.Errorf("chatCompletionsPath = %q, want %q", chatCompletionsPath, "/chat/completions")
	}
}
