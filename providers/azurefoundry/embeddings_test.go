package azurefoundry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestCreateEmbeddingsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}

		// Verify API key header
		if r.Header.Get("api-key") != "test-key" {
			t.Errorf("api-key = %q, want test-key", r.Header.Get("api-key"))
		}

		// Parse request body
		var req azureEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if len(req.Input) != 2 {
			t.Errorf("len(Input) = %d, want 2", len(req.Input))
		}

		// Send response
		resp := azureEmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-3-large",
			Data: []azureEmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: []interface{}{0.1, 0.2, 0.3},
				},
				{
					Object:    "embedding",
					Index:     1,
					Embedding: []interface{}{0.4, 0.5, 0.6},
				},
			},
			Usage: azureEmbeddingUsage{
				PromptTokens: 10,
				TotalTokens:  10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{Text: "Hello world"},
			{Text: "Goodbye world"},
		},
	})

	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors) != 2 {
		t.Fatalf("len(Vectors) = %d, want 2", len(resp.Vectors))
	}

	if resp.Vectors[0].Index != 0 {
		t.Errorf("Vectors[0].Index = %d, want 0", resp.Vectors[0].Index)
	}

	if len(resp.Vectors[0].Vector) != 3 {
		t.Errorf("len(Vectors[0].Vector) = %d, want 3", len(resp.Vectors[0].Vector))
	}

	if resp.Model != "text-embedding-3-large" {
		t.Errorf("Model = %q, want text-embedding-3-large", resp.Model)
	}

	if resp.Usage.TotalTokens != 10 {
		t.Errorf("Usage.TotalTokens = %d, want 10", resp.Usage.TotalTokens)
	}
}

func TestCreateEmbeddingsWithMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := azureEmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-3-large",
			Data: []azureEmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: []interface{}{0.1, 0.2, 0.3},
				},
			},
			Usage: azureEmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{
				ID:       "doc-1",
				Text:     "Hello world",
				Metadata: map[string]string{"source": "test"},
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if resp.Vectors[0].ID != "doc-1" {
		t.Errorf("Vectors[0].ID = %q, want doc-1", resp.Vectors[0].ID)
	}

	if resp.Vectors[0].Metadata["source"] != "test" {
		t.Errorf("Vectors[0].Metadata[source] = %v, want test", resp.Vectors[0].Metadata["source"])
	}
}

func TestCreateEmbeddingsWithDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req azureEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Dimensions == nil || *req.Dimensions != 256 {
			t.Errorf("Dimensions = %v, want 256", req.Dimensions)
		}

		resp := azureEmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-3-large",
			Data: []azureEmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: make([]interface{}, 256), // 256-dim embedding
				},
			},
			Usage: azureEmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	dims := 256
	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{Text: "Hello world"},
		},
		Dimensions: &dims,
	})

	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors[0].Vector) != 256 {
		t.Errorf("len(Vector) = %d, want 256", len(resp.Vectors[0].Vector))
	}
}

func TestCreateEmbeddingsWithInputType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req azureEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.InputType != "query" {
			t.Errorf("InputType = %q, want query", req.InputType)
		}

		resp := azureEmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-3-large",
			Data: []azureEmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: []interface{}{0.1, 0.2, 0.3},
				},
			},
			Usage: azureEmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{Text: "Hello world"},
		},
		InputType: core.InputTypeQuery,
	})

	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}
}

func TestCreateEmbeddingsBase64Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := azureEmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-3-large",
			Data: []azureEmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: "AAAA/wAAgD8AAAA/", // base64 encoded
				},
			},
			Usage: azureEmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model:          "text-embedding-3-large",
		EncodingFormat: core.EncodingFormatBase64,
		Input: []core.EmbeddingInput{
			{Text: "Hello world"},
		},
	})

	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if resp.Vectors[0].VectorB64 != "AAAA/wAAgD8AAAA/" {
		t.Errorf("VectorB64 = %q, want AAAA/wAAgD8AAAA/", resp.Vectors[0].VectorB64)
	}
}

func TestCreateEmbeddingsError400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Invalid model","code":"invalid_model"}}`))
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "invalid-model",
		Input: []core.EmbeddingInput{
			{Text: "Hello world"},
		},
	})

	if !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound, got %v", err)
	}
}

