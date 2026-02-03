package openai

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/petal-labs/iris/core"
)

// DefaultAPIKeyEnvVar is the environment variable name for the OpenAI API key.
const DefaultAPIKeyEnvVar = "OPENAI_API_KEY"

// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
var ErrAPIKeyNotFound = errors.New("openai: OPENAI_API_KEY environment variable not set")

// NewFromEnv creates a new OpenAI provider using the OPENAI_API_KEY environment variable.
// This is a convenience factory for quick setup:
//
//	provider, err := openai.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(provider)
//
// Additional options can be passed to customize the provider:
//
//	provider, err := openai.NewFromEnv(openai.WithOrgID("org-xxx"))
func NewFromEnv(opts ...Option) (*OpenAI, error) {
	apiKey := os.Getenv(DefaultAPIKeyEnvVar)
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	return New(apiKey, opts...), nil
}

// OpenAI is an LLM provider implementation for the OpenAI API.
// OpenAI is safe for concurrent use.
type OpenAI struct {
	config Config
}

// New creates a new OpenAI provider with the given API key and options.
func New(apiKey string, opts ...Option) *OpenAI {
	cfg := Config{
		APIKey:     core.NewSecret(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &OpenAI{config: cfg}
}

// ID returns the provider identifier.
func (p *OpenAI) ID() string {
	return "openai"
}

// Models returns the list of available models.
func (p *OpenAI) Models() []core.ModelInfo {
	// Return a copy to prevent mutation
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *OpenAI) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling, core.FeatureImageGeneration, core.FeatureEmbeddings:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *OpenAI) buildHeaders() http.Header {
	headers := make(http.Header)

	// Required headers
	headers.Set("Authorization", "Bearer "+p.config.APIKey.Expose())
	headers.Set("Content-Type", "application/json")

	// Optional organization header
	if p.config.OrgID != "" {
		headers.Set("OpenAI-Organization", p.config.OrgID)
	}

	// Optional project header
	if p.config.ProjectID != "" {
		headers.Set("OpenAI-Project", p.config.ProjectID)
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
// Routes to either the Chat Completions API or Responses API based on the model.
func (p *OpenAI) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if p.shouldUseResponsesAPI(req.Model) {
		return p.doResponsesChat(ctx, req)
	}
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
// Routes to either the Chat Completions API or Responses API based on the model.
func (p *OpenAI) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	if p.shouldUseResponsesAPI(req.Model) {
		return p.doResponsesStreamChat(ctx, req)
	}
	return p.doStreamChat(ctx, req)
}

// shouldUseResponsesAPI determines if a model should use the Responses API.
// Returns true for models that declare APIEndpointResponses, false otherwise.
// Unknown models default to the Chat Completions API for backward compatibility.
func (p *OpenAI) shouldUseResponsesAPI(model core.ModelID) bool {
	info := GetModelInfo(model)
	if info == nil {
		// Unknown model - default to completions for backward compatibility
		return false
	}
	return info.GetAPIEndpoint() == core.APIEndpointResponses
}

// Compile-time check that OpenAI implements Provider.
var _ core.Provider = (*OpenAI)(nil)

// Compile-time check that OpenAI implements ImageGenerator.
var _ core.ImageGenerator = (*OpenAI)(nil)

// Compile-time check that OpenAI implements EmbeddingProvider.
var _ core.EmbeddingProvider = (*OpenAI)(nil)
