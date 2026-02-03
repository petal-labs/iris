package huggingface

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
)

// Environment variable names for the Hugging Face API token.
const (
	HFTokenEnvVar          = "HF_TOKEN"
	HuggingFaceTokenEnvVar = "HUGGINGFACE_TOKEN"
)

// ErrAPIKeyNotFound is returned when no API token environment variable is set.
var ErrAPIKeyNotFound = errors.New("huggingface: HF_TOKEN or HUGGINGFACE_TOKEN environment variable not set")

// NewFromEnv creates a new Hugging Face provider using the HF_TOKEN or HUGGINGFACE_TOKEN environment variable.
// HF_TOKEN takes precedence if both are set.
//
// This is a convenience factory for quick setup:
//
//	provider, err := huggingface.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(provider)
func NewFromEnv(opts ...Option) (*HuggingFace, error) {
	apiKey := os.Getenv(HFTokenEnvVar)
	if apiKey == "" {
		apiKey = os.Getenv(HuggingFaceTokenEnvVar)
	}
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	return New(apiKey, opts...), nil
}

// HuggingFace is an LLM provider implementation for Hugging Face Inference Providers.
// HuggingFace is safe for concurrent use.
type HuggingFace struct {
	config Config
}

// New creates a new Hugging Face provider with the given API key and options.
// The API key should be a Hugging Face token with "Make calls to Inference Providers" permission.
func New(apiKey string, opts ...Option) *HuggingFace {
	cfg := Config{
		APIKey:        core.NewSecret(apiKey),
		BaseURL:       DefaultBaseURL,
		HubAPIBaseURL: HubAPIBaseURL,
		HTTPClient:    http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &HuggingFace{config: cfg}
}

// ID returns the provider identifier.
func (p *HuggingFace) ID() string {
	return "huggingface"
}

// Models returns an empty list since Hugging Face supports thousands of
// user-specified models. Use ListModels() to discover available models.
func (p *HuggingFace) Models() []core.ModelInfo {
	return []core.ModelInfo{}
}

// Supports reports whether the provider supports the given feature.
func (p *HuggingFace) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *HuggingFace) buildHeaders() http.Header {
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
func (p *HuggingFace) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *HuggingFace) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// Compile-time check that HuggingFace implements Provider.
var _ core.Provider = (*HuggingFace)(nil)