func TestCreateEmbeddingsError429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
	}))
	defer server.Close()

	p := New(server.URL, "test-key")
	_, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{Text: "Hello world"},
		},
	})

	if !errors.Is(err, core.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestBuildEmbeddingRequest(t *testing.T) {
	dims := 256
	req := &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{Text: "Hello"},
			{Text: "World"},
		},
		EncodingFormat: core.EncodingFormatFloat,
		Dimensions:     &dims,
		User:           "user-123",
		InputType:      core.InputTypeDocument,
	}

	result := buildEmbeddingRequest(req)

	if result.Model != "text-embedding-3-large" {
		t.Errorf("Model = %q, want text-embedding-3-large", result.Model)
	}

	if len(result.Input) != 2 {
		t.Fatalf("len(Input) = %d, want 2", len(result.Input))
	}

	if result.Input[0] != "Hello" {
		t.Errorf("Input[0] = %q, want Hello", result.Input[0])
	}

	if result.EncodingFormat != "float" {
		t.Errorf("EncodingFormat = %q, want float", result.EncodingFormat)
	}

	if result.Dimensions == nil || *result.Dimensions != 256 {
		t.Errorf("Dimensions = %v, want 256", result.Dimensions)
	}

	if result.User != "user-123" {
		t.Errorf("User = %q, want user-123", result.User)
	}

	if result.InputType != "document" {
		t.Errorf("InputType = %q, want document", result.InputType)
	}
}

func TestBuildEmbeddingRequestMinimal(t *testing.T) {
	req := &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{Text: "Hello"},
		},
	}

	result := buildEmbeddingRequest(req)

	if result.EncodingFormat != "" {
		t.Errorf("EncodingFormat = %q, want empty", result.EncodingFormat)
	}

	if result.Dimensions != nil {
		t.Errorf("Dimensions = %v, want nil", result.Dimensions)
	}

	if result.InputType != "" {
		t.Errorf("InputType = %q, want empty", result.InputType)
	}
}

func TestMapEmbeddingResponse(t *testing.T) {
	req := &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{ID: "doc-1", Text: "Hello", Metadata: map[string]string{"key": "value"}},
		},
	}

	resp := &azureEmbeddingResponse{
		Object: "list",
		Model:  "text-embedding-3-large",
		Data: []azureEmbeddingData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: []interface{}{0.1, 0.2, 0.3},
			},
		},
		Usage: azureEmbeddingUsage{
			PromptTokens: 5,
			TotalTokens:  5,
		},
	}

	result := mapEmbeddingResponse(resp, req)

	if result.Model != "text-embedding-3-large" {
		t.Errorf("Model = %q, want text-embedding-3-large", result.Model)
	}

	if len(result.Vectors) != 1 {
		t.Fatalf("len(Vectors) = %d, want 1", len(result.Vectors))
	}

	vec := result.Vectors[0]
	if vec.ID != "doc-1" {
		t.Errorf("ID = %q, want doc-1", vec.ID)
	}

	if vec.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want value", vec.Metadata["key"])
	}

	if len(vec.Vector) != 3 {
		t.Errorf("len(Vector) = %d, want 3", len(vec.Vector))
	}

	if result.Usage.TotalTokens != 5 {
		t.Errorf("Usage.TotalTokens = %d, want 5", result.Usage.TotalTokens)
	}
}

func TestMapEmbeddingResponseOutOfBoundsIndex(t *testing.T) {
	req := &core.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: []core.EmbeddingInput{
			{ID: "doc-1", Text: "Hello"},
		},
	}

	resp := &azureEmbeddingResponse{
		Object: "list",
		Model:  "text-embedding-3-large",
		Data: []azureEmbeddingData{
			{
				Object:    "embedding",
				Index:     5, // Out of bounds
				Embedding: []interface{}{0.1, 0.2, 0.3},
			},
		},
		Usage: azureEmbeddingUsage{
			PromptTokens: 5,
			TotalTokens:  5,
		},
	}

	result := mapEmbeddingResponse(resp, req)

	// Should handle gracefully - no ID/Metadata copied
	if result.Vectors[0].ID != "" {
		t.Errorf("ID = %q, want empty (out of bounds index)", result.Vectors[0].ID)
	}
}
