//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/xai"
)

func TestXai_ChatCompletion(t *testing.T) {
	runConformanceChatCompletion(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_Streaming(t *testing.T) {
	runConformanceStreaming(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_WithTools(t *testing.T) {
	runConformanceWithTools(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_SystemMessage(t *testing.T) {
	runConformanceSystemMessage(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_Temperature(t *testing.T) {
	runConformanceTemperature(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_MaxTokens(t *testing.T) {
	runConformanceMaxTokens(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_MultipleMessages(t *testing.T) {
	runConformanceMultipleMessages(t, xaiChatConformanceConfig())
}

func TestXai_ChatCompletion_Grok3(t *testing.T) {
	skipIfNoXaiKey(t)

	apiKey := getXaiKey(t)
	provider := xai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with Grok 3 flagship model.
	resp, err := client.Chat(xai.ModelGrok3).
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
	t.Logf("Model: %s", xai.ModelGrok3)
}

func TestXai_ChatCompletion_GrokCodeFast(t *testing.T) {
	skipIfNoXaiKey(t)

	apiKey := getXaiKey(t)
	provider := xai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with Grok Code Fast model.
	resp, err := client.Chat(xai.ModelGrokCodeFast).
		User("Write a Python function that adds two numbers. Just the function, no explanation.").
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// Should contain Python function syntax.
	output := strings.ToLower(resp.Output)
	if !strings.Contains(output, "def") {
		t.Logf("Note: Expected response to contain 'def', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Model: %s", xai.ModelGrokCodeFast)
}

func xaiChatConformanceConfig() chatConformanceConfig {
	return chatConformanceConfig{
		getAPIKey: func(t *testing.T) string {
			skipIfNoXaiKey(t)
			return getXaiKey(t)
		},
		newProvider: func(apiKey string) core.Provider {
			return xai.New(apiKey)
		},
		defaultModel:     xai.ModelGrok3Mini,
		toolModel:        xai.ModelGrok3Mini,
		timeout:          30 * time.Second,
		responseIDPolicy: conformanceNote,
		responseIDNote:   "Response ID is empty",
		usagePolicy:      conformanceNote,
		usageNote:        "Response usage total tokens is 0",
		strictMaxTokens:  false,
	}
}
