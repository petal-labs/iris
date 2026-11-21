package elevenlabs

import (
	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers"
)

func init() {
	providers.Register(providerID, func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
