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

func TestCreateEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("Path = %q, want /v1/embeddings", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header incorrect")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header incorrect")
		}

		// Verify request body
		var req openAIEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("Model = %q, want text-embedding-3-small", req.Model)
		}
		if len(req.Input) != 1 || req.Input[0] != "hello world" {
			t.Errorf("Input = %v, want [hello world]", req.Input)
		}

		// Return response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIEmbeddingResponse{
			Object: "list",
			Data: []openAIEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2, 0.3}},
			},
			Model: "text-embedding-3-small",
			Usage: openAIEmbeddingUsage{PromptTokens: 2, TotalTokens: 2},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{{Text: "hello world"}},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors) != 1 {
		t.Fatalf("len(Vectors) = %d, want 1", len(resp.Vectors))
	}
	if resp.Vectors[0].Index != 0 {
		t.Errorf("Vectors[0].Index = %d, want 0", resp.Vectors[0].Index)
	}
	if len(resp.Vectors[0].Vector) != 3 {
		t.Errorf("len(Vector) = %d, want 3", len(resp.Vectors[0].Vector))
	}
	if resp.Usage.PromptTokens != 2 {
		t.Errorf("Usage.PromptTokens = %d, want 2", resp.Usage.PromptTokens)
	}
}

func TestCreateEmbeddings_BatchInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIEmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Input) != 3 {
			t.Errorf("len(Input) = %d, want 3", len(req.Input))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIEmbeddingResponse{
			Object: "list",
			Data: []openAIEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1}},
				{Object: "embedding", Index: 1, Embedding: []float64{0.2}},
				{Object: "embedding", Index: 2, Embedding: []float64{0.3}},
			},
			Model: "text-embedding-3-small",
			Usage: openAIEmbeddingUsage{PromptTokens: 6, TotalTokens: 6},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{
			{Text: "one"},
			{Text: "two"},
			{Text: "three"},
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors) != 3 {
		t.Fatalf("len(Vectors) = %d, want 3", len(resp.Vectors))
	}

	// Verify index alignment
	for i, vec := range resp.Vectors {
		if vec.Index != i {
			t.Errorf("Vectors[%d].Index = %d, want %d", i, vec.Index, i)
		}
	}
}

func TestCreateEmbeddings_WithIDAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIEmbeddingResponse{
			Object: "list",
			Data: []openAIEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1}},
				{Object: "embedding", Index: 1, Embedding: []float64{0.2}},
			},
			Model: "text-embedding-3-small",
			Usage: openAIEmbeddingUsage{PromptTokens: 4, TotalTokens: 4},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{
			{Text: "first", ID: "id-1", Metadata: map[string]string{"source": "doc1"}},
			{Text: "second", ID: "id-2", Metadata: map[string]string{"source": "doc2"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	// Verify ID passthrough
	if resp.Vectors[0].ID != "id-1" {
		t.Errorf("Vectors[0].ID = %q, want id-1", resp.Vectors[0].ID)
	}
	if resp.Vectors[1].ID != "id-2" {
		t.Errorf("Vectors[1].ID = %q, want id-2", resp.Vectors[1].ID)
	}

	// Verify Metadata passthrough
	if resp.Vectors[0].Metadata["source"] != "doc1" {
		t.Errorf("Vectors[0].Metadata[source] = %q, want doc1", resp.Vectors[0].Metadata["source"])
	}
	if resp.Vectors[1].Metadata["source"] != "doc2" {
		t.Errorf("Vectors[1].Metadata[source] = %q, want doc2", resp.Vectors[1].Metadata["source"])
	}
}

func TestCreateEmbeddings_Base64Format(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIEmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.EncodingFormat != "base64" {
			t.Errorf("EncodingFormat = %q, want base64", req.EncodingFormat)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIEmbeddingResponse{
			Object: "list",
			Data: []openAIEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: "SGVsbG8gV29ybGQ="},
			},
			Model: "text-embedding-3-small",
			Usage: openAIEmbeddingUsage{PromptTokens: 2, TotalTokens: 2},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model:          "text-embedding-3-small",
		Input:          []core.EmbeddingInput{{Text: "hello"}},
		EncodingFormat: core.EncodingFormatBase64,
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if resp.Vectors[0].VectorB64 != "SGVsbG8gV29ybGQ=" {
		t.Errorf("VectorB64 = %q, want SGVsbG8gV29ybGQ=", resp.Vectors[0].VectorB64)
	}
	if len(resp.Vectors[0].Vector) != 0 {
		t.Errorf("Vector should be empty for base64 format")
	}
}

func TestCreateEmbeddings_WithDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIEmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Dimensions == nil || *req.Dimensions != 256 {
			t.Errorf("Dimensions = %v, want 256", req.Dimensions)
		}

		// Return 256-dimension vector
		embedding := make([]float64, 256)
		for i := range embedding {
			embedding[i] = float64(i) * 0.001
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIEmbeddingResponse{
			Object: "list",
			Data: []openAIEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: embedding},
			},
			Model: "text-embedding-3-large",
			Usage: openAIEmbeddingUsage{PromptTokens: 2, TotalTokens: 2},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	dims := 256
	resp, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model:      "text-embedding-3-large",
		Input:      []core.EmbeddingInput{{Text: "hello"}},
		Dimensions: &dims,
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors[0].Vector) != 256 {
		t.Errorf("len(Vector) = %d, want 256", len(resp.Vectors[0].Vector))
	}
}

func TestCreateEmbeddings_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(openAIErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		})
	}))
	defer server.Close()

	provider := New("bad-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Expected ProviderError, got %T", err)
	}

	if provErr.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", provErr.Status, http.StatusUnauthorized)
	}
	if !errors.Is(provErr, core.ErrUnauthorized) {
		t.Errorf("Expected ErrUnauthorized, got %v", provErr.Err)
	}
}

func TestCreateEmbeddings_WithOrgAndProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify organization and project headers
		if r.Header.Get("OpenAI-Organization") != "org-123" {
			t.Errorf("OpenAI-Organization = %q, want org-123", r.Header.Get("OpenAI-Organization"))
		}
		if r.Header.Get("OpenAI-Project") != "proj-456" {
			t.Errorf("OpenAI-Project = %q, want proj-456", r.Header.Get("OpenAI-Project"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openAIEmbeddingResponse{
			Object: "list",
			Data: []openAIEmbeddingData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1}},
			},
			Model: "text-embedding-3-small",
			Usage: openAIEmbeddingUsage{PromptTokens: 2, TotalTokens: 2},
		})
	}))
	defer server.Close()

	provider := New("test-key",
		WithBaseURL(server.URL+"/v1"),
		WithOrgID("org-123"),
		WithProjectID("proj-456"),
	)

	_, err := provider.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}
}
