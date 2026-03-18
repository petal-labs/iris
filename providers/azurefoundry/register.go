package azurefoundry

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.Register("azurefoundry", func(apiKey string) core.Provider {
		// When using registry, endpoint must be set via environment
		// or use NewFromEnv/New directly for full configuration
		return New("", apiKey)
	})
}
