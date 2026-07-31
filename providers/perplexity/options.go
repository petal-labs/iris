package perplexity

import (
	"net/http"
	"time"

	"github.com/petal-labs/iris/core"
)

// Config holds configuration for the Perplexity provider.
type Config struct {
	// APIKey is the Perplexity API key (required).
	// Stored as Secret to prevent accidental logging.
	APIKey core.Secret

	// BaseURL is the API base URL. Defaults to https://api.perplexity.ai
	BaseURL string

	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout is retained for compatibility but is not applied by this provider.
	//
	// Deprecated: use core.WithTimeout on the Client instead.
	Timeout time.Duration
}

// DefaultBaseURL is the default Perplexity API base URL.
const DefaultBaseURL = "https://api.perplexity.ai"

// Option configures the Perplexity provider.
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
//
// Deprecated: this option is currently inert for this provider and has no
// effect on requests. Use core.WithTimeout on the Client (or a context
// deadline) to bound execution. Retained for API compatibility.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}
