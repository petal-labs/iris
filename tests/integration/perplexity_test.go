//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/perplexity"
)

func TestPerplexity_ChatCompletion(t *testing.T) {
	runConformanceChatCompletion(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_Streaming(t *testing.T) {
	runConformanceStreaming(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_WithTools(t *testing.T) {
	runConformanceWithTools(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_SystemMessage(t *testing.T) {
	runConformanceSystemMessage(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_Temperature(t *testing.T) {
	runConformanceTemperature(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_MaxTokens(t *testing.T) {
	runConformanceMaxTokens(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_MultipleMessages(t *testing.T) {
	runConformanceMultipleMessages(t, perplexityChatConformanceConfig())
}

func TestPerplexity_ChatCompletion_SonarPro(t *testing.T) {
	skipIfNoPerplexityKey(t)

	apiKey := getPerplexityKey(t)
	provider := perplexity.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with Sonar Pro model.
	resp, err := client.Chat(perplexity.ModelSonarPro).
		User("What is the capital of France? Answer in one word.").
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	output := strings.ToLower(resp.Output)
	if !strings.Contains(output, "paris") {
		t.Errorf("Expected response to contain 'paris', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Model: %s", perplexity.ModelSonarPro)
}

func perplexityChatConformanceConfig() chatConformanceConfig {
	return chatConformanceConfig{
		getAPIKey: func(t *testing.T) string {
			skipIfNoPerplexityKey(t)
			return getPerplexityKey(t)
		},
		newProvider: func(apiKey string) core.Provider {
			return perplexity.New(apiKey)
		},
		defaultModel:     perplexity.ModelSonar,
		toolModel:        perplexity.ModelSonarPro,
		timeout:          30 * time.Second,
		responseIDPolicy: conformanceNote,
		responseIDNote:   "Response ID is empty",
		usagePolicy:      conformanceNote,
		usageNote:        "Response usage total tokens is 0",
		strictMaxTokens:  false,
	}
}
