package core

import (
	"context"
	"testing"
)

type capabilityBaseProvider struct {
	id string
}

func (p *capabilityBaseProvider) ID() string          { return p.id }
func (*capabilityBaseProvider) Models() []ModelInfo   { return nil }
func (*capabilityBaseProvider) Supports(Feature) bool { return false }
func (*capabilityBaseProvider) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotSupported
}
func (*capabilityBaseProvider) StreamChat(context.Context, *ChatRequest) (*ChatStream, error) {
	return nil, ErrNotSupported
}

type embeddingCapableProvider struct{ *capabilityBaseProvider }

func (*embeddingCapableProvider) CreateEmbeddings(context.Context, *EmbeddingRequest) (*EmbeddingResponse, error) {
	return &EmbeddingResponse{}, nil
}

type contextualizedEmbeddingCapableProvider struct{ *capabilityBaseProvider }

func (*contextualizedEmbeddingCapableProvider) CreateContextualizedEmbeddings(context.Context, *ContextualizedEmbeddingRequest) (*ContextualizedEmbeddingResponse, error) {
	return &ContextualizedEmbeddingResponse{}, nil
}

type rerankerCapableProvider struct{ *capabilityBaseProvider }

func (*rerankerCapableProvider) Rerank(context.Context, *RerankRequest) (*RerankResponse, error) {
	return &RerankResponse{}, nil
}

type imageCapableProvider struct{ *capabilityBaseProvider }

func (*imageCapableProvider) GenerateImage(context.Context, *ImageGenerateRequest) (*ImageResponse, error) {
	return &ImageResponse{}, nil
}
func (*imageCapableProvider) EditImage(context.Context, *ImageEditRequest) (*ImageResponse, error) {
	return &ImageResponse{}, nil
}
func (*imageCapableProvider) StreamImage(context.Context, *ImageGenerateRequest) (*ImageStream, error) {
	return &ImageStream{}, nil
}

type contentPartCapableProvider struct{ *capabilityBaseProvider }

func (*contentPartCapableProvider) SupportsContentPart(ModelID, Role, ContentPart) bool {
	return true
}

func TestAsEmbeddingProvider(t *testing.T) {
	provider := &embeddingCapableProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "embedding"}}
	got, ok := AsEmbeddingProvider(provider)
	if !ok || got != provider {
		t.Fatalf("AsEmbeddingProvider() = (%v, %v), want provider, true", got, ok)
	}

	got, ok = AsEmbeddingProvider(&capabilityBaseProvider{id: "base"})
	if ok || got != nil {
		t.Fatalf("AsEmbeddingProvider(non-capable) = (%v, %v), want nil, false", got, ok)
	}
}

func TestAsContextualizedEmbeddingProvider(t *testing.T) {
	provider := &contextualizedEmbeddingCapableProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "contextualized"}}
	got, ok := AsContextualizedEmbeddingProvider(provider)
	if !ok || got != provider {
		t.Fatalf("AsContextualizedEmbeddingProvider() = (%v, %v), want provider, true", got, ok)
	}

	got, ok = AsContextualizedEmbeddingProvider(&capabilityBaseProvider{id: "base"})
	if ok || got != nil {
		t.Fatalf("AsContextualizedEmbeddingProvider(non-capable) = (%v, %v), want nil, false", got, ok)
	}
}

func TestAsReranker(t *testing.T) {
	provider := &rerankerCapableProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "reranker"}}
	got, ok := AsReranker(provider)
	if !ok || got != provider {
		t.Fatalf("AsReranker() = (%v, %v), want provider, true", got, ok)
	}

	got, ok = AsReranker(&capabilityBaseProvider{id: "base"})
	if ok || got != nil {
		t.Fatalf("AsReranker(non-capable) = (%v, %v), want nil, false", got, ok)
	}
}

func TestAsImageGenerator(t *testing.T) {
	provider := &imageCapableProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "image"}}
	got, ok := AsImageGenerator(provider)
	if !ok || got != provider {
		t.Fatalf("AsImageGenerator() = (%v, %v), want provider, true", got, ok)
	}

	got, ok = AsImageGenerator(&capabilityBaseProvider{id: "base"})
	if ok || got != nil {
		t.Fatalf("AsImageGenerator(non-capable) = (%v, %v), want nil, false", got, ok)
	}
}

func TestAsContentPartSupporter(t *testing.T) {
	provider := &contentPartCapableProvider{capabilityBaseProvider: &capabilityBaseProvider{id: "content"}}
	got, ok := AsContentPartSupporter(provider)
	if !ok || got != provider {
		t.Fatalf("AsContentPartSupporter() = (%v, %v), want provider, true", got, ok)
	}

	got, ok = AsContentPartSupporter(&capabilityBaseProvider{id: "base"})
	if ok || got != nil {
		t.Fatalf("AsContentPartSupporter(non-capable) = (%v, %v), want nil, false", got, ok)
	}
}
