package commands

import (
	"fmt"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/gemini"
	"github.com/petal-labs/iris/providers/huggingface"
	"github.com/petal-labs/iris/providers/ollama"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/xai"
	"github.com/petal-labs/iris/providers/zai"
)

type providerConstructor func(apiKey, baseURL string) (core.Provider, error)

func defaultProviderFactory() ProviderFactory {
	constructors := map[string]providerConstructor{
		"openai": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []openai.Option
			if baseURL != "" {
				opts = append(opts, openai.WithBaseURL(baseURL))
			}
			return openai.New(apiKey, opts...), nil
		},
		"anthropic": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []anthropic.Option
			if baseURL != "" {
				opts = append(opts, anthropic.WithBaseURL(baseURL))
			}
			return anthropic.New(apiKey, opts...), nil
		},
		"gemini": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []gemini.Option
			if baseURL != "" {
				opts = append(opts, gemini.WithBaseURL(baseURL))
			}
			return gemini.New(apiKey, opts...), nil
		},
		"xai": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []xai.Option
			if baseURL != "" {
				opts = append(opts, xai.WithBaseURL(baseURL))
			}
			return xai.New(apiKey, opts...), nil
		},
		"zai": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []zai.Option
			if baseURL != "" {
				opts = append(opts, zai.WithBaseURL(baseURL))
			}
			return zai.New(apiKey, opts...), nil
		},
		"ollama": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []ollama.Option
			if baseURL != "" {
				opts = append(opts, ollama.WithBaseURL(baseURL))
			}
			if apiKey != "" {
				opts = append(opts, ollama.WithAPIKey(apiKey))
			}
			return ollama.New(opts...), nil
		},
		"huggingface": func(apiKey, baseURL string) (core.Provider, error) {
			var opts []huggingface.Option
			if baseURL != "" {
				opts = append(opts, huggingface.WithBaseURL(baseURL))
			}
			return huggingface.New(apiKey, opts...), nil
		},
	}

	return func(providerID, apiKey string, cfg *config.Config) (core.Provider, error) {
		baseURL := providerBaseURL(cfg, providerID)
		if ctor, ok := constructors[providerID]; ok {
			return ctor(apiKey, baseURL)
		}

		// Fall back to registry for externally-registered providers.
		if providers.IsRegistered(providerID) {
			return providers.Create(providerID, apiKey)
		}

		return nil, fmt.Errorf("unsupported provider: %s (available: %v)", providerID, providers.List())
	}
}

func providerBaseURL(cfg *config.Config, providerID string) string {
	if cfg == nil {
		return ""
	}
	pc := cfg.GetProvider(providerID)
	if pc == nil {
		return ""
	}
	return pc.BaseURL
}
