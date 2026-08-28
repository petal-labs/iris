package gemini

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
		wantPath := "/v1beta/models/" + string(ModelGemini35Flash) + ":countTokens"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("x-goog-api-key = %q, want test-key", r.Header.Get("x-goog-api-key"))
		}

		var body struct {
			GenerateContentRequest geminiRequest `json:"generateContentRequest"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		request := body.GenerateContentRequest
		if request.Model != "models/"+string(ModelGemini35Flash) {
			t.Errorf("nested model = %q, want models/%s", request.Model, ModelGemini35Flash)
		}
		if request.SystemInstruction == nil || request.SystemInstruction.Parts[0].Text != "Be concise." {
			t.Errorf("system instruction = %#v, want Be concise.", request.SystemInstruction)
		}
		if len(request.Contents) != 1 || request.Contents[0].Role != "user" {
			t.Errorf("contents = %#v, want one user content", request.Contents)
		}
		if len(request.Tools) != 1 || request.Tools[0].FunctionDeclarations[0].Name != "weather" {
			t.Errorf("tools = %#v, want weather tool", request.Tools)
		}
		if request.GenerationConfig != nil {
			t.Errorf("generationConfig = %#v, want omitted", request.GenerationConfig)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"totalTokens": 29})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))
	response, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}
	if response.InputTokens != 29 {
		t.Errorf("InputTokens = %d, want 29", response.InputTokens)
	}
}

func TestCountTokensValidatesInputAndAuth(t *testing.T) {
	tests := []struct {
		name     string
		provider *Gemini
		ctx      context.Context
		request  *core.ChatRequest
		wantAuth bool
	}{
		{name: "empty API key", provider: New(""), ctx: context.Background(), request: validGeminiTokenCountRequest(ModelGemini35Flash), wantAuth: true},
		{name: "nil context", provider: New("test-key"), ctx: nil, request: validGeminiTokenCountRequest(ModelGemini35Flash)},
		{name: "nil request", provider: New("test-key"), ctx: context.Background(), request: nil},
		{name: "missing model", provider: New("test-key"), ctx: context.Background(), request: validGeminiTokenCountRequest("")},
		{name: "unsafe model", provider: New("test-key"), ctx: context.Background(), request: validGeminiTokenCountRequest("../unsafe")},
		{name: "missing messages", provider: New("test-key"), ctx: context.Background(), request: &core.ChatRequest{Model: ModelGemini35Flash}},
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
				"error": map[string]any{"code": 429, "message": "quota exhausted", "status": "RESOURCE_EXHAUSTED"},
			})
		}))
		defer server.Close()

		provider := New("test-key", WithBaseURL(server.URL))
		_, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
		if !errors.Is(err, core.ErrRateLimited) {
			t.Fatalf("error = %v, want ErrRateLimited", err)
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"totalTokens":`))
		}))
		defer server.Close()

		provider := New("test-key", WithBaseURL(server.URL))
		_, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("error = %v, want ErrDecode", err)
		}
	})

	t.Run("missing token count", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		provider := New("test-key", WithBaseURL(server.URL))
		_, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("error = %v, want ErrDecode", err)
		}
	})

	t.Run("oversized response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(strings.Repeat("x", maxTokenCountResponseBytes+1)))
		}))
		defer server.Close()

		provider := New("test-key", WithBaseURL(server.URL))
		_, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("error = %v, want ErrDecode", err)
		}
	})

	t.Run("invalid base URL", func(t *testing.T) {
		provider := New("test-key", WithBaseURL("://invalid"))
		_, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
		if !errors.Is(err, core.ErrNetwork) {
			t.Fatalf("error = %v, want ErrNetwork", err)
		}
	})

	t.Run("invalid tool schema", func(t *testing.T) {
		request := validGeminiTokenCountRequest(ModelGemini35Flash)
		request.Tools = []core.Tool{invalidTokenCountTool{}}
		provider := New("test-key")
		_, err := provider.CountTokens(context.Background(), request)
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
		_, err := provider.CountTokens(context.Background(), validGeminiTokenCountRequest(ModelGemini35Flash))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context.DeadlineExceeded", err)
		}
	})
}

func validGeminiTokenCountRequest(model core.ModelID) *core.ChatRequest {
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

type invalidTokenCountTool struct{ tokenCountTool }

func (invalidTokenCountTool) Schema() core.ToolSchema {
	return core.ToolSchema{JSONSchema: json.RawMessage(`[`)}
}

func TestCountTokensSupportsCapability(t *testing.T) {
	provider := New("test-key")
	if !provider.Supports(core.FeatureTokenCounting) {
		t.Error("Gemini should report token-counting support")
	}
	if _, ok := core.AsTokenCounter(provider); !ok {
		t.Error("Gemini should implement core.TokenCounter")
	}
}
