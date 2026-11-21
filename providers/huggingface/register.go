package huggingface

import (
	"github.com/petal-labs/iris/v2/core"
	"github.com/petal-labs/iris/v2/providers"
)

func init() {
	providers.Register("huggingface", func(apiKey string) core.Provider {
		return New(apiKey)
	})
}
