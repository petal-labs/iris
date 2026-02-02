//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/voyageai"
)

func TestVoyageAI_ContextualizedEmbeddings_Basic(t *testing.T) {
	skipIfNoVoyageAPIKey(t)

	apiKey := getVoyageAPIKey(t)
	provider := voyageai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.CreateContextualizedEmbeddings(ctx, &core.ContextualizedEmbeddingRequest{
		Model: "voyage-context-3",
		Inputs: [][]string{
			{"The quick brown fox", "jumps over the lazy dog"},
		},
		InputType: core.InputTypeDocument,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	// Should have 1 document
	if len(resp.Embeddings) != 1 {
		t.Fatalf("len(Embeddings) = %d, want 1", len(resp.Embeddings))
	}

	// Document should have 2 chunks
	if len(resp.Embeddings[0]) != 2 {
		t.Fatalf("len(Embeddings[0]) = %d, want 2", len(resp.Embeddings[0]))
	}

	// Default dimension is 1024
	if len(resp.Embeddings[0][0].Vector) != 1024 {
		t.Errorf("len(Vector) = %d, want 1024", len(resp.Embeddings[0][0].Vector))
	}

	if resp.Usage.TotalTokens == 0 {
		t.Error("Usage.TotalTokens should be > 0")
	}

	t.Logf("Contextualized embeddings: %d docs, %d chunks, %d dims, %d tokens",
		len(resp.Embeddings), len(resp.Embeddings[0]),
		len(resp.Embeddings[0][0].Vector), resp.Usage.TotalTokens)
}

func TestVoyageAI_ContextualizedEmbeddings_MultipleDocuments(t *testing.T) {
	skipIfNoVoyageAPIKey(t)

	apiKey := getVoyageAPIKey(t)
	provider := voyageai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.CreateContextualizedEmbeddings(ctx, &core.ContextualizedEmbeddingRequest{
		Model: "voyage-context-3",
		Inputs: [][]string{
			{"Document 1, chunk 1", "Document 1, chunk 2"},
			{"Document 2, chunk 1", "Document 2, chunk 2", "Document 2, chunk 3"},
		},
		InputType: core.InputTypeDocument,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	// Should have 2 documents
	if len(resp.Embeddings) != 2 {
		t.Fatalf("len(Embeddings) = %d, want 2", len(resp.Embeddings))
	}

	// First document has 2 chunks
	if len(resp.Embeddings[0]) != 2 {
		t.Errorf("len(Embeddings[0]) = %d, want 2", len(resp.Embeddings[0]))
	}

	// Second document has 3 chunks
	if len(resp.Embeddings[1]) != 3 {
		t.Errorf("len(Embeddings[1]) = %d, want 3", len(resp.Embeddings[1]))
	}

	t.Logf("Multiple docs: doc1=%d chunks, doc2=%d chunks",
		len(resp.Embeddings[0]), len(resp.Embeddings[1]))
}

func TestVoyageAI_ContextualizedEmbeddings_WithDimensions(t *testing.T) {
	skipIfNoVoyageAPIKey(t)

	apiKey := getVoyageAPIKey(t)
	provider := voyageai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dims := 256
	resp, err := provider.CreateContextualizedEmbeddings(ctx, &core.ContextualizedEmbeddingRequest{
		Model:           "voyage-context-3",
		Inputs:          [][]string{{"Test chunk"}},
		InputType:       core.InputTypeDocument,
		OutputDimension: &dims,
	})
	if err != nil {
		t.Fatalf("CreateContextualizedEmbeddings() error = %v", err)
	}

	if len(resp.Embeddings[0][0].Vector) != 256 {
		t.Errorf("len(Vector) = %d, want 256", len(resp.Embeddings[0][0].Vector))
	}

	t.Logf("Custom dimensions: %d", len(resp.Embeddings[0][0].Vector))
}
