package ollama

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	// Ollama doesn't require an API key, so we ignore the apiKey parameter.
	providers.Register("ollama", func(apiKey string) core.Provider {
		return New()
	})
}
