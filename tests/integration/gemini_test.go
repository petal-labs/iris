//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/gemini"
)

func TestGemini_ChatCompletion(t *testing.T) {
	runConformanceChatCompletion(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_Streaming(t *testing.T) {
	runConformanceStreaming(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_WithTools(t *testing.T) {
	runConformanceWithTools(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_SystemMessage(t *testing.T) {
	runConformanceSystemMessage(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_Temperature(t *testing.T) {
	runConformanceTemperature(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_MaxTokens(t *testing.T) {
	runConformanceMaxTokens(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_MultipleMessages(t *testing.T) {
	runConformanceMultipleMessages(t, geminiChatConformanceConfig())
}

func TestGemini_ChatCompletion_Flash(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with Flash model.
	resp, err := client.Chat(gemini.ModelGemini25Flash).
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
	t.Logf("Model: %s", gemini.ModelGemini25Flash)
}

func TestGemini_ImageGeneration(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
		Model:  gemini.ModelGemini25FlashImage,
		Prompt: "A simple red circle on a white background",
	})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}

	if len(resp.Data) == 0 {
		t.Fatal("Expected at least one image in response")
	}

	data, err := resp.Data[0].GetBytes()
	if err != nil {
		t.Fatalf("GetBytes failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Image data is empty")
	}

	t.Logf("Generated image: %d bytes", len(data))
}

func TestGemini_ImageGeneration_StreamingNotSupported(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Streaming should return an error for Gemini.
	_, err := provider.StreamImage(ctx, &core.ImageGenerateRequest{
		Model:  gemini.ModelGemini25FlashImage,
		Prompt: "A simple blue square",
	})
	if err == nil {
		t.Fatal("Expected error for streaming image generation, got nil")
	}

	if !strings.Contains(err.Error(), "not support") {
		t.Logf("Error message: %v", err)
	}

	t.Logf("Got expected error: %v", err)
}

func geminiChatConformanceConfig() chatConformanceConfig {
	return chatConformanceConfig{
		getAPIKey: func(t *testing.T) string {
			skipIfNoGeminiKey(t)
			return getGeminiKey(t)
		},
		newProvider: func(apiKey string) core.Provider {
			return gemini.New(apiKey)
		},
		defaultModel:     gemini.ModelGemini25FlashLite,
		toolModel:        gemini.ModelGemini25FlashLite,
		timeout:          30 * time.Second,
		responseIDPolicy: conformanceNote,
		responseIDNote:   "Response ID is empty (Gemini may not return IDs)",
		usagePolicy:      conformanceRequire,
		strictMaxTokens:  true,
	}
}
