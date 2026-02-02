package ollama

import (
	"net/http"
	"time"
)

// Default base URLs for Ollama API.
const (
	// DefaultLocalURL is the default URL for local Ollama instances.
	DefaultLocalURL = "http://localhost:11434"

	// DefaultCloudURL is the URL for Ollama Cloud (ollama.com).
	DefaultCloudURL = "https://ollama.com/api"
)

// Config holds the configuration for the Ollama provider.
type Config struct {
	// APIKey is the API key for Ollama Cloud. Optional for local instances.
	APIKey string

	// BaseURL is the base URL for the Ollama API.
	// Defaults to DefaultLocalURL.
	BaseURL string

	// HTTPClient is the HTTP client to use for requests.
	// Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Headers contains additional HTTP headers to include in requests.
	Headers http.Header

	// Timeout is the request timeout. Zero means no timeout.
	Timeout time.Duration
}

// Option is a function that configures the Ollama provider.
type Option func(*Config)

// WithAPIKey sets the API key for Ollama Cloud.
// This is optional for local Ollama instances.
func WithAPIKey(key string) Option {
	return func(c *Config) {
		c.APIKey = key
	}
}

// WithBaseURL sets a custom base URL for the Ollama API.
func WithBaseURL(url string) Option {
	return func(c *Config) {
		c.BaseURL = url
	}
}

// WithCloud configures the provider for Ollama Cloud (ollama.com).
// This sets the base URL to DefaultCloudURL. You should also call
// WithAPIKey to provide authentication.
func WithCloud() Option {
	return func(c *Config) {
		c.BaseURL = DefaultCloudURL
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// WithHeaders sets additional HTTP headers to include in requests.
func WithHeaders(headers http.Header) Option {
	return func(c *Config) {
		c.Headers = headers
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}
