package azurefoundry

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/petal-labs/iris/core"
)

// Environment variable names for configuration.
const (
	// EnvEndpoint is the environment variable for the Azure endpoint.
	EnvEndpoint = "AZURE_AI_ENDPOINT"
	// EnvAPIKey is the environment variable for the Azure API key.
	EnvAPIKey = "AZURE_AI_API_KEY"
	// EnvDeploymentID is the optional environment variable for deployment ID.
	EnvDeploymentID = "AZURE_AI_DEPLOYMENT_ID"
)

// Sentinel errors.
var (
	// ErrEndpointNotFound is returned when the endpoint environment variable is not set.
	ErrEndpointNotFound = errors.New("azurefoundry: AZURE_AI_ENDPOINT environment variable not set")
	// ErrAPIKeyNotFound is returned when the API key environment variable is not set.
	ErrAPIKeyNotFound = errors.New("azurefoundry: AZURE_AI_API_KEY environment variable not set")
	// ErrNoAuth is returned when neither API key nor token credential is configured.
	ErrNoAuth = errors.New("azurefoundry: no authentication configured (provide API key or token credential)")
	// ErrDeploymentRequired is returned when deployment ID is required but not set.
	ErrDeploymentRequired = errors.New("azurefoundry: deployment ID required when using OpenAI endpoint format")
)

// AzureFoundry is an LLM provider implementation for Azure AI Foundry.
// AzureFoundry is safe for concurrent use.
type AzureFoundry struct {
	config     Config
	tokenCache *tokenCache
}

// New creates a new Azure AI Foundry provider with API key authentication.
//
// Example:
//
//	provider := azurefoundry.New(
//	    "https://my-resource.services.ai.azure.com",
//	    "my-api-key",
//	)
//	client := core.NewClient(provider)
func New(endpoint, apiKey string, opts ...Option) *AzureFoundry {
	cfg := Config{
		Endpoint:   normalizeEndpoint(endpoint),
		APIKey:     core.NewSecret(apiKey),
		APIVersion: DefaultAPIVersion,
		HTTPClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Adjust default API version for OpenAI endpoint format
	if cfg.UseOpenAIEndpoint && cfg.APIVersion == DefaultAPIVersion {
		cfg.APIVersion = DefaultOpenAIAPIVersion
	}

	return &AzureFoundry{config: cfg}
}

// NewFromEnv creates a provider using environment variables:
//   - AZURE_AI_ENDPOINT: The Azure resource endpoint (required)
//   - AZURE_AI_API_KEY: The API key (required)
//   - AZURE_AI_DEPLOYMENT_ID: The deployment ID (optional)
//
// Example:
//
//	provider, err := azurefoundry.NewFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewFromEnv(opts ...Option) (*AzureFoundry, error) {
	endpoint := os.Getenv(EnvEndpoint)
	if endpoint == "" {
		return nil, ErrEndpointNotFound
	}

	apiKey := os.Getenv(EnvAPIKey)
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}

	// Check for optional deployment ID
	if deploymentID := os.Getenv(EnvDeploymentID); deploymentID != "" {
		opts = append([]Option{WithDeploymentID(deploymentID)}, opts...)
	}

	return New(endpoint, apiKey, opts...), nil
}

// NewWithCredential creates a provider with Entra ID (Azure AD) authentication.
// The credential must implement the TokenCredential interface, which is compatible
// with azure-sdk-for-go's azcore.TokenCredential.
//
// Example with azure-identity:
//
//	cred, err := azidentity.NewDefaultAzureCredential(nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	provider := azurefoundry.NewWithCredential(
//	    "https://my-resource.services.ai.azure.com",
//	    cred,
//	)
func NewWithCredential(endpoint string, credential TokenCredential, opts ...Option) *AzureFoundry {
	cfg := Config{
		Endpoint:        normalizeEndpoint(endpoint),
		TokenCredential: credential,
		APIVersion:      DefaultAPIVersion,
		HTTPClient:      http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Adjust default API version for OpenAI endpoint format
	if cfg.UseOpenAIEndpoint && cfg.APIVersion == DefaultAPIVersion {
		cfg.APIVersion = DefaultOpenAIAPIVersion
	}

	p := &AzureFoundry{config: cfg}
	if credential != nil {
		p.tokenCache = newTokenCache(credential)
	}

	return p
}

// ID returns the provider identifier.
func (p *AzureFoundry) ID() string {
	return "azurefoundry"
}

// Models returns the list of available models.
// Azure AI Foundry supports various models depending on your deployment.
// This returns common models; actual availability depends on your Azure configuration.
func (p *AzureFoundry) Models() []core.ModelInfo {
	result := make([]core.ModelInfo, len(models))
	copy(result, models)
	return result
}

// Supports reports whether the provider supports the given feature.
func (p *AzureFoundry) Supports(feature core.Feature) bool {
	switch feature {
	case core.FeatureChat, core.FeatureChatStreaming, core.FeatureToolCalling,
		core.FeatureEmbeddings, core.FeatureStructuredOutput, core.FeatureReasoning:
		return true
	default:
		return false
	}
}

// buildHeaders constructs the HTTP headers for an API request.
func (p *AzureFoundry) buildHeaders(ctx context.Context) (http.Header, error) {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	// Authentication: prefer token credential over API key
	if p.tokenCache != nil {
		token, err := p.tokenCache.getToken(ctx)
		if err != nil {
			return nil, err
		}
		headers.Set("Authorization", "Bearer "+token)
	} else if !p.config.APIKey.IsEmpty() {
		headers.Set("api-key", p.config.APIKey.Expose())
	} else {
		return nil, ErrNoAuth
	}

	// Copy any extra headers
	for key, values := range p.config.Headers {
		for _, v := range values {
			headers.Add(key, v)
		}
	}

	return headers, nil
}

// buildChatURL constructs the URL for chat completions.
func (p *AzureFoundry) buildChatURL(model core.ModelID) (string, error) {
	if p.config.UseOpenAIEndpoint {
		// Azure OpenAI format: /openai/deployments/{deployment-id}/chat/completions
		deploymentID := p.config.DeploymentID
		if deploymentID == "" {
			deploymentID = string(model)
		}
		if deploymentID == "" {
			return "", ErrDeploymentRequired
		}
		return p.config.Endpoint + "/openai/deployments/" + deploymentID +
			"/chat/completions?api-version=" + p.config.APIVersion, nil
	}

	// Model Inference API format: /models/chat/completions
	return p.config.Endpoint + "/models/chat/completions?api-version=" + p.config.APIVersion, nil
}

// Chat sends a non-streaming chat request.
func (p *AzureFoundry) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.doChat(ctx, req)
}

// StreamChat sends a streaming chat request.
func (p *AzureFoundry) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return p.doStreamChat(ctx, req)
}

// normalizeEndpoint removes trailing slashes from the endpoint URL.
func normalizeEndpoint(endpoint string) string {
	return strings.TrimRight(endpoint, "/")
}

// Compile-time check that AzureFoundry implements Provider.
var _ core.Provider = (*AzureFoundry)(nil)
