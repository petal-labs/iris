package openai

import (
	"net/http"
	"time"

	"github.com/petal-labs/iris/core"
)

// Config holds configuration for the OpenAI provider.
type Config struct {
	// APIKey is the OpenAI API key (required).
	// Stored as Secret to prevent accidental logging.
	APIKey core.Secret

	// BaseURL is the API base URL. Defaults to https://api.openai.com/v1
	BaseURL string

	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// OrgID is the optional OpenAI organization ID.
	OrgID string

	// ProjectID is the optional OpenAI project ID.
	ProjectID string

	// Headers contains optional extra headers to include in requests.
	Headers http.Header

	// Timeout sets the timeout for this provider's direct (non-chat)
	// operations — embeddings, files, images, batch, and vector stores.
	// Chat and streaming honor core.WithTimeout and context deadlines.
	Timeout time.Duration
}

// DefaultBaseURL is the default OpenAI API base URL.
const DefaultBaseURL = "https://api.openai.com/v1"

// Option configures the OpenAI provider.
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

// WithOrgID sets the OpenAI organization ID header.
func WithOrgID(org string) Option {
	return func(c *Config) {
		c.OrgID = org
	}
}

// WithProjectID sets the OpenAI project ID header.
func WithProjectID(project string) Option {
	return func(c *Config) {
		c.ProjectID = project
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
