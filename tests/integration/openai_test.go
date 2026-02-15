//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
)

func TestOpenAI_ChatCompletion(t *testing.T) {
	runConformanceChatCompletion(t, openAIChatConformanceConfig())
}

func TestOpenAI_ChatCompletion_Streaming(t *testing.T) {
	runConformanceStreaming(t, openAIChatConformanceConfig())
}

func TestOpenAI_ChatCompletion_WithTools(t *testing.T) {
	runConformanceWithTools(t, openAIChatConformanceConfig())
}

func TestOpenAI_ChatCompletion_SystemMessage(t *testing.T) {
	runConformanceSystemMessage(t, openAIChatConformanceConfig())
}

func TestOpenAI_ChatCompletion_Temperature(t *testing.T) {
	runConformanceTemperature(t, openAIChatConformanceConfig())
}

func TestOpenAI_ChatCompletion_MaxTokens(t *testing.T) {
	runConformanceMaxTokens(t, openAIChatConformanceConfig())
}

func openAIChatConformanceConfig() chatConformanceConfig {
	return chatConformanceConfig{
		getAPIKey: func(t *testing.T) string {
			skipIfNoAPIKey(t)
			return getAPIKey(t)
		},
		newProvider: func(apiKey string) core.Provider {
			return openai.New(apiKey)
		},
		defaultModel:     openai.ModelGPT4oMini,
		toolModel:        openai.ModelGPT4oMini,
		timeout:          30 * time.Second,
		responseIDPolicy: conformanceRequire,
		usagePolicy:      conformanceRequire,
		strictMaxTokens:  true,
	}
}
