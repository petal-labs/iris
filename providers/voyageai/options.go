package voyageai

import (
	"net/http"
	"time"
)

// Config holds configuration for the Voyage AI provider.
type Config struct {
	// APIKey is the Voyage AI API key (required).
	APIKey string

	// BaseURL is the API base URL. Defaults to https://api.voyageai.com/v1
	BaseURL string

	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout is the optional request timeout.
	Timeout time.Duration
}

// DefaultBaseURL is the default Voyage AI API base URL.
const DefaultBaseURL = "https://api.voyageai.com/v1"

// Option configures the Voyage AI provider.
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
