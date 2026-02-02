package providers

import (
	"fmt"
	"sort"
	"sync"

	"github.com/petal-labs/iris/core"
)

// ProviderFactory creates a provider instance with the given API key.
// Some providers (like Ollama) may ignore the key parameter.
type ProviderFactory func(apiKey string) core.Provider

// registry holds registered provider factories.
var (
	registryMu sync.RWMutex
	registry   = make(map[string]ProviderFactory)
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
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
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
	factory := Get(name)
	if factory == nil {
		return nil, fmt.Errorf("unknown provider: %s (available: %v)", name, List())
	}
	return factory(apiKey), nil
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
