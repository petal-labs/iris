package core

import (
	"context"
	"testing"
)

// mockContextualizedProvider implements ContextualizedEmbeddingProvider for testing.
type mockContextualizedProvider struct{}

func (m *mockContextualizedProvider) CreateContextualizedEmbeddings(ctx context.Context, req *ContextualizedEmbeddingRequest) (*ContextualizedEmbeddingResponse, error) {
	return &ContextualizedEmbeddingResponse{
		Embeddings: [][]EmbeddingVector{
			{{Index: 0, Vector: []float32{0.1, 0.2}}},
		},
		Model: "test-model",
		Usage: EmbeddingUsage{TotalTokens: 10},
	}, nil
}

func TestContextualizedEmbeddingProvider_Interface(t *testing.T) {
	var provider ContextualizedEmbeddingProvider = &mockContextualizedProvider{}

	resp, err := provider.CreateContextualizedEmbeddings(context.Background(), &ContextualizedEmbeddingRequest{
		Model:  "test-model",
		Inputs: [][]string{{"chunk1", "chunk2"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Embeddings) != 1 {
		t.Errorf("len(Embeddings) = %d, want 1", len(resp.Embeddings))
	}
}
