package elevenlabs

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
)

// DefaultBaseURL is the default ElevenLabs API base URL.
const DefaultBaseURL = "https://api.elevenlabs.io"

// DefaultAPIKeyEnvVar is the environment variable read by [NewFromEnv].
const DefaultAPIKeyEnvVar = "ELEVENLABS_API_KEY"

// ErrAPIKeyNotFound is returned by [NewFromEnv] when ELEVENLABS_API_KEY is unset or blank.
var ErrAPIKeyNotFound = errors.New("elevenlabs: ELEVENLABS_API_KEY environment variable not set")

// Config holds configuration for the ElevenLabs provider.
//
// Privacy: APIKey is stored as [core.Secret]. String and GoString emit a
// redacted summary that never includes the API key or custom header values.
type Config struct {
	// APIKey is the ElevenLabs API key sent via the xi-api-key header.
	// Stored as core.Secret to prevent accidental logging or JSON serialization.
	APIKey core.Secret `json:"api_key"`

	// BaseURL is the API base URL. Defaults to https://api.elevenlabs.io.
	BaseURL string `json:"base_url"`

	// HTTPClient is the HTTP client used for API requests. Defaults to http.DefaultClient.
	HTTPClient *http.Client `json:"-"`

	// Headers contains optional extra HTTP headers included on requests.
	Headers http.Header `json:"-"`

	// Timeout bounds direct provider unary requests when the caller's context
	// does not already supply a deadline. Zero or negative disables the default timeout.
	Timeout time.Duration `json:"timeout_ns,omitempty"`
}

// String returns a log-safe summary with the API key and custom header values redacted.
func (c Config) String() string {
	return fmt.Sprintf("Config{base_url=%q, api_key=%s, timeout=%s, extra_headers=%d}",
		c.BaseURL, c.APIKey.String(), c.Timeout, len(c.Headers))
}

// GoString returns a log-safe representation for %#v formatting.
func (c Config) GoString() string {
	return "elevenlabs." + c.String()
}

// Option configures an [ElevenLabs] provider instance.
type Option func(*Config)

// WithAPIKey sets the ElevenLabs API key, overriding any value passed to [New].
func WithAPIKey(key string) Option {
	return func(c *Config) {
		c.APIKey = core.NewSecret(key)
	}
}

// WithBaseURL sets a custom API base URL (for testing or proxy gateways).
// Trailing slashes are trimmed. Blank values leave the current BaseURL unchanged.
func WithBaseURL(rawURL string) Option {
	return func(c *Config) {
		trimmed := strings.TrimRight(strings.TrimSpace(rawURL), "/")
		if trimmed != "" {
			c.BaseURL = trimmed
		}
	}
}

// WithHTTPClient sets a custom HTTP client. A nil client is ignored so the
// provider always retains a non-nil client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		if client != nil {
			c.HTTPClient = client
		}
	}
}

// WithHeader sets an extra HTTP header to include on outgoing requests.
// Blank header names are ignored.
func WithHeader(key, value string) Option {
	return func(c *Config) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if c.Headers == nil {
			c.Headers = make(http.Header)
		}
		c.Headers.Set(key, value)
	}
}

// WithTimeout sets the per-provider timeout for unary API operations when the
// caller's context does not already have a deadline. Pass 0 to disable.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}
