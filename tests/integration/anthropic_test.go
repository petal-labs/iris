//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/anthropic"
)

func TestAnthropic_ChatCompletion(t *testing.T) {
	runConformanceChatCompletion(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_Streaming(t *testing.T) {
	runConformanceStreaming(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_WithTools(t *testing.T) {
	runConformanceWithTools(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_SystemMessage(t *testing.T) {
	runConformanceSystemMessage(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_Temperature(t *testing.T) {
	runConformanceTemperature(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_MaxTokens(t *testing.T) {
	runConformanceMaxTokens(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_MultipleMessages(t *testing.T) {
	runConformanceMultipleMessages(t, anthropicChatConformanceConfig())
}

func TestAnthropic_ChatCompletion_Sonnet(t *testing.T) {
	skipIfNoAnthropicKey(t)

	apiKey := getAnthropicKey(t)
	provider := anthropic.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with Sonnet model for more complex reasoning.
	resp, err := client.Chat(anthropic.ModelClaudeSonnet45).
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
	t.Logf("Model: %s", anthropic.ModelClaudeSonnet45)
}

func anthropicChatConformanceConfig() chatConformanceConfig {
	return chatConformanceConfig{
		getAPIKey: func(t *testing.T) string {
			skipIfNoAnthropicKey(t)
			return getAnthropicKey(t)
		},
		newProvider: func(apiKey string) core.Provider {
			return anthropic.New(apiKey)
		},
		defaultModel:     anthropic.ModelClaudeHaiku45,
		toolModel:        anthropic.ModelClaudeHaiku45,
		timeout:          30 * time.Second,
		responseIDPolicy: conformanceRequire,
		usagePolicy:      conformanceRequire,
		strictMaxTokens:  true,
	}
}
