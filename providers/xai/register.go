package xai

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.Register("xai", func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
