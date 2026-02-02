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

func TestRerank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/rerank" {
			t.Errorf("Path = %q, want /rerank", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header incorrect")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type header incorrect")
		}

		// Verify request body
		var req voyageRerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Model != "rerank-2" {
			t.Errorf("Model = %q, want rerank-2", req.Model)
		}
		if req.Query != "What is the capital of France?" {
			t.Errorf("Query = %q, want 'What is the capital of France?'", req.Query)
		}
		if len(req.Documents) != 3 {
			t.Errorf("len(Documents) = %d, want 3", len(req.Documents))
		}

		// Return response with sorted results (descending relevance)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageRerankResponse{
			Object: "list",
			Data: []voyageRerankData{
				{Index: 1, RelevanceScore: 0.95},
				{Index: 0, RelevanceScore: 0.72},
				{Index: 2, RelevanceScore: 0.35},
			},
			Model: "rerank-2",
			Usage: voyageRerankUsage{TotalTokens: 50},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	resp, err := provider.Rerank(context.Background(), &core.RerankRequest{
		Model: "rerank-2",
		Query: "What is the capital of France?",
		Documents: []string{
			"Berlin is the capital of Germany.",
			"Paris is the capital of France.",
			"Madrid is the capital of Spain.",
		},
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(resp.Results))
	}

	// Results should be in descending order of relevance
	if resp.Results[0].Index != 1 {
		t.Errorf("Results[0].Index = %d, want 1", resp.Results[0].Index)
	}
	if resp.Results[0].RelevanceScore != 0.95 {
		t.Errorf("Results[0].RelevanceScore = %f, want 0.95", resp.Results[0].RelevanceScore)
	}
	if resp.Results[1].Index != 0 {
		t.Errorf("Results[1].Index = %d, want 0", resp.Results[1].Index)
	}
	if resp.Results[2].Index != 2 {
		t.Errorf("Results[2].Index = %d, want 2", resp.Results[2].Index)
	}

	if resp.Model != "rerank-2" {
		t.Errorf("Model = %q, want rerank-2", resp.Model)
	}
	if resp.Usage.TotalTokens != 50 {
		t.Errorf("Usage.TotalTokens = %d, want 50", resp.Usage.TotalTokens)
	}
}

func TestRerank_WithTopK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageRerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.TopK == nil || *req.TopK != 2 {
			t.Errorf("TopK = %v, want 2", req.TopK)
		}

		// Return only top 2 results
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageRerankResponse{
			Object: "list",
			Data: []voyageRerankData{
				{Index: 1, RelevanceScore: 0.95},
				{Index: 0, RelevanceScore: 0.72},
			},
			Model: "rerank-2",
			Usage: voyageRerankUsage{TotalTokens: 50},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	topK := 2
	resp, err := provider.Rerank(context.Background(), &core.RerankRequest{
		Model: "rerank-2",
		Query: "What is the capital of France?",
		Documents: []string{
			"Berlin is the capital of Germany.",
			"Paris is the capital of France.",
			"Madrid is the capital of Spain.",
		},
		TopK: &topK,
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(resp.Results))
	}

	// Verify only top 2 results are returned
	if resp.Results[0].Index != 1 {
		t.Errorf("Results[0].Index = %d, want 1", resp.Results[0].Index)
	}
	if resp.Results[1].Index != 0 {
		t.Errorf("Results[1].Index = %d, want 0", resp.Results[1].Index)
	}
}

func TestRerank_WithReturnDocuments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageRerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if !req.ReturnDocuments {
			t.Errorf("ReturnDocuments = %v, want true", req.ReturnDocuments)
		}

		// Return response with document text included
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageRerankResponse{
			Object: "list",
			Data: []voyageRerankData{
				{Index: 1, RelevanceScore: 0.95, Document: "Paris is the capital of France."},
				{Index: 0, RelevanceScore: 0.72, Document: "Berlin is the capital of Germany."},
			},
			Model: "rerank-2",
			Usage: voyageRerankUsage{TotalTokens: 50},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	resp, err := provider.Rerank(context.Background(), &core.RerankRequest{
		Model: "rerank-2",
		Query: "What is the capital of France?",
		Documents: []string{
			"Berlin is the capital of Germany.",
			"Paris is the capital of France.",
		},
		ReturnDocuments: true,
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(resp.Results))
	}

	// Verify document text is included
	if resp.Results[0].Document != "Paris is the capital of France." {
		t.Errorf("Results[0].Document = %q, want 'Paris is the capital of France.'", resp.Results[0].Document)
	}
	if resp.Results[1].Document != "Berlin is the capital of Germany." {
		t.Errorf("Results[1].Document = %q, want 'Berlin is the capital of Germany.'", resp.Results[1].Document)
	}
}

func TestRerank_WithTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageRerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Truncation == nil || *req.Truncation != true {
			t.Errorf("Truncation = %v, want true", req.Truncation)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(voyageRerankResponse{
			Object: "list",
			Data: []voyageRerankData{
				{Index: 0, RelevanceScore: 0.85},
			},
			Model: "rerank-2",
			Usage: voyageRerankUsage{TotalTokens: 30},
		})
	}))
	defer server.Close()

	provider := New("test-key", WithBaseURL(server.URL))

	truncate := true
	_, err := provider.Rerank(context.Background(), &core.RerankRequest{
		Model:      "rerank-2",
		Query:      "test query",
		Documents:  []string{"test document"},
		Truncation: &truncate,
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
}

func TestRerank_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(voyageErrorResponse{
			Detail: "Invalid API key",
		})
	}))
	defer server.Close()

	provider := New("bad-key", WithBaseURL(server.URL))

	_, err := provider.Rerank(context.Background(), &core.RerankRequest{
		Model:     "rerank-2",
		Query:     "test query",
		Documents: []string{"test document"},
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
