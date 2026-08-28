package core

import (
	"context"
	"testing"
)

type unwrapTestEmbedder struct {
	Provider
}

func (p *unwrapTestEmbedder) CreateEmbeddings(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error) {
	return &EmbeddingResponse{Model: "unwrapped"}, nil
}

type unwrapTestWrapper struct {
	Provider
}

func (w *unwrapTestWrapper) Unwrap() Provider {
	return w.Provider
}

type cyclicUnwrapTestWrapper struct {
	Provider
}

func (w *cyclicUnwrapTestWrapper) Unwrap() Provider {
	return w
}

func TestAsProviderCapabilityFollowsNestedUnwrapChain(t *testing.T) {
	embedder := &unwrapTestEmbedder{}
	wrapped := &unwrapTestWrapper{Provider: &unwrapTestWrapper{Provider: embedder}}

	got, ok := AsEmbeddingProvider(wrapped)
	if !ok || got != embedder {
		t.Fatalf("AsEmbeddingProvider() = (%v, %v), want underlying embedder", got, ok)
	}
	response, err := got.CreateEmbeddings(context.Background(), &EmbeddingRequest{})
	if err != nil || response.Model != "unwrapped" {
		t.Fatalf("unwrapped CreateEmbeddings() = (%v, %v), want usable capability", response, err)
	}
}

func TestAsProviderCapabilityHandlesInvalidUnwrapChains(t *testing.T) {
	tests := map[string]Provider{
		"nil provider": nil,
		"nil unwrap":   &unwrapTestWrapper{},
		"cycle":        &cyclicUnwrapTestWrapper{},
	}
	for name, provider := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := AsEmbeddingProvider(provider)
			if ok || got != nil {
				t.Errorf("AsEmbeddingProvider() = (%v, %v), want (nil, false)", got, ok)
			}
		})
	}
}
