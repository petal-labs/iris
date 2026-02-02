package huggingface

import (
	"net/http"
	"time"
)

// DefaultBaseURL is the default Hugging Face inference router URL.
const DefaultBaseURL = "https://router.huggingface.co"

// HubAPIBaseURL is the base URL for the Hugging Face Hub API.
const HubAPIBaseURL = "https://huggingface.co/api"

// Provider policy constants for routing requests.
const (
	// PolicyAuto uses HF's default provider selection based on user preferences.
	PolicyAuto = "auto"

	// PolicyFastest routes to the provider with highest throughput.
	PolicyFastest = "fastest"

	// PolicyCheapest routes to the provider with lowest cost per output token.
	PolicyCheapest = "cheapest"
)

// Config holds configuration for the Hugging Face provider.
type Config struct {
	// APIKey is the Hugging Face token (required).
	// Must have "Make calls to Inference Providers" permission.
	APIKey string

	// BaseURL is the inference API base URL.
	// Defaults to https://router.huggingface.co
	BaseURL string

	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout is the optional request timeout.
	Timeout time.Duration

	// ProviderPolicy controls how requests are routed to inference providers.
	// Valid values: "auto" (default), "fastest", "cheapest", or a specific
	// provider name (e.g., "cerebras", "together", "groq").
	// This can be overridden per-request by appending a suffix to the model name.
	ProviderPolicy string
}

// Option configures the Hugging Face provider.
type Option func(*Config)

// WithBaseURL sets the inference API base URL.
func WithBaseURL(url string) Option {
	return func(c *Config) {
		c.BaseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// WithHeader adds an extra header to include in requests.
func WithHeader(key, value string) Option {
	return func(c *Config) {
		if c.Headers == nil {
			c.Headers = make(http.Header)
		}
		c.Headers.Set(key, value)
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithProviderPolicy sets the default provider routing policy.
// Valid values: "auto", "fastest", "cheapest", or a specific provider name.
// This can be overridden per-request by appending a suffix to the model name
// (e.g., "meta-llama/Llama-3-8B-Instruct:fastest").
func WithProviderPolicy(policy string) Option {
	return func(c *Config) {
		c.ProviderPolicy = policy
	}
}
