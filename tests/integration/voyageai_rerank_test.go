//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/voyageai"
)

func TestVoyageAI_Rerank_Basic(t *testing.T) {
	skipIfNoVoyageAPIKey(t)

	apiKey := getVoyageAPIKey(t)
	provider := voyageai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Rerank(ctx, &core.RerankRequest{
		Model: "rerank-2.5-lite",
		Query: "What is the capital of France?",
		Documents: []string{
			"Berlin is the capital of Germany.",
			"Paris is the capital of France.",
			"London is the capital of England.",
		},
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(resp.Results))
	}

	// The Paris document (index 1) should be ranked first
	if resp.Results[0].Index != 1 {
		t.Errorf("Results[0].Index = %d, want 1 (Paris document)", resp.Results[0].Index)
	}

	// Scores should be in descending order
	for i := 1; i < len(resp.Results); i++ {
		if resp.Results[i].RelevanceScore > resp.Results[i-1].RelevanceScore {
			t.Errorf("Results not sorted by descending relevance: %f > %f",
				resp.Results[i].RelevanceScore, resp.Results[i-1].RelevanceScore)
		}
	}

	t.Logf("Top result: index=%d, score=%.4f, tokens=%d",
		resp.Results[0].Index, resp.Results[0].RelevanceScore, resp.Usage.TotalTokens)
}

func TestVoyageAI_Rerank_WithTopK(t *testing.T) {
	skipIfNoVoyageAPIKey(t)

	apiKey := getVoyageAPIKey(t)
	provider := voyageai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topK := 2
	resp, err := provider.Rerank(ctx, &core.RerankRequest{
		Model: "rerank-2.5-lite",
		Query: "programming languages",
		Documents: []string{
			"Python is a popular programming language.",
			"The weather is nice today.",
			"JavaScript is used for web development.",
			"I like to eat pizza.",
		},
		TopK: &topK,
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if len(resp.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2", len(resp.Results))
	}

	t.Logf("Top 2 results: indices=%d,%d", resp.Results[0].Index, resp.Results[1].Index)
}

func TestVoyageAI_Rerank_WithReturnDocuments(t *testing.T) {
	skipIfNoVoyageAPIKey(t)

	apiKey := getVoyageAPIKey(t)
	provider := voyageai.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Rerank(ctx, &core.RerankRequest{
		Model:           "rerank-2.5-lite",
		Query:           "capital city",
		Documents:       []string{"Paris is beautiful.", "London is historic."},
		ReturnDocuments: true,
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	// Documents should be returned
	for i, result := range resp.Results {
		if result.Document == "" {
			t.Errorf("Results[%d].Document should not be empty", i)
		}
	}

	t.Logf("Top result document: %q", resp.Results[0].Document)
}
