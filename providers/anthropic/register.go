package anthropic

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.Register("anthropic", func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
