package xai

import (
	"net/http"
	"time"

	"github.com/petal-labs/iris/core"
)

// Config holds configuration for the xAI provider.
type Config struct {
	// APIKey is the xAI API key (required).
	// Stored as Secret to prevent accidental logging.
	APIKey core.Secret

	// BaseURL is the API base URL. Defaults to https://api.x.ai/v1
	BaseURL string

	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout sets the timeout for this provider's direct (non-chat)
	// operations — embeddings, files, images, batch, and vector stores.
	// Chat and streaming honor core.WithTimeout and context deadlines.
	Timeout time.Duration
}

// DefaultBaseURL is the default xAI API base URL.
const DefaultBaseURL = "https://api.x.ai/v1"

// Option configures the xAI provider.
type Option func(*Config)

// WithAPIKey sets the API key, overriding any value passed to New.
func WithAPIKey(key string) Option {
	return func(c *Config) {
		c.APIKey = core.NewSecret(key)
	}
}

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

// WithTimeout sets the timeout for this provider's direct (non-chat)
// operations — embeddings, files, images, batch, and vector stores. Chat and
// streaming honor core.WithTimeout and context deadlines.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}
