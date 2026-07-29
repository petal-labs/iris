package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestCreateEmbeddings_SingleInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/embed" {
			t.Errorf("Path = %q, want /api/embed", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header incorrect")
		}

		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Model != "nomic-embed-text" {
			t.Errorf("Model = %q, want nomic-embed-text", req.Model)
		}
		if len(req.Input) != 1 || req.Input[0] != "hello world" {
			t.Errorf("Input = %v, want [hello world]", req.Input)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model:           "nomic-embed-text",
			Embeddings:      [][]float32{{0.1, 0.2, 0.3}},
			PromptEvalCount: 8,
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "nomic-embed-text",
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
	if resp.Vectors[0].Vector[0] != 0.1 {
		t.Errorf("Vector[0] = %v, want 0.1", resp.Vectors[0].Vector[0])
	}
	if resp.Model != "nomic-embed-text" {
		t.Errorf("Model = %q, want nomic-embed-text", resp.Model)
	}
	if resp.Usage.PromptTokens != 8 {
		t.Errorf("Usage.PromptTokens = %d, want 8", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("Usage.TotalTokens = %d, want 8", resp.Usage.TotalTokens)
	}
}

func TestCreateEmbeddings_BatchPreservesOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if len(req.Input) != 3 {
			t.Errorf("len(Input) = %d, want 3", len(req.Input))
		}
		if req.Input[0] != "one" || req.Input[1] != "two" || req.Input[2] != "three" {
			t.Errorf("Input = %v, want [one two three]", req.Input)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model: "nomic-embed-text",
			Embeddings: [][]float32{
				{0.1},
				{0.2},
				{0.3},
			},
			PromptEvalCount: 6,
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "nomic-embed-text",
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
	want := []float32{0.1, 0.2, 0.3}
	for i, vec := range resp.Vectors {
		if vec.Index != i {
			t.Errorf("Vectors[%d].Index = %d, want %d", i, vec.Index, i)
		}
		if len(vec.Vector) != 1 || vec.Vector[0] != want[i] {
			t.Errorf("Vectors[%d].Vector = %v, want [%v]", i, vec.Vector, want[i])
		}
	}
}

func TestCreateEmbeddings_PreservesIDAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model: "nomic-embed-text",
			Embeddings: [][]float32{
				{0.1},
				{0.2},
			},
			PromptEvalCount: 4,
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "nomic-embed-text",
		Input: []core.EmbeddingInput{
			{Text: "first", ID: "id-1", Metadata: map[string]string{"source": "doc1"}},
			{Text: "second", ID: "id-2", Metadata: map[string]string{"source": "doc2"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if resp.Vectors[0].ID != "id-1" {
		t.Errorf("Vectors[0].ID = %q, want id-1", resp.Vectors[0].ID)
	}
	if resp.Vectors[1].ID != "id-2" {
		t.Errorf("Vectors[1].ID = %q, want id-2", resp.Vectors[1].ID)
	}
	if resp.Vectors[0].Metadata["source"] != "doc1" {
		t.Errorf("Vectors[0].Metadata[source] = %q, want doc1", resp.Vectors[0].Metadata["source"])
	}
	if resp.Vectors[1].Metadata["source"] != "doc2" {
		t.Errorf("Vectors[1].Metadata[source] = %q, want doc2", resp.Vectors[1].Metadata["source"])
	}
}

func TestCreateEmbeddings_MapsDimensionsAndTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Dimensions == nil || *req.Dimensions != 768 {
			t.Errorf("Dimensions = %v, want 768", req.Dimensions)
		}
		if req.Truncate == nil || *req.Truncate != true {
			t.Errorf("Truncate = %v, want true", req.Truncate)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model:           "nomic-embed-text",
			Embeddings:      [][]float32{{0.1, 0.2}},
			PromptEvalCount: 2,
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	dims := 768
	truncate := true
	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model:      "nomic-embed-text",
		Input:      []core.EmbeddingInput{{Text: "hello"}},
		Dimensions: &dims,
		Truncation: &truncate,
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}
}

func TestCreateEmbeddings_OmitsDimensionsAndTruncationWhenUnset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if _, ok := raw["dimensions"]; ok {
			t.Error("dimensions should be omitted when unset")
		}
		if _, ok := raw["truncate"]; ok {
			t.Error("truncate should be omitted when unset")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model:      "nomic-embed-text",
			Embeddings: [][]float32{{0.1}},
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "nomic-embed-text",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}
}

func TestCreateEmbeddings_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ollamaErrorResponse{
			Error: `model "nomic-embed-text" not found, try pulling it first`,
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "nomic-embed-text",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("Expected ProviderError, got %T", err)
	}
	if provErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", provErr.Status, http.StatusNotFound)
	}
	if provErr.Provider != "ollama" {
		t.Errorf("Provider = %q, want ollama", provErr.Provider)
	}
	if !errors.Is(provErr, core.ErrBadRequest) {
		t.Errorf("Expected ErrBadRequest for 404, got %v", provErr.Err)
	}
}

func TestCreateEmbeddings_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is not json"))
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "nomic-embed-text",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err == nil {
		t.Fatal("Expected error for malformed response, got nil")
	}
}

func TestCreateEmbeddings_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.CreateEmbeddings(ctx, &core.EmbeddingRequest{
		Model: "nomic-embed-text",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err == nil {
		t.Fatal("Expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, core.ErrNetwork) {
		t.Errorf("Expected context.Canceled or ErrNetwork, got %v", err)
	}
}

func TestProviderSupports_Embeddings(t *testing.T) {
	p := New()
	if !p.Supports(core.FeatureEmbeddings) {
		t.Error("Supports(FeatureEmbeddings) = false, want true")
	}
}
