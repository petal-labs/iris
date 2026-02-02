package core

import (
	"context"
	"testing"
)

// mockRerankerProvider implements RerankerProvider for testing.
type mockRerankerProvider struct{}

func (m *mockRerankerProvider) Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error) {
	return &RerankResponse{
		Results: []RerankResult{
			{Index: 1, RelevanceScore: 0.9},
			{Index: 0, RelevanceScore: 0.5},
		},
		Model: "test-reranker",
		Usage: RerankUsage{TotalTokens: 20},
	}, nil
}

func TestRerankerProvider_Interface(t *testing.T) {
	var provider RerankerProvider = &mockRerankerProvider{}

	resp, err := provider.Rerank(context.Background(), &RerankRequest{
		Model:     "test-reranker",
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2", len(resp.Results))
	}
	// Results should be sorted by relevance (descending)
	if resp.Results[0].RelevanceScore < resp.Results[1].RelevanceScore {
		t.Error("Results should be sorted by descending relevance")
	}
}
