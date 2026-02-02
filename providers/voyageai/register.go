package voyageai

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.Register("voyageai", func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
