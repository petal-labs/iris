package anthropic

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
)

// DefaultAPIKeyEnvVar is the environment variable name for the Anthropic API key.
const DefaultAPIKeyEnvVar = "ANTHROPIC_API_KEY"

// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
var ErrAPIKeyNotFound = errors.New("anthropic: ANTHROPIC_API_KEY environment variable not set")

// NewFromEnv creates a new Anthropic provider using the ANTHROPIC_API_KEY environment variable.
// This is a convenience factory for quick setup:
//
//	provider, err := anthropic.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(provider)
//
// Additional options can be passed to customize the provider.
func NewFromEnv(opts ...Option) (*Anthropic, error) {
	apiKey := os.Getenv(DefaultAPIKeyEnvVar)
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	return New(apiKey, opts...), nil
}

// Anthropic is an LLM provider implementation for the Anthropic API.
// Anthropic is safe for concurrent use.
type Anthropic struct {
	config Config
}

// New creates a new Anthropic provider with the given API key and options.
func New(apiKey string, opts ...Option) *Anthropic {
	cfg := Config{
		APIKey:       core.NewSecret(apiKey),
		BaseURL:      DefaultBaseURL,
		HTTPClient:   http.DefaultClient,
		Version:      DefaultVersion,
		FilesAPIBeta: DefaultFilesAPIBeta,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Anthropic{config: cfg}
}

// ID returns the provider identifier.
func (p *Anthropic) ID() string {
	return "anthropic"
}

// Models returns the list of available models.
func (p *Anthropic) Models() []core.ModelInfo {
	// Return a copy to prevent mutation
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *Anthropic) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *Anthropic) buildHeaders() http.Header {
	headers := make(http.Header)

	// Required headers for Anthropic API
	headers.Set("x-api-key", p.config.APIKey.Expose())
	headers.Set("anthropic-version", p.config.Version)
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
func (p *Anthropic) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *Anthropic) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// Compile-time check that Anthropic implements Provider.
var _ core.Provider = (*Anthropic)(nil)
