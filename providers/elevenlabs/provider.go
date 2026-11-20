package elevenlabs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

// ElevenLabs is a speech-focused Iris provider for the ElevenLabs API.
//
// ElevenLabs is immutable after construction and safe for concurrent use
// across goroutines.
type ElevenLabs struct {
	config Config
}

// New creates a new ElevenLabs provider with the given API key and options.
func New(apiKey string, opts ...Option) *ElevenLabs {
	cfg := Config{
		APIKey:     core.NewSecret(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
		Timeout:    timeoutx.Default,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Headers != nil {
		cfg.Headers = cfg.Headers.Clone()
	}

	return &ElevenLabs{config: cfg}
}

// NewFromEnv creates a new ElevenLabs provider using the ELEVENLABS_API_KEY
// environment variable. Returns [ErrAPIKeyNotFound] when the variable is unset
// or whitespace-only.
func NewFromEnv(opts ...Option) (*ElevenLabs, error) {
	apiKey := strings.TrimSpace(os.Getenv(DefaultAPIKeyEnvVar))
	if apiKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	return New(apiKey, opts...), nil
}

// ID returns the provider identifier ("elevenlabs").
func (p *ElevenLabs) ID() string {
	return providerID
}

// Models returns a defensive copy of the ElevenLabs model catalog.
func (p *ElevenLabs) Models() []core.ModelInfo {
	result := make([]core.ModelInfo, len(models))
	for i, m := range models {
		result[i] = cloneModelInfo(m)
	}
	return result
}

// Supports reports whether the provider currently supports the given feature.
func (p *ElevenLabs) Supports(feature core.Feature) bool {
	return feature == core.FeatureSpeechSynthesis
}

// Chat is not supported by ElevenLabs. Returns [*core.ProviderError] wrapping
// [core.ErrNotSupported].
func (p *ElevenLabs) Chat(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return nil, &core.ProviderError{
		Provider: providerID,
		Message:  "chat is not supported by ElevenLabs",
		Err:      core.ErrNotSupported,
	}
}

// StreamChat is not supported by ElevenLabs. Returns [*core.ProviderError]
// wrapping [core.ErrNotSupported].
func (p *ElevenLabs) StreamChat(ctx context.Context, req *core.ChatRequest) (*core.ChatStream, error) {
	return nil, &core.ProviderError{
		Provider: providerID,
		Message:  "streaming chat is not supported by ElevenLabs",
		Err:      core.ErrNotSupported,
	}
}

// String returns a log-safe summary with credentials redacted.
func (p *ElevenLabs) String() string {
	if p == nil {
		return "ElevenLabs{<nil>}"
	}
	return fmt.Sprintf("ElevenLabs{%s}", p.config.String())
}

// GoString returns a log-safe representation for %#v formatting.
func (p *ElevenLabs) GoString() string {
	if p == nil {
		return "(*elevenlabs.ElevenLabs)(nil)"
	}
	return "elevenlabs." + p.String()
}

// Compile-time check that ElevenLabs implements core.Provider.
var _ core.Provider = (*ElevenLabs)(nil)
