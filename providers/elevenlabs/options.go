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

// DefaultStreamChunkSize is the default maximum byte size per emitted audio
// chunk during HTTP streaming TTS.
const DefaultStreamChunkSize = 16 * 1024

// DefaultStreamBufferSize is the default bounded channel capacity for queued
// audio chunks during HTTP streaming TTS.
const DefaultStreamBufferSize = 16

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

	// StreamChunkSize sets the maximum read buffer size per AudioChunk during
	// HTTP streaming TTS. Defaults to DefaultStreamChunkSize (16 KiB).
	StreamChunkSize int `json:"stream_chunk_size,omitempty"`

	// StreamBufferSize sets the bounded channel capacity for AudioChunk values
	// during HTTP streaming TTS. Defaults to DefaultStreamBufferSize (16 chunks).
	StreamBufferSize int `json:"stream_buffer_size,omitempty"`

	// VoiceManagementEnabled explicitly opts this provider instance into mutating
	// voice operations (CreateVoice, EditVoice, EditVoiceSettings, DeleteVoice).
	// Defaults to false so voice APIs remain read-only unless explicitly enabled.
	VoiceManagementEnabled bool `json:"voice_management_enabled,omitempty"`

	// VoiceAuthorizer optionally enforces application-level RBAC, consent policy,
	// or audit checks before executing any mutating voice management operation.
	VoiceAuthorizer VoiceAuthorizer `json:"-"`

	// wsDialer overrides the default WebSocket dialer in tests.
	wsDialer wsDialer
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

// WithStreamChunkSize sets the maximum byte size per emitted [core.AudioChunk]
// for HTTP streaming TTS. Non-positive values are ignored.
func WithStreamChunkSize(bytes int) Option {
	return func(c *Config) {
		if bytes > 0 {
			c.StreamChunkSize = bytes
		}
	}
}

// WithStreamBufferSize sets the bounded channel capacity (in chunks) for
// [core.SpeechStream.Ch]. Non-positive values are ignored.
func WithStreamBufferSize(chunks int) Option {
	return func(c *Config) {
		if chunks > 0 {
			c.StreamBufferSize = chunks
		}
	}
}

// WithVoiceManagementEnabled explicitly opts this provider instance into
// mutating voice operations ([ElevenLabs.CreateVoice], [ElevenLabs.EditVoice],
// [ElevenLabs.EditVoiceSettings], and [ElevenLabs.DeleteVoice]).
//
// Without this option, all mutating voice methods fail fast with
// [ErrVoiceManagementDisabled] before making any HTTP requests.
func WithVoiceManagementEnabled() Option {
	return func(c *Config) {
		c.VoiceManagementEnabled = true
	}
}

// WithVoiceAuthorizer registers an authorization hook invoked before every
// mutating voice operation. Combine with [WithVoiceManagementEnabled] to
// enforce application-level RBAC, quota, or audit controls.
func WithVoiceAuthorizer(auth VoiceAuthorizer) Option {
	return func(c *Config) {
		c.VoiceAuthorizer = auth
	}
}
