package xai

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
)

// DefaultAPIKeyEnvVar is the environment variable name for the xAI API key.
const DefaultAPIKeyEnvVar = "XAI_API_KEY"

// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
var ErrAPIKeyNotFound = errors.New("xai: XAI_API_KEY environment variable not set")

// NewFromEnv creates a new xAI provider using the XAI_API_KEY environment variable.
// This is a convenience factory for quick setup:
//
//	provider, err := xai.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(provider)
func NewFromEnv(opts ...Option) (*Xai, error) {
	apiKey := os.Getenv(DefaultAPIKeyEnvVar)
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	return New(apiKey, opts...), nil
}

// Xai is an LLM provider implementation for the xAI Grok API.
// Xai is safe for concurrent use.
type Xai struct {
	config Config
}

// New creates a new xAI provider with the given API key and options.
func New(apiKey string, opts ...Option) *Xai {
	cfg := Config{
		APIKey:     core.NewSecret(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Xai{config: cfg}
}

// ID returns the provider identifier.
func (p *Xai) ID() string {
	return "xai"
}

// Models returns the list of available models.
func (p *Xai) Models() []core.ModelInfo {
	// Return a copy to prevent mutation
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *Xai) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *Xai) buildHeaders() http.Header {
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

// Chat sends a non-streaming chat request.
func (p *Xai) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *Xai) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// Compile-time check that Xai implements Provider.
var _ core.Provider = (*Xai)(nil)
