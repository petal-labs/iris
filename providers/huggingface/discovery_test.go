package huggingface

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestGetModelStatus(t *testing.T) {
	t.Run("warm status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/api/models/meta-llama/Llama-3-8B-Instruct" {
				t.Errorf("Path = %q, want /api/models/meta-llama/Llama-3-8B-Instruct", r.URL.Path)
			}
			if r.URL.Query().Get("expand") != "inference" {
				t.Errorf("expand param = %q, want inference", r.URL.Query().Get("expand"))
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization = %q, want Bearer test-key", r.Header.Get("Authorization"))
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":        "meta-llama/Llama-3-8B-Instruct",
				"inference": "warm",
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		status, err := p.GetModelStatus(context.Background(), "meta-llama/Llama-3-8B-Instruct")

		if err != nil {
			t.Fatalf("GetModelStatus() error = %v", err)
		}
		if status != ModelStatusWarm {
			t.Errorf("status = %v, want %v", status, ModelStatusWarm)
		}
	})

	t.Run("unknown status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"id":        "some/model",
				"inference": "",
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		status, err := p.GetModelStatus(context.Background(), "some/model")

		if err != nil {
			t.Fatalf("GetModelStatus() error = %v", err)
		}
		if status != ModelStatusUnknown {
			t.Errorf("status = %v, want %v", status, ModelStatusUnknown)
		}
	})

	t.Run("model not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "Model not found",
					"type":    "not_found_error",
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.GetModelStatus(context.Background(), "invalid/model")

		if err == nil {
			t.Fatal("GetModelStatus() should return error for 404")
		}

		var provErr *core.ProviderError
		if !errors.As(err, &provErr) {
			t.Fatal("err should be *core.ProviderError")
		}
		if provErr.Status != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", provErr.Status, http.StatusNotFound)
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New("test-key", WithBaseURL("http://localhost:0"), WithHubAPIBaseURL("http://localhost:0/api"))
		_, err := p.GetModelStatus(context.Background(), "some/model")

		if !errors.Is(err, core.ErrNetwork) {
			t.Error("err should wrap core.ErrNetwork")
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.GetModelStatus(context.Background(), "some/model")

		if !errors.Is(err, core.ErrDecode) {
			t.Error("err should wrap core.ErrDecode")
		}
	})

	t.Run("without API key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Error("Authorization header should be empty")
			}
			json.NewEncoder(w).Encode(map[string]any{
				"id":        "public/model",
				"inference": "warm",
			})
		}))
		defer server.Close()

		p := New("", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		status, err := p.GetModelStatus(context.Background(), "public/model")

		if err != nil {
			t.Fatalf("GetModelStatus() error = %v", err)
		}
		if status != ModelStatusWarm {
			t.Errorf("status = %v, want %v", status, ModelStatusWarm)
		}
	})
}

func TestGetModelProviders(t *testing.T) {
	t.Run("with providers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("expand") != "inferenceProviderMapping" {
				t.Errorf("expand param = %q, want inferenceProviderMapping", r.URL.Query().Get("expand"))
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":        "meta-llama/Llama-3-8B-Instruct",
				"inference": "warm",
				"inferenceProviderMapping": map[string]any{
					"cerebras": map[string]any{
						"status":     "live",
						"providerId": "llama-3-8b",
						"task":       "conversational",
					},
					"together": map[string]any{
						"status":     "staging",
						"providerId": "meta-llama/Llama-3-8B-Instruct",
						"task":       "text-generation",
					},
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		providers, err := p.GetModelProviders(context.Background(), "meta-llama/Llama-3-8B-Instruct")

		if err != nil {
			t.Fatalf("GetModelProviders() error = %v", err)
		}
		if len(providers) != 2 {
			t.Fatalf("providers count = %d, want 2", len(providers))
		}

		// Find cerebras provider
		var cerebras *InferenceProvider
		for i := range providers {
			if providers[i].Name == "cerebras" {
				cerebras = &providers[i]
				break
			}
		}

		if cerebras == nil {
			t.Fatal("cerebras provider not found")
		}
		if cerebras.Status != "live" {
			t.Errorf("cerebras.Status = %q, want %q", cerebras.Status, "live")
		}
		if cerebras.ProviderID != "llama-3-8b" {
			t.Errorf("cerebras.ProviderID = %q, want %q", cerebras.ProviderID, "llama-3-8b")
		}
		if cerebras.Task != "conversational" {
			t.Errorf("cerebras.Task = %q, want %q", cerebras.Task, "conversational")
		}
	})

	t.Run("empty providers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"id":                       "some/model",
				"inference":                "",
				"inferenceProviderMapping": map[string]any{},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		providers, err := p.GetModelProviders(context.Background(), "some/model")

		if err != nil {
			t.Fatalf("GetModelProviders() error = %v", err)
		}
		if len(providers) != 0 {
			t.Errorf("providers count = %d, want 0", len(providers))
		}
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "Invalid token",
				},
			})
		}))
		defer server.Close()

		p := New("bad-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.GetModelProviders(context.Background(), "some/model")

		if !errors.Is(err, core.ErrUnauthorized) {
			t.Error("err should wrap core.ErrUnauthorized")
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New("test-key", WithBaseURL("http://localhost:0"), WithHubAPIBaseURL("http://localhost:0/api"))
		_, err := p.GetModelProviders(context.Background(), "some/model")

		if !errors.Is(err, core.ErrNetwork) {
			t.Error("err should wrap core.ErrNetwork")
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.GetModelProviders(context.Background(), "some/model")

		if !errors.Is(err, core.ErrDecode) {
			t.Error("err should wrap core.ErrDecode")
		}
	})
}

