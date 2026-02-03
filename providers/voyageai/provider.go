package voyageai

import (
	"context"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// VoyageAI is an embedding and reranking provider implementation for the Voyage AI API.
// VoyageAI is safe for concurrent use.
type VoyageAI struct {
	config Config
}

// New creates a new Voyage AI provider with the given API key and options.
func New(apiKey string, opts ...Option) *VoyageAI {
	cfg := Config{
		APIKey:     core.NewSecret(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &VoyageAI{config: cfg}
}

// ID returns the provider identifier.
func (p *VoyageAI) ID() string {
	return "voyageai"
}

// Models returns the list of available models.
func (p *VoyageAI) Models() []core.ModelInfo {
	// Return a copy to prevent mutation
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *VoyageAI) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureEmbeddings, core.FeatureContextualizedEmbeddings, core.FeatureReranking:
		return true
	default:
		return false
	}
}

// Chat is not supported by Voyage AI. Returns ErrNotSupported.
func (p *VoyageAI) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return nil, &core.ProviderError{
		Provider: "voyageai",
		Message:  "chat is not supported by Voyage AI",
		Err:      core.ErrNotSupported,
	}
}

// StreamChat is not supported by Voyage AI. Returns ErrNotSupported.
func (p *VoyageAI) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return nil, &core.ProviderError{
		Provider: "voyageai",
		Message:  "streaming chat is not supported by Voyage AI",
		Err:      core.ErrNotSupported,
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *VoyageAI) buildHeaders() http.Header {
	headers := make(http.Header)

	// Required headers
	headers.Set("Authorization", "Bearer "+p.config.APIKey.Expose())
	headers.Set("Content-Type", "application/json")

	// Copy any extra headers
	for key, values := range p.config.Headers {
		for _, v := range values {
			headers.Add(key, v)
		}
	}

	return headers
}

// Compile-time check that VoyageAI implements Provider.
var _ core.Provider = (*VoyageAI)(nil)
