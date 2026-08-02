// Package zai provides a Z.ai GLM API provider implementation for Iris.
package zai

import (
	"net/http"
	"time"

	"github.com/petal-labs/iris/core"
)

// DefaultBaseURL is the default base URL for the Z.ai API (coding endpoint).
const DefaultBaseURL = "https://api.z.ai/api/coding/paas/v4"

// Config holds the configuration for the Z.ai provider.
type Config struct {
	// APIKey is the API key for authentication.
	// Stored as Secret to prevent accidental logging.
	APIKey core.Secret

	// BaseURL is the base URL for the API. Defaults to DefaultBaseURL.
	BaseURL string

	// HTTPClient is the HTTP client to use for requests.
	HTTPClient *http.Client

	// Headers are additional headers to include in requests.
	Headers http.Header

	// Timeout is retained for compatibility but is not applied by this provider.
	//
	// Deprecated: use core.WithTimeout on the Client instead.
	Timeout time.Duration
}

// Option is a functional option for configuring the Z.ai provider.
type Option func(*Config)

// WithAPIKey sets the API key, overriding any value passed to New.
func WithAPIKey(key string) Option {
	return func(c *Config) {
		c.APIKey = core.NewSecret(key)
	}
}

// WithBaseURL sets the base URL for the API.
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

// WithHeaders sets additional headers to include in requests.
func WithHeaders(headers http.Header) Option {
	return func(c *Config) {
		c.Headers = headers
	}
}

// WithTimeout sets the request timeout.
//
// Deprecated: this option is currently inert for this provider and has no
// effect on requests. Use core.WithTimeout on the Client (or a context
// deadline) to bound execution. Retained for API compatibility.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}
