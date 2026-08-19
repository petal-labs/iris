package perplexity

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	providers.Register("perplexity", func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
