package huggingface

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
			if r.URL.Path != "/v1/chat/completions" {
				t.Errorf("Path = %q, want /v1/chat/completions", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization = %q, want Bearer test-key", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
			}

			// Verify request body
			body, _ := io.ReadAll(r.Body)
			var req hfRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("Failed to unmarshal request: %v", err)
			}
			if req.Model != "meta-llama/Llama-3-8B-Instruct" {
				t.Errorf("Model = %q, want %q", req.Model, "meta-llama/Llama-3-8B-Instruct")
			}
			if req.Stream {
				t.Error("Stream should be false")
			}

			// Write response
			w.Header().Set("x-request-id", "req-123")
			json.NewEncoder(w).Encode(hfResponse{
				ID:    "resp-456",
				Model: "meta-llama/Llama-3-8B-Instruct",
				Choices: []hfChoice{
					{
						Index:        0,
						Message:      hfRespMsg{Role: "assistant", Content: "Hello!"},
						FinishReason: "stop",
					},
				},
				Usage: hfUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL))
		resp, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err != nil {
			t.Fatalf("doChat() error = %v", err)
		}

		if resp.ID != "resp-456" {
			t.Errorf("ID = %q, want %q", resp.ID, "resp-456")
		}
		if resp.Model != "meta-llama/Llama-3-8B-Instruct" {
			t.Errorf("Model = %q, want %q", resp.Model, "meta-llama/Llama-3-8B-Instruct")
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
			json.NewEncoder(w).Encode(hfResponse{
				ID:    "resp-789",
				Model: "meta-llama/Llama-3-8B-Instruct",
				Choices: []hfChoice{
					{
						Index: 0,
						Message: hfRespMsg{
							Role:    "assistant",
							Content: "",
							ToolCalls: []hfToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: hfFunctionCall{
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
			Model:    "meta-llama/Llama-3-8B-Instruct",
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
			Model:    "meta-llama/Llama-3-8B-Instruct",
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
			Model:    "meta-llama/Llama-3-8B-Instruct",
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
			Model:    "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if !errors.Is(err, core.ErrDecode) {
			t.Error("err should wrap core.ErrDecode")
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New("test-key", WithBaseURL("http://localhost:0"))
		_, err := p.doChat(context.Background(), &core.ChatRequest{
			Model:    "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if !errors.Is(err, core.ErrNetwork) {
			t.Error("err should wrap core.ErrNetwork")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		p := New("test-key", WithBaseURL(server.URL))
		_, err := p.doChat(ctx, &core.ChatRequest{
			Model:    "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hi"}},
		})

		if err == nil {
			t.Fatal("doChat() should return error on cancelled context")
		}
	})
}

func TestBuildModelString(t *testing.T) {
	tests := []struct {
		name           string
		model          core.ModelID
		providerPolicy string
		want           string
	}{
		{
			name:           "no policy",
			model:          "meta-llama/Llama-3-8B-Instruct",
			providerPolicy: "",
			want:           "meta-llama/Llama-3-8B-Instruct",
		},
		{
			name:           "auto policy (default)",
			model:          "meta-llama/Llama-3-8B-Instruct",
			providerPolicy: PolicyAuto,
			want:           "meta-llama/Llama-3-8B-Instruct",
		},
		{
			name:           "fastest policy",
			model:          "meta-llama/Llama-3-8B-Instruct",
			providerPolicy: PolicyFastest,
			want:           "meta-llama/Llama-3-8B-Instruct:fastest",
		},
		{
			name:           "cheapest policy",
			model:          "meta-llama/Llama-3-8B-Instruct",
			providerPolicy: PolicyCheapest,
			want:           "meta-llama/Llama-3-8B-Instruct:cheapest",
		},
		{
			name:           "specific provider",
			model:          "meta-llama/Llama-3-8B-Instruct",
			providerPolicy: "cerebras",
			want:           "meta-llama/Llama-3-8B-Instruct:cerebras",
		},
		{
			name:           "model already has suffix",
			model:          "meta-llama/Llama-3-8B-Instruct:together",
			providerPolicy: PolicyFastest,
			want:           "meta-llama/Llama-3-8B-Instruct:together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.providerPolicy != "" {
				opts = append(opts, WithProviderPolicy(tt.providerPolicy))
			}
			p := New("test-key", opts...)

			got := p.buildModelString(tt.model)
			if got != tt.want {
				t.Errorf("buildModelString(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestChatURL(t *testing.T) {
	p := New("test-key")
	expected := DefaultBaseURL + "/v1/chat/completions"
	if got := p.chatURL(); got != expected {
		t.Errorf("chatURL() = %q, want %q", got, expected)
	}

	p = New("test-key", WithBaseURL("https://custom.api.co"))
	expected = "https://custom.api.co/v1/chat/completions"
	if got := p.chatURL(); got != expected {
		t.Errorf("chatURL() = %q, want %q", got, expected)
	}
}

func TestChatCompletionsPath(t *testing.T) {
	if chatCompletionsPath != "/v1/chat/completions" {
		t.Errorf("chatCompletionsPath = %q, want %q", chatCompletionsPath, "/v1/chat/completions")
	}
}
