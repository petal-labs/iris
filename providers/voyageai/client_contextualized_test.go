package voyageai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestCreateContextualizedEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/contextualizedembeddings" {
			t.Errorf("Path = %q, want /v1/contextualizedembeddings", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header incorrect")
		}

		// Verify request body
		var req voyageContextualizedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Model != "voyage-context-3" {
			t.Errorf("Model = %q, want voyage-context-3", req.Model)
		}
		if len(req.Inputs) != 2 {
			t.Errorf("len(Inputs) = %d, want 2", len(req.Inputs))
		}
		if len(req.Inputs[0]) != 2 {
			t.Errorf("len(Inputs[0]) = %d, want 2", len(req.Inputs[0]))
		}

		// Return nested response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageContextualizedResponse{
			Object: "list",
			Data: []voyageContextualizedDocData{
				{
					Object: "list",
					Index:  0,
					Data: []voyageContextualizedChunkData{
						{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2}},
						{Object: "embedding", Index: 1, Embedding: []float64{0.3, 0.4}},
					},
				},
				{
					Object: "list",
					Index:  1,
					Data: []voyageContextualizedChunkData{
						{Object: "embedding", Index: 0, Embedding: []float64{0.5, 0.6}},
						{Object: "embedding", Index: 1, Embedding: []float64{0.7, 0.8}},
					},
				},
			},
			Model: "voyage-context-3",
			Usage: voyageContextualizedUsage{TotalTokens: 24},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model: "voyage-context-3",
		Inputs: [][]string{
			{"doc1_chunk1", "doc1_chunk2"},
			{"doc2_chunk1", "doc2_chunk2"},
		},
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	// Should have 2 documents
	if len(resp.Embeddings) != 2 {
		t.Fatalf("len(Embeddings) = %d, want 2", len(resp.Embeddings))
	}

	// Each document should have 2 chunks
	if len(resp.Embeddings[0]) != 2 {
		t.Errorf("len(Embeddings[0]) = %d, want 2", len(resp.Embeddings[0]))
	}
	if len(resp.Embeddings[1]) != 2 {
		t.Errorf("len(Embeddings[1]) = %d, want 2", len(resp.Embeddings[1]))
	}

	// Verify embedding values
	if resp.Embeddings[0][0].Vector[0] != 0.1 {
		t.Errorf("Embeddings[0][0].Vector[0] = %f, want 0.1", resp.Embeddings[0][0].Vector[0])
	}

	if resp.Usage.TotalTokens != 24 {
		t.Errorf("Usage.TotalTokens = %d, want 24", resp.Usage.TotalTokens)
	}
}

func TestCreateContextualizedEmbeddings_WithInputType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageContextualizedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.InputType != "document" {
			t.Errorf("InputType = %q, want document", req.InputType)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageContextualizedResponse{
			Object: "list",
			Data: []voyageContextualizedDocData{
				{Object: "list", Index: 0, Data: []voyageContextualizedChunkData{
					{Object: "embedding", Index: 0, Embedding: []float64{0.1}},
				}},
			},
			Model: "voyage-context-3",
			Usage: voyageContextualizedUsage{TotalTokens: 10},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model:     "voyage-context-3",
		Inputs:    [][]string{{"chunk1"}},
		InputType: core.InputTypeDocument,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}
}

func TestCreateContextualizedEmbeddings_WithDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageContextualizedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.OutputDimension == nil || *req.OutputDimension != 256 {
			t.Errorf("OutputDimension = %v, want 256", req.OutputDimension)
		}

		// Return 256-dimension embeddings
		embedding := make([]float64, 256)
		for i := range embedding {
			embedding[i] = float64(i) * 0.001
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageContextualizedResponse{
			Object: "list",
			Data: []voyageContextualizedDocData{
				{Object: "list", Index: 0, Data: []voyageContextualizedChunkData{
					{Object: "embedding", Index: 0, Embedding: embedding},
				}},
			},
			Model: "voyage-context-3",
			Usage: voyageContextualizedUsage{TotalTokens: 10},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	dims := 256
	resp, err := provider.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model:           "voyage-context-3",
		Inputs:          [][]string{{"chunk1"}},
		OutputDimension: &dims,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	if len(resp.Embeddings[0][0].Vector) != 256 {
		t.Errorf("len(Vector) = %d, want 256", len(resp.Embeddings[0][0].Vector))
	}
}

func TestCreateContextualizedEmbeddings_Base64Format(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageContextualizedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.EncodingFormat != "base64" {
			t.Errorf("EncodingFormat = %q, want base64", req.EncodingFormat)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageContextualizedResponse{
			Object: "list",
			Data: []voyageContextualizedDocData{
				{
					Object: "list",
					Index:  0,
					Data: []voyageContextualizedChunkData{
						{Object: "embedding", Index: 0, Embedding: "SGVsbG8gV29ybGQ="},
						{Object: "embedding", Index: 1, Embedding: "Q2h1bmsgdHdv"},
					},
				},
			},
			Model: "voyage-context-3",
			Usage: voyageContextualizedUsage{TotalTokens: 10},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model:          "voyage-context-3",
		Inputs:         [][]string{{"chunk1", "chunk2"}},
		EncodingFormat: core.EncodingFormatBase64,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	// Verify base64 values are properly mapped
	if resp.Embeddings[0][0].VectorB64 != "SGVsbG8gV29ybGQ=" {
		t.Errorf("Embeddings[0][0].VectorB64 = %q, want SGVsbG8gV29ybGQ=", resp.Embeddings[0][0].VectorB64)
	}
	if resp.Embeddings[0][1].VectorB64 != "Q2h1bmsgdHdv" {
		t.Errorf("Embeddings[0][1].VectorB64 = %q, want Q2h1bmsgdHdv", resp.Embeddings[0][1].VectorB64)
	}

	// Vector should be empty for base64 format
	if len(resp.Embeddings[0][0].Vector) != 0 {
		t.Errorf("Vector should be empty for base64 format, got length %d", len(resp.Embeddings[0][0].Vector))
	}
}

func TestCreateContextualizedEmbeddings_WithOutputDType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageContextualizedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.OutputDType != "int8" {
			t.Errorf("OutputDType = %q, want int8", req.OutputDType)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageContextualizedResponse{
			Object: "list",
			Data: []voyageContextualizedDocData{
				{
					Object: "list",
					Index:  0,
					Data: []voyageContextualizedChunkData{
						{Object: "embedding", Index: 0, Embedding: []float64{-128, 0, 127}},
					},
				},
			},
			Model: "voyage-context-3",
			Usage: voyageContextualizedUsage{TotalTokens: 10},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	resp, err := provider.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model:       "voyage-context-3",
		Inputs:      [][]string{{"chunk1"}},
		OutputDType: core.OutputDTypeInt8,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	if len(resp.Embeddings[0][0].Vector) != 3 {
		t.Errorf("len(Vector) = %d, want 3", len(resp.Embeddings[0][0].Vector))
	}
}

func TestCreateContextualizedEmbeddings_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(voyageErrorResponse{
			Detail: "Rate limit exceeded",
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL+"/v1"))

	_, err := provider.CreateContextualizedEmbeddings(context.Background(), &core.ContextualizedEmbeddingRequest{
		Model:  "voyage-context-3",
		Inputs: [][]string{{"chunk1"}},
	})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Expected ProviderError, got %T", err)
	}

	if !errors.Is(provErr, core.ErrRateLimited) {
		t.Errorf("Expected ErrRateLimited, got %v", provErr.Err)
	}
}
