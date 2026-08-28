package providers

import (
	"fmt"
	"sort"
	"sync"

	"github.com/petal-labs/iris/core"
)

// ProviderFactory creates a provider instance with the given API key.
type ProviderFactory func(apiKey string) core.Provider

// ProviderConfig supplies common construction values to a configured registry
// factory. Endpoint is primarily used by providers such as Azure AI Foundry
// whose resource endpoint cannot be inferred from an API key.
type ProviderConfig struct {
	APIKey   string
	Endpoint string
}

// ConfiguredProviderFactory creates a provider from common registry values.
type ConfiguredProviderFactory func(config ProviderConfig) core.Provider

// registry holds registered provider factories.
var (
	registryMu         sync.RWMutex
	registry           = make(map[string]ProviderFactory)
	configuredRegistry = make(map[string]ConfiguredProviderFactory)
)

// Register adds a provider factory to the registry.
// It is typically called from a provider's init() function.
// If a provider with the same name is already registered, it will be overwritten.
//
// Example usage in a provider package:
//
//	func init() {
//	    providers.Register("openai", func(apiKey string) core.Provider {
//	        return New(apiKey)
//	    })
//	}
func Register(name string, factory ProviderFactory) {
	RegisterConfigured(name, func(config ProviderConfig) core.Provider {
		return factory(config.APIKey)
	})
}

// RegisterConfigured adds a provider factory that can consume common values
// beyond an API key. Get and Create remain available through an API-key-only
// adapter for backward compatibility.
func RegisterConfigured(name string, factory ConfiguredProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	configuredRegistry[name] = factory
	registry[name] = func(apiKey string) core.Provider {
		return factory(ProviderConfig{APIKey: apiKey})
	}
}

// Get retrieves a provider factory by name.
// Returns nil if the provider is not registered.
func Get(name string) ProviderFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// Create creates a new provider instance by name with the given API key.
// Returns an error if the provider is not registered.
func Create(name, apiKey string) (core.Provider, error) {
	return CreateWithConfig(name, ProviderConfig{APIKey: apiKey})
}

// CreateWithConfig creates a registered provider with common construction
// values such as an API key and resource endpoint.
func CreateWithConfig(name string, config ProviderConfig) (core.Provider, error) {
	factory := getConfigured(name)
	if factory == nil {
		return nil, fmt.Errorf("unknown provider: %s (available: %v)", name, List())
	}
	return factory(config), nil
}

func getConfigured(name string) ConfiguredProviderFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return configuredRegistry[name]
}

// List returns the names of all registered providers in sorted order.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsRegistered returns true if a provider with the given name is registered.
func IsRegistered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}
