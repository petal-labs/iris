package ollama

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

// Environment variable names for Ollama configuration.
const (
	OllamaAPIKeyEnvVar = "OLLAMA_API_KEY"
	OllamaHostEnvVar   = "OLLAMA_HOST"
	// DefaultAPIKeyEnvVar is the canonical API-key environment variable alias.
	DefaultAPIKeyEnvVar = OllamaAPIKeyEnvVar
)

// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
var ErrAPIKeyNotFound = errors.New("ollama: OLLAMA_API_KEY environment variable not set")

// NewFromEnv creates an Ollama provider using OLLAMA_HOST and the optional
// OLLAMA_API_KEY. A missing API key is valid for local Ollama instances, so the
// current implementation returns a nil error; the error result preserves the
// common provider-factory shape. Explicit options override environment values.
func NewFromEnv(opts ...Option) (*Ollama, error) {
	envOpts := make([]Option, 0, len(opts)+2)
	host := os.Getenv(OllamaHostEnvVar)
	apiKey := os.Getenv(OllamaAPIKeyEnvVar)
	if host != "" {
		envOpts = append(envOpts, WithBaseURL(host))
	} else if apiKey != "" {
		envOpts = append(envOpts, WithCloud())
	}
	if apiKey != "" {
		envOpts = append(envOpts, WithAPIKey(apiKey))
	}
	envOpts = append(envOpts, opts...)
	return New(envOpts...), nil
}

// NewLocal creates a new Ollama provider for a local Ollama instance.
// This is a convenience factory for quick local setup:
//
//	provider := ollama.NewLocal()
//	client := core.NewClient(provider)
//
// If OLLAMA_HOST is set, it uses that URL; otherwise defaults to http://localhost:11434.
func NewLocal(opts ...Option) *Ollama {
	baseOpts := make([]Option, 0, len(opts)+1)

	// Check for custom host from environment
	if host := os.Getenv(OllamaHostEnvVar); host != "" {
		baseOpts = append(baseOpts, WithBaseURL(host))
	}

	baseOpts = append(baseOpts, opts...)
	return New(baseOpts...)
}

// NewCloudFromEnv creates a new Ollama provider for Ollama Cloud using the OLLAMA_API_KEY environment variable.
// This is a convenience factory for quick cloud setup:
//
//	provider, err := ollama.NewCloudFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(provider)
func NewCloudFromEnv(opts ...Option) (*Ollama, error) {
	apiKey := os.Getenv(OllamaAPIKeyEnvVar)
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	baseOpts := []Option{WithCloud(), WithAPIKey(apiKey)}
	baseOpts = append(baseOpts, opts...)
	return New(baseOpts...), nil
}

// Ollama is an LLM provider implementation for the Ollama API.
// Ollama is safe for concurrent use.
type Ollama struct {
	config Config
}

// New creates a new Ollama provider with the given options.
// For local Ollama instances, no API key is required.
// For Ollama Cloud, use WithCloud() and WithAPIKey().
func New(opts ...Option) *Ollama {
	cfg := Config{
		BaseURL:    DefaultLocalURL,
		HTTPClient: http.DefaultClient,
		Timeout:    timeoutx.Default,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Ollama{config: cfg}
}

// ID returns the provider identifier.
func (p *Ollama) ID() string {
	return "ollama"
}

// Models returns models installed on the configured Ollama instance. Because
// Provider.Models cannot return an error or accept a context, discovery is
// bounded to two seconds and falls back to an illustrative catalog when the
// instance cannot be reached. Use ListModels for error-aware discovery.
func (p *Ollama) Models() []core.ModelInfo {
	ctx, cancel := p.discoveryContext()
	defer cancel()
	models, err := p.ListModels(ctx)
	if err == nil {
		return models
	}
	return illustrativeModels()
}

// Supports reports whether the provider supports the given feature.
func (p *Ollama) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning, core.FeatureEmbeddings:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *Ollama) buildHeaders() http.Header {
	headers := make(http.Header)

	// Content type is always required
	headers.Set("Content-Type", "application/json")

	// Authorization header only if API key is provided (for Ollama Cloud)
	if !p.config.APIKey.IsEmpty() {
		headers.Set("Authorization", "Bearer "+p.config.APIKey.Expose())
	}

	// Copy any extra headers
	for key, values := range p.config.Headers {
		for _, v := range values {
			headers.Add(key, v)
		}
	}

	return headers
}

// Chat sends a non-streaming chat request.
func (p *Ollama) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *Ollama) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// Compile-time check that Ollama implements Provider.
var _ core.Provider = (*Ollama)(nil)
