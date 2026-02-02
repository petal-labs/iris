package core

import "testing"

func TestFeature_ContextualizedEmbeddings(t *testing.T) {
	if FeatureContextualizedEmbeddings != "contextualized_embeddings" {
		t.Errorf("FeatureContextualizedEmbeddings = %q, want contextualized_embeddings", FeatureContextualizedEmbeddings)
	}
}

func TestFeature_Reranking(t *testing.T) {
	if FeatureReranking != "reranking" {
		t.Errorf("FeatureReranking = %q, want reranking", FeatureReranking)
	}
}
