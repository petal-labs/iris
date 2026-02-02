package anthropic

import (
	"net/http"
	"time"
)

// Config holds configuration for the Anthropic provider.
type Config struct {
	// APIKey is the Anthropic API key (required).
	APIKey string

	// BaseURL is the API base URL. Defaults to https://api.anthropic.com
	BaseURL string

	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Version is the Anthropic API version. Defaults to 2023-06-01.
	Version string

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout is the optional request timeout.
	Timeout time.Duration

	// FilesAPIBeta is the beta version for Files API. Defaults to DefaultFilesAPIBeta.
	FilesAPIBeta string
}

// DefaultBaseURL is the default Anthropic API base URL.
const DefaultBaseURL = "https://api.anthropic.com"

// DefaultVersion is the default Anthropic API version.
const DefaultVersion = "2023-06-01"

// DefaultFilesAPIBeta is the default beta version for the Files API.
const DefaultFilesAPIBeta = "files-api-2025-04-14"

// Option configures the Anthropic provider.
type Option func(*Config)

// WithBaseURL sets the API base URL.
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

// WithVersion sets the Anthropic API version.
func WithVersion(version string) Option {
	return func(c *Config) {
		c.Version = version
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

// WithFilesAPIBeta sets the beta version header for Files API operations.
func WithFilesAPIBeta(version string) Option {
	return func(c *Config) {
		c.FilesAPIBeta = version
	}
}
