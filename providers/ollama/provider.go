package ollama

import (
	"context"
	"net/http"

	"github.com/petal-labs/iris/core"
)

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
