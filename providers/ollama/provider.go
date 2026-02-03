package ollama

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
)

// Environment variable names for Ollama configuration.
const (
	OllamaAPIKeyEnvVar = "OLLAMA_API_KEY"
	OllamaHostEnvVar   = "OLLAMA_HOST"
)

// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
var ErrAPIKeyNotFound = errors.New("ollama: OLLAMA_API_KEY environment variable not set")

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

// Models returns example models available through Ollama.
// Note: Ollama models are dynamic - you can use any model you have pulled locally.
func (p *Ollama) Models() []core.ModelInfo {
	// Return common example models for documentation purposes
	// Users can use any model they have pulled
	return []core.ModelInfo{
		{ID: "llama3.2", DisplayName: "Llama 3.2", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "llama3.2:70b", DisplayName: "Llama 3.2 70B", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "mistral", DisplayName: "Mistral 7B", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "mixtral", DisplayName: "Mixtral 8x7B", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling}},
		{ID: "qwen3", DisplayName: "Qwen 3", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning}},
		{ID: "gemma3", DisplayName: "Gemma 3", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
		{ID: "deepseek-coder", DisplayName: "DeepSeek Coder", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
		{ID: "codellama", DisplayName: "Code Llama", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
	}
}

// Supports reports whether the provider supports the given feature.
func (p *Ollama) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureReasoning:
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
