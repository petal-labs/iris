package elevenlabs

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.Register(providerID, func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