func TestListModels(t *testing.T) {
	t.Run("basic list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/api/models" {
				t.Errorf("Path = %q, want /api/models", r.URL.Path)
			}

			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":           "meta-llama/Llama-3-8B-Instruct",
					"pipeline_tag": "text-generation",
					"inference":    "warm",
				},
				{
					"id":           "google/gemma-2-2b-it",
					"pipeline_tag": "text-generation",
					"inference":    "warm",
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		models, err := p.ListModels(context.Background(), ListModelsOptions{})

		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
		if len(models) != 2 {
			t.Fatalf("models count = %d, want 2", len(models))
		}
		if models[0].ID != "meta-llama/Llama-3-8B-Instruct" {
			t.Errorf("models[0].ID = %q, want %q", models[0].ID, "meta-llama/Llama-3-8B-Instruct")
		}
		if models[0].PipelineTag != "text-generation" {
			t.Errorf("models[0].PipelineTag = %q, want %q", models[0].PipelineTag, "text-generation")
		}
		if models[0].Inference != "warm" {
			t.Errorf("models[0].Inference = %q, want %q", models[0].Inference, "warm")
		}
	})

	t.Run("with provider filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("inference_provider") != "cerebras" {
				t.Errorf("inference_provider param = %q, want cerebras", r.URL.Query().Get("inference_provider"))
			}
			json.NewEncoder(w).Encode([]map[string]any{})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{
			Provider: "cerebras",
		})

		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
	})

	t.Run("with pipeline tag filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("pipeline_tag") != "text-generation" {
				t.Errorf("pipeline_tag param = %q, want text-generation", r.URL.Query().Get("pipeline_tag"))
			}
			json.NewEncoder(w).Encode([]map[string]any{})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{
			PipelineTag: "text-generation",
		})

		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
	})

	t.Run("with limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "10" {
				t.Errorf("limit param = %q, want 10", r.URL.Query().Get("limit"))
			}
			json.NewEncoder(w).Encode([]map[string]any{})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{
			Limit: 10,
		})

		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
	})

	t.Run("with all options", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if query.Get("inference_provider") != "all" {
				t.Errorf("inference_provider = %q, want all", query.Get("inference_provider"))
			}
			if query.Get("pipeline_tag") != "text-generation" {
				t.Errorf("pipeline_tag = %q, want text-generation", query.Get("pipeline_tag"))
			}
			if query.Get("limit") != "50" {
				t.Errorf("limit = %q, want 50", query.Get("limit"))
			}
			json.NewEncoder(w).Encode([]map[string]any{})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{
			Provider:    "all",
			PipelineTag: "text-generation",
			Limit:       50,
		})

		if err != nil {
			t.Fatalf("ListModels() error = %v", err)
		}
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "Internal error",
				},
			})
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{})

		if !errors.Is(err, core.ErrServer) {
			t.Error("err should wrap core.ErrServer")
		}
	})

	t.Run("network error", func(t *testing.T) {
		p := New("test-key", WithBaseURL("http://localhost:0"), WithHubAPIBaseURL("http://localhost:0/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{})

		if !errors.Is(err, core.ErrNetwork) {
			t.Error("err should wrap core.ErrNetwork")
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{not valid`))
		}))
		defer server.Close()

		p := New("test-key", WithBaseURL(server.URL), WithHubAPIBaseURL(server.URL+"/api"))
		_, err := p.ListModels(context.Background(), ListModelsOptions{})

		if !errors.Is(err, core.ErrDecode) {
			t.Error("err should wrap core.ErrDecode")
		}
	})
}

func TestModelStatusString(t *testing.T) {
	tests := []struct {
		status ModelStatus
		want   string
	}{
		{ModelStatusWarm, "warm"},
		{ModelStatusUnknown, "unknown"},
		{ModelStatus(""), "unknown"},
		{ModelStatus("other"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferenceProviderString(t *testing.T) {
	p := InferenceProvider{
		Name:       "cerebras",
		Status:     "live",
		Task:       "conversational",
		ProviderID: "llama-3-8b",
	}

	expected := "cerebras (live, conversational)"
	if got := p.String(); got != expected {
		t.Errorf("String() = %q, want %q", got, expected)
	}
}

func TestInferenceProviderIsLive(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"live lowercase", "live", true},
		{"live uppercase", "LIVE", true},
		{"live mixed", "Live", true},
		{"staging", "staging", false},
		{"empty", "", false},
		{"other", "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := InferenceProvider{Status: tt.status}
			if got := p.IsLive(); got != tt.want {
				t.Errorf("IsLive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHubAPIURL(t *testing.T) {
	p := New("test-key")

	expected := HubAPIBaseURL + "/models/test"
	if got := p.hubAPIURL("/models/test"); got != expected {
		t.Errorf("hubAPIURL() = %q, want %q", got, expected)
	}

	// Test with custom Hub API URL
	customURL := "https://custom.huggingface.co/api"
	p2 := New("test-key", WithHubAPIBaseURL(customURL))
	expected2 := customURL + "/models/test"
	if got := p2.hubAPIURL("/models/test"); got != expected2 {
		t.Errorf("hubAPIURL() with custom = %q, want %q", got, expected2)
	}
}
