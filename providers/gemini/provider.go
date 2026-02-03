package gemini

import (
	"context"
	"net/http"

	"github.com/petal-labs/iris/core"
)

// Gemini is an LLM provider implementation for the Google Gemini API.
// Gemini is safe for concurrent use.
type Gemini struct {
	config Config
}

// New creates a new Gemini provider with the given API key and options.
func New(apiKey string, opts ...Option) *Gemini {
	cfg := Config{
		APIKey:     core.NewSecret(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Gemini{config: cfg}
}

// ID returns the provider identifier.
func (p *Gemini) ID() string {
	return "gemini"
}

// Models returns the list of available models.
func (p *Gemini) Models() []core.ModelInfo {
	// Return a copy to prevent mutation
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *Gemini) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning, core.FeatureImageGeneration:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *Gemini) buildHeaders() http.Header {
	headers := make(http.Header)

	// Required headers for Gemini API
	headers.Set("x-goog-api-key", p.config.APIKey.Expose())
	headers.Set("Content-Type", "application/json")

	// Copy any extra headers
	for key, values := range p.config.Headers {
		for _, v := range values {
			headers.Add(key, v)
		}
	}

	return headers
}

// Chat sends a non-streaming chat request.
func (p *Gemini) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *Gemini) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// Compile-time check that Gemini implements Provider.
var _ core.Provider = (*Gemini)(nil)

// Compile-time check that Gemini implements ImageGenerator.
var _ core.ImageGenerator = (*Gemini)(nil)
