package azurefoundry

import (
	"net/http"
	"time"

	"github.com/petal-labs/iris/core"
)

// Config holds configuration for the Azure AI Foundry provider.
type Config struct {
	// APIKey is the Azure API key (required if not using Entra ID).
	// Stored as Secret to prevent accidental logging.
	APIKey core.Secret

	// Endpoint is the Azure resource endpoint (required).
	// Format: https://{resource}.services.ai.azure.com or
	//         https://{resource}.openai.azure.com
	Endpoint string

	// DeploymentID is the model deployment name.
	// Required when using UseOpenAIEndpoint, optional for Model Inference API.
	DeploymentID string

	// APIVersion is the API version string.
	// Defaults to DefaultAPIVersion.
	APIVersion string

	// HTTPClient is the HTTP client to use.
	// Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// TokenCredential provides Entra ID tokens (alternative to APIKey).
	// When set, APIKey is ignored and Bearer token auth is used.
	TokenCredential TokenCredential

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout is the optional request timeout.
	// If set, requests will be cancelled after this duration.
	Timeout time.Duration

	// UseOpenAIEndpoint uses the Azure OpenAI endpoint format instead of
	// the Model Inference API:
	//   /openai/deployments/{deployment-id}/chat/completions
	// instead of:
	//   /models/chat/completions
	UseOpenAIEndpoint bool
}

// DefaultAPIVersion is the default API version for the Model Inference API.
const DefaultAPIVersion = "2024-05-01-preview"

// DefaultOpenAIAPIVersion is the default API version for Azure OpenAI endpoints.
const DefaultOpenAIAPIVersion = "2024-10-21"

// Option configures the Azure AI Foundry provider.
type Option func(*Config)

// WithAPIVersion sets the API version.
// Defaults to "2024-05-01-preview" for Model Inference API
// or "2024-10-21" for Azure OpenAI endpoints.
func WithAPIVersion(version string) Option {
	return func(c *Config) {
		c.APIVersion = version
	}
}

// WithDeploymentID sets a default deployment ID for requests.
// Required when using WithOpenAIEndpoint(), optional otherwise.
// The deployment ID corresponds to your model deployment name in Azure.
func WithDeploymentID(id string) Option {
	return func(c *Config) {
		c.DeploymentID = id
	}
}

// WithHTTPClient sets a custom HTTP client.
// Use this to configure custom timeouts, transport settings, or proxies.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// WithHeader adds an extra header to include in all requests.
// Can be called multiple times to add multiple headers.
func WithHeader(key, value string) Option {
	return func(c *Config) {
		if c.Headers == nil {
			c.Headers = make(http.Header)
		}
		c.Headers.Set(key, value)
	}
}

// WithTimeout sets the request timeout.
// Requests will be cancelled after this duration.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithOpenAIEndpoint configures the provider to use the Azure OpenAI
// endpoint format instead of the Model Inference API.
//
// Azure OpenAI endpoint format:
//
//	POST https://{resource}.openai.azure.com/openai/deployments/{deployment-id}/chat/completions?api-version=2024-10-21
//
// Model Inference API format (default):
//
//	POST https://{resource}.services.ai.azure.com/models/chat/completions?api-version=2024-05-01-preview
//
// Use this option when you need Azure OpenAI-specific features or when
// your deployment is configured through the Azure OpenAI Service.
func WithOpenAIEndpoint() Option {
	return func(c *Config) {
		c.UseOpenAIEndpoint = true
	}
}

// WithTokenCredential sets a token credential for Entra ID authentication.
// When set, the APIKey is ignored and Bearer token authentication is used.
//
// Example with azure-identity:
//
//	cred, _ := azidentity.NewDefaultAzureCredential(nil)
//	provider := azurefoundry.New(endpoint, "", azurefoundry.WithTokenCredential(cred))
func WithTokenCredential(credential TokenCredential) Option {
	return func(c *Config) {
		c.TokenCredential = credential
	}
}
