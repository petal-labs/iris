package azurefoundry

import (
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.RegisterConfigured("azurefoundry", func(config providers.ProviderConfig) core.Provider {
		endpoint := config.Endpoint
		if endpoint == "" {
			endpoint = os.Getenv(EnvEndpoint)
		}
		return New(endpoint, config.APIKey)
	})
}
