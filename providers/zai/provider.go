package zai

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
)

// DefaultAPIKeyEnvVar is the environment variable name for the Z.ai API key.
const DefaultAPIKeyEnvVar = "ZAI_API_KEY"

// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
var ErrAPIKeyNotFound = errors.New("zai: ZAI_API_KEY environment variable not set")

// NewFromEnv creates a new Z.ai provider using the ZAI_API_KEY environment variable.
// This is a convenience factory for quick setup:
//
//	provider, err := zai.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(provider)
func NewFromEnv(opts ...Option) (*Zai, error) {
	apiKey := os.Getenv(DefaultAPIKeyEnvVar)
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	return New(apiKey, opts...), nil
}

// Zai is an LLM provider implementation for the Z.ai GLM API.
// Zai is safe for concurrent use.
type Zai struct {
	config Config
}

// New creates a new Z.ai provider with the given API key and options.
func New(apiKey string, opts ...Option) *Zai {
	cfg := Config{
		APIKey:     core.NewSecret(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Zai{config: cfg}
}

// ID returns the provider identifier.
func (p *Zai) ID() string {
	return "zai"
}

// Models returns the list of available models.
func (p *Zai) Models() []core.ModelInfo {
	// Return a copy to prevent mutation
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *Zai) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *Zai) buildHeaders() http.Header {
	headers := make(http.Header)

	// Required headers
	headers.Set("Authorization", "Bearer "+p.config.APIKey.Expose())
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept-Language", "en-US,en")

	// Copy any extra headers
	for key, values := range p.config.Headers {
		for _, v := range values {
			headers.Add(key, v)
		}
	}

	return headers
}

// Chat sends a non-streaming chat request.
func (p *Zai) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *Zai) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// Compile-time checks that Zai implements required interfaces.
var _ core.Provider = (*Zai)(nil)
