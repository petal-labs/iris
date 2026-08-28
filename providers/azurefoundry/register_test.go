package azurefoundry

import (
	"testing"

	"github.com/petal-labs/iris/providers"
)

func TestConfiguredRegistryFactory(t *testing.T) {
	created, err := providers.CreateWithConfig("azurefoundry", providers.ProviderConfig{
		APIKey:   "registry-key",
		Endpoint: "https://configured.services.ai.azure.com",
	})
	if err != nil {
		t.Fatalf("CreateWithConfig() error = %v", err)
	}
	provider, ok := created.(*AzureFoundry)
	if !ok {
		t.Fatalf("provider type = %T, want *AzureFoundry", created)
	}
	if provider.config.Endpoint != "https://configured.services.ai.azure.com" || provider.config.APIKey.Expose() != "registry-key" {
		t.Errorf("registry config = %#v, want endpoint and API key", provider.config)
	}
}

func TestLegacyRegistryFactoryUsesEnvironmentEndpoint(t *testing.T) {
	t.Setenv(EnvEndpoint, "https://environment.services.ai.azure.com")
	created, err := providers.Create("azurefoundry", "registry-key")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	provider := created.(*AzureFoundry)
	if provider.config.Endpoint != "https://environment.services.ai.azure.com" {
		t.Errorf("Endpoint = %q, want environment endpoint", provider.config.Endpoint)
	}
}
