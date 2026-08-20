package commands

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/azurefoundry"
	"github.com/petal-labs/iris/providers/gemini"
	"github.com/petal-labs/iris/providers/huggingface"
	"github.com/petal-labs/iris/providers/ollama"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/perplexity"
	"github.com/petal-labs/iris/providers/voyageai"
	"github.com/petal-labs/iris/providers/xai"
	"github.com/petal-labs/iris/providers/zai"
)

// keylessProviders lists providers that can operate without an API key
// (local inference). For these, a missing keystore entry is not an error:
// the provider is constructed with an empty key, and its own behavior
// decides what happens (local Ollama works; Ollama Cloud fails at request
// time with the provider's unauthorized error).
var keylessProviders = map[string]bool{
	"ollama": true,
}

// providerAllowsEmptyAPIKey reports whether providerID can be used without
// an API key stored in the keystore.
func providerAllowsEmptyAPIKey(providerID string) bool {
	return keylessProviders[providerID]
}

func defaultProviderFactory() ProviderFactory {
	return func(providerID, apiKey string, cfg *config.Config) (core.Provider, error) {
		provider, handled, err := createBuiltInProvider(providerID, apiKey, providerConfig(cfg, providerID))
		if handled {
			return provider, err
		}

		// Fall back to registry for externally-registered providers.
		if providers.IsRegistered(providerID) {
			return providers.Create(providerID, apiKey)
		}

		return nil, fmt.Errorf("unsupported provider: %s (available: %v)", providerID, providers.List())
	}
}

func providerConfig(cfg *config.Config, providerID string) config.ProviderConfig {
	if cfg == nil {
		return config.ProviderConfig{}
	}
	pc := cfg.GetProvider(providerID)
	if pc == nil {
		return config.ProviderConfig{}
	}
	return *pc
}

func createBuiltInProvider(providerID, apiKey string, cfg config.ProviderConfig) (core.Provider, bool, error) {
	switch providerID {
	case "openai":
		return newOpenAIProvider(apiKey, cfg.BaseURL), true, nil
	case "anthropic":
		return newAnthropicProvider(apiKey, cfg.BaseURL), true, nil
	case "gemini":
		return newGeminiProvider(apiKey, cfg.BaseURL), true, nil
	case "xai":
		return newXAIProvider(apiKey, cfg.BaseURL), true, nil
	case "zai":
		return newZAIProvider(apiKey, cfg.BaseURL), true, nil
	case "ollama":
		return newOllamaProvider(apiKey, cfg.BaseURL), true, nil
	case "huggingface":
		return newHuggingFaceProvider(apiKey, cfg.BaseURL), true, nil
	case "perplexity":
		return newPerplexityProvider(apiKey, cfg.BaseURL), true, nil
	case "voyageai":
		return newVoyageAIProvider(apiKey, cfg.BaseURL), true, nil
	case "azurefoundry":
		provider, err := newAzureFoundryProvider(apiKey, cfg)
		return provider, true, err
	default:
		return nil, false, nil
	}
}

func newOpenAIProvider(apiKey, baseURL string) core.Provider {
	var opts []openai.Option
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(apiKey, opts...)
}

func newAnthropicProvider(apiKey, baseURL string) core.Provider {
	var opts []anthropic.Option
	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}
	return anthropic.New(apiKey, opts...)
}

func newGeminiProvider(apiKey, baseURL string) core.Provider {
	var opts []gemini.Option
	if baseURL != "" {
		opts = append(opts, gemini.WithBaseURL(baseURL))
	}
	return gemini.New(apiKey, opts...)
}

func newXAIProvider(apiKey, baseURL string) core.Provider {
	var opts []xai.Option
	if baseURL != "" {
		opts = append(opts, xai.WithBaseURL(baseURL))
	}
	return xai.New(apiKey, opts...)
}

func newZAIProvider(apiKey, baseURL string) core.Provider {
	var opts []zai.Option
	if baseURL != "" {
		opts = append(opts, zai.WithBaseURL(baseURL))
	}
	return zai.New(apiKey, opts...)
}

func newOllamaProvider(apiKey, baseURL string) core.Provider {
	var opts []ollama.Option
	if baseURL != "" {
		opts = append(opts, ollama.WithBaseURL(baseURL))
	}
	if apiKey != "" {
		opts = append(opts, ollama.WithAPIKey(apiKey))
	}
	return ollama.New(opts...)
}

func newHuggingFaceProvider(apiKey, baseURL string) core.Provider {
	var opts []huggingface.Option
	if baseURL != "" {
		opts = append(opts, huggingface.WithBaseURL(baseURL))
	}
	return huggingface.New(apiKey, opts...)
}

func newPerplexityProvider(apiKey, baseURL string) core.Provider {
	var opts []perplexity.Option
	if baseURL != "" {
		opts = append(opts, perplexity.WithBaseURL(baseURL))
	}
	return perplexity.New(apiKey, opts...)
}

func newVoyageAIProvider(apiKey, baseURL string) core.Provider {
	var opts []voyageai.Option
	if baseURL != "" {
		opts = append(opts, voyageai.WithBaseURL(baseURL))
	}
	return voyageai.New(apiKey, opts...)
}

func newAzureFoundryProvider(apiKey string, cfg config.ProviderConfig) (core.Provider, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(cfg.BaseURL)
	}
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv(azurefoundry.EnvEndpoint))
	}
	if endpoint == "" {
		return nil, fmt.Errorf("azurefoundry endpoint required: set providers.azurefoundry.endpoint or %s", azurefoundry.EnvEndpoint)
	}
	cfg.DeploymentID = strings.TrimSpace(cfg.DeploymentID)
	if cfg.DeploymentID == "" {
		cfg.DeploymentID = strings.TrimSpace(os.Getenv(azurefoundry.EnvDeploymentID))
	}
	cfg.APIVersion = strings.TrimSpace(cfg.APIVersion)
	if err := validateAzureFoundryConfig(endpoint, cfg); err != nil {
		return nil, err
	}

	return azurefoundry.New(endpoint, apiKey, azureFoundryOptions(cfg)...), nil
}

func azureFoundryOptions(cfg config.ProviderConfig) []azurefoundry.Option {
	opts := make([]azurefoundry.Option, 0, 3)
	if cfg.DeploymentID != "" {
		opts = append(opts, azurefoundry.WithDeploymentID(cfg.DeploymentID))
	}
	if cfg.APIVersion != "" {
		opts = append(opts, azurefoundry.WithAPIVersion(cfg.APIVersion))
	}
	if cfg.UseOpenAIEndpoint {
		opts = append(opts, azurefoundry.WithOpenAIEndpoint())
	}
	return opts
}

var azureConfigValuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func validateAzureFoundryConfig(endpoint string, cfg config.ProviderConfig) error {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("invalid azurefoundry endpoint %q: must be an absolute HTTP(S) URL", endpoint)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid azurefoundry endpoint %q: credentials, queries, and fragments are not allowed", endpoint)
	}
	if parsed.EscapedPath() != "" && parsed.EscapedPath() != "/" {
		return fmt.Errorf("invalid azurefoundry endpoint %q: path must be empty", endpoint)
	}
	if cfg.DeploymentID != "" && !azureConfigValuePattern.MatchString(cfg.DeploymentID) {
		return fmt.Errorf("invalid azurefoundry deployment_id %q", cfg.DeploymentID)
	}
	if cfg.APIVersion != "" && !azureConfigValuePattern.MatchString(cfg.APIVersion) {
		return fmt.Errorf("invalid azurefoundry api_version %q", cfg.APIVersion)
	}
	return nil
}
