package ollama

import (
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
)

func init() {
	// Ollama does not require an API key locally, but preserve one when registry
	// callers target an authenticated remote or cloud instance.
	providers.Register("ollama", func(apiKey string) core.Provider {
		return New(WithAPIKey(apiKey))
	})
}
