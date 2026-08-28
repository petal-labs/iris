package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %q, want /api/tags", r.URL.Path)
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("X-Custom = %q, want value", r.Header.Get("X-Custom"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"models": []any{
				map[string]any{"name": "qwen3:8b", "model": "qwen3:8b", "size": 5_000_000_000},
				map[string]any{"model": "nomic-embed-text:latest"},
				map[string]any{},
			},
		})
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL), WithHeaders(http.Header{"X-Custom": []string{"value"}}))
	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
	if models[0].ID != "qwen3:8b" || models[0].DisplayName != "qwen3:8b" {
		t.Errorf("models[0] = %#v, want qwen3:8b", models[0])
	}
	if models[1].ID != "nomic-embed-text:latest" {
		t.Errorf("models[1] = %#v, want legacy model-field fallback", models[1])
	}
	if len(models[0].Capabilities) != 0 {
		t.Errorf("capabilities = %v, want empty because /api/tags does not advertise them", models[0].Capabilities)
	}
}

func TestModelsUsesLocalTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"models": []any{map[string]any{"name": "local-only:latest"}},
		})
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL))
	models := provider.Models()
	if len(models) != 1 || models[0].ID != "local-only:latest" {
		t.Fatalf("Models() = %#v, want local /api/tags result", models)
	}
}

func TestModelsReturnsEmptyLocalCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL))
	if models := provider.Models(); len(models) != 0 {
		t.Fatalf("Models() = %#v, want empty local catalog", models)
	}
}

func TestModelsFallsBackToIllustrativeCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "offline"})
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL))
	models := provider.Models()
	if !hasModel(models, "llama3.2") {
		t.Fatalf("Models() fallback = %#v, want illustrative llama3.2 entry", models)
	}
}

func TestListModelsAvoidsDuplicateAPIPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %q, want /api/tags", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
	}))
	defer server.Close()

	provider := New(WithBaseURL(server.URL + "/api"))
	if _, err := provider.ListModels(context.Background()); err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
}

func TestListModelsErrors(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		provider := New()
		//nolint:staticcheck // Intentionally verify that the public API rejects a nil context.
		_, err := provider.ListModels(nil)
		if err == nil {
			t.Fatal("ListModels() should reject a nil context")
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "busy"})
		}))
		defer server.Close()

		provider := New(WithBaseURL(server.URL))
		_, err := provider.ListModels(context.Background())
		if !errors.Is(err, core.ErrRateLimited) {
			t.Fatalf("error = %v, want ErrRateLimited", err)
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"models":`))
		}))
		defer server.Close()

		provider := New(WithBaseURL(server.URL))
		_, err := provider.ListModels(context.Background())
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("error = %v, want ErrDecode", err)
		}
	})

	t.Run("provider timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		t.Cleanup(func() { go server.Close() })

		provider := New(WithBaseURL(server.URL), WithTimeout(20*time.Millisecond))
		_, err := provider.ListModels(context.Background())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context.DeadlineExceeded", err)
		}
	})
}

func hasModel(models []core.ModelInfo, id core.ModelID) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}
