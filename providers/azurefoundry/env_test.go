package azurefoundry

import (
	"errors"
	"testing"
)

func TestNewFromEnv(t *testing.T) {
	t.Run("missing endpoint", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "")
		t.Setenv(EnvAPIKey, "key")
		provider, err := NewFromEnv()
		if !errors.Is(err, ErrEndpointNotFound) || provider != nil {
			t.Fatalf("NewFromEnv() = (%v, %v), want nil, ErrEndpointNotFound", provider, err)
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "https://resource.services.ai.azure.com")
		t.Setenv(EnvAPIKey, "")
		provider, err := NewFromEnv()
		if !errors.Is(err, ErrAPIKeyNotFound) || provider != nil {
			t.Fatalf("NewFromEnv() = (%v, %v), want nil, ErrAPIKeyNotFound", provider, err)
		}
	})

	t.Run("environment and option precedence", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "https://resource.services.ai.azure.com")
		t.Setenv(EnvAPIKey, "env-key")
		t.Setenv(EnvDeploymentID, "env-deployment")
		provider, err := NewFromEnv(WithAPIKey("option-key"), WithDeploymentID("option-deployment"))
		if err != nil {
			t.Fatalf("NewFromEnv() error = %v", err)
		}
		if provider.config.Endpoint != "https://resource.services.ai.azure.com" {
			t.Errorf("Endpoint = %q, want environment endpoint", provider.config.Endpoint)
		}
		if provider.config.APIKey.Expose() != "option-key" || provider.config.DeploymentID != "option-deployment" {
			t.Error("explicit options should override environment-derived values")
		}
	})
}
