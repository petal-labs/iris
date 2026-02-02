//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
)

func TestOpenAI_Embeddings_SingleInput(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.CreateEmbeddings(ctx, &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{
			{Text: "Hello, world!"},
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors) != 1 {
		t.Fatalf("len(Vectors) = %d, want 1", len(resp.Vectors))
	}

	// text-embedding-3-small returns 1536 dimensions by default
	if len(resp.Vectors[0].Vector) != 1536 {
		t.Errorf("len(Vector) = %d, want 1536", len(resp.Vectors[0].Vector))
	}

	if resp.Usage.PromptTokens == 0 {
		t.Error("Usage.PromptTokens should be > 0")
	}

	t.Logf("Embedding dimensions: %d, tokens: %d", len(resp.Vectors[0].Vector), resp.Usage.PromptTokens)
}

func TestOpenAI_Embeddings_BatchInput(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.CreateEmbeddings(ctx, &core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: []core.EmbeddingInput{
			{Text: "First text", ID: "doc-1"},
			{Text: "Second text", ID: "doc-2"},
			{Text: "Third text", ID: "doc-3"},
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors) != 3 {
		t.Fatalf("len(Vectors) = %d, want 3", len(resp.Vectors))
	}

	// Verify IDs are passed through
	for i, vec := range resp.Vectors {
		expectedID := []string{"doc-1", "doc-2", "doc-3"}[vec.Index]
		if vec.ID != expectedID {
			t.Errorf("Vectors[%d].ID = %q, want %q", i, vec.ID, expectedID)
		}
	}

	t.Logf("Batch embeddings: %d vectors, %d total tokens", len(resp.Vectors), resp.Usage.TotalTokens)
}

func TestOpenAI_Embeddings_WithDimensions(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	provider := openai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dims := 256
	resp, err := provider.CreateEmbeddings(ctx, &core.EmbeddingRequest{
		Model:      "text-embedding-3-small",
		Input:      []core.EmbeddingInput{{Text: "Test with custom dimensions"}},
		Dimensions: &dims,
	})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Vectors[0].Vector) != 256 {
		t.Errorf("len(Vector) = %d, want 256", len(resp.Vectors[0].Vector))
	}

	t.Logf("Custom dimensions: %d", len(resp.Vectors[0].Vector))
}
