package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestCountTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/messages/count_tokens" {
			t.Errorf("path = %q, want /v1/messages/count_tokens", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q, want test-key", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != DefaultVersion {
			t.Errorf("anthropic-version = %q, want %q", r.Header.Get("anthropic-version"), DefaultVersion)
		}

		var body struct {
			Model    string             `json:"model"`
			System   string             `json:"system"`
			Messages []anthropicMessage `json:"messages"`
			Tools    []anthropicTool    `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != string(ModelClaudeSonnet5) {
			t.Errorf("model = %q, want %q", body.Model, ModelClaudeSonnet5)
		}
		if body.System != "Be concise." {
			t.Errorf("system = %q, want Be concise.", body.System)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
			t.Errorf("messages = %#v, want one user message", body.Messages)
		}
		if len(body.Tools) != 1 || body.Tools[0].Name != "weather" {
			t.Errorf("tools = %#v, want weather tool", body.Tools)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"input_tokens": 37})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))
	response, err := provider.CountTokens(context.Background(), tokenCountRequest(ModelClaudeSonnet5))
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}
	if response.InputTokens != 37 {
		t.Errorf("InputTokens = %d, want 37", response.InputTokens)
	}
}

func TestCountTokensValidationAndAuth(t *testing.T) {
	tests := []struct {
		name     string
		provider *Anthropic
		ctx      context.Context
		request  *core.ChatRequest
		wantAuth bool
	}{
		{name: "empty API key", provider: New(""), ctx: context.Background(), request: tokenCountRequest(ModelClaudeSonnet5), wantAuth: true},
		{name: "nil context", provider: New("test-key"), ctx: nil, request: tokenCountRequest(ModelClaudeSonnet5)},
		{name: "nil request", provider: New("test-key"), ctx: context.Background(), request: nil},
		{name: "missing model", provider: New("test-key"), ctx: context.Background(), request: tokenCountRequest("")},
		{name: "missing messages", provider: New("test-key"), ctx: context.Background(), request: &core.ChatRequest{Model: ModelClaudeSonnet5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:staticcheck // The nil-context case verifies public API validation.
			_, err := tt.provider.CountTokens(tt.ctx, tt.request)
			if err == nil {
				t.Fatal("CountTokens() should reject invalid input")
			}
			if tt.wantAuth && !errors.Is(err, core.ErrUnauthorized) {
				t.Errorf("error = %v, want ErrUnauthorized", err)
			}
		})
	}
}

func TestCountTokensErrorsAndTimeout(t *testing.T) {
	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"type":  "error",
				"error": map[string]string{"type": "rate_limit_error", "message": "slow down"},
			})
		}))
		defer server.Close()

		provider := New("test-key", WithBaseURL(server.URL))
		_, err := provider.CountTokens(context.Background(), tokenCountRequest(ModelClaudeSonnet5))
		if !errors.Is(err, core.ErrRateLimited) {
			t.Fatalf("error = %v, want ErrRateLimited", err)
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"input_tokens":`))
		}))
		defer server.Close()

		provider := New("test-key", WithBaseURL(server.URL))
		_, err := provider.CountTokens(context.Background(), tokenCountRequest(ModelClaudeSonnet5))
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("error = %v, want ErrDecode", err)
		}
	})

	t.Run("provider timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		t.Cleanup(func() { go server.Close() })

		provider := New("test-key", WithBaseURL(server.URL), WithTimeout(20*time.Millisecond))
		_, err := provider.CountTokens(context.Background(), tokenCountRequest(ModelClaudeSonnet5))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context.DeadlineExceeded", err)
		}
	})
}

func tokenCountRequest(model core.ModelID) *core.ChatRequest {
	return &core.ChatRequest{
		Model: model,
		Messages: []core.Message{
			{Role: core.RoleSystem, Content: "Be concise."},
			{Role: core.RoleUser, Content: "What is the weather?"},
		},
		Tools: []core.Tool{tokenCountTool{}},
	}
}

type tokenCountTool struct{}

func (tokenCountTool) Name() string        { return "weather" }
func (tokenCountTool) Description() string { return "Get the weather" }
func (tokenCountTool) Schema() core.ToolSchema {
	return core.ToolSchema{JSONSchema: json.RawMessage(`{"type":"object"}`)}
}

func TestCountTokensSupportsCapability(t *testing.T) {
	provider := New("test-key")
	if !provider.Supports(core.FeatureTokenCounting) {
		t.Error("Anthropic should report token-counting support")
	}
	if _, ok := core.AsTokenCounter(provider); !ok {
		t.Error("Anthropic should implement core.TokenCounter")
	}
	if got := strings.TrimSpace(string(core.FeatureTokenCounting)); got != "token_counting" {
		t.Errorf("FeatureTokenCounting = %q, want token_counting", got)
	}
}
