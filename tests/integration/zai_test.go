//go:build integration

package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/zai"
)

// zaiTestMutex ensures Z.ai tests run sequentially to avoid rate limiting.
var zaiTestMutex sync.Mutex

// zaiTestSetup acquires the mutex and returns cleanup function.
// This prevents concurrent Z.ai API calls which cause rate limiting.
func zaiTestSetup(t *testing.T) func() {
	t.Helper()
	zaiTestMutex.Lock()
	return func() {
		// Small delay between tests to avoid rate limiting.
		time.Sleep(500 * time.Millisecond)
		zaiTestMutex.Unlock()
	}
}

func TestZai_ChatCompletion(t *testing.T) {
	runConformanceChatCompletion(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_Streaming(t *testing.T) {
	runConformanceStreaming(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_WithTools(t *testing.T) {
	runConformanceWithTools(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_SystemMessage(t *testing.T) {
	runConformanceSystemMessage(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_Temperature(t *testing.T) {
	runConformanceTemperature(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_MaxTokens(t *testing.T) {
	runConformanceMaxTokens(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_MultipleMessages(t *testing.T) {
	runConformanceMultipleMessages(t, zaiChatConformanceConfig())
}

func TestZai_ChatCompletion_GLM47(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Test with GLM-4.7 flagship model.
	resp, err := client.Chat(zai.ModelGLM47).
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
	t.Logf("Model: %s", zai.ModelGLM47)
}

func TestZai_ChatCompletion_GLM45Flash(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with GLM-4.5 Flash model (different series).
	resp, err := client.Chat(zai.ModelGLM45Flash).
		User("What is 10 + 5? Reply with just the number.").
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	if !strings.Contains(resp.Output, "15") {
		t.Errorf("Expected response to contain '15', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Model: %s", zai.ModelGLM45Flash)
}

func zaiChatConformanceConfig() chatConformanceConfig {
	return chatConformanceConfig{
		getAPIKey: func(t *testing.T) string {
			skipIfNoZaiKey(t)
			return getZaiKey(t)
		},
		newProvider: func(apiKey string) core.Provider {
			return zai.New(apiKey)
		},
		defaultModel:     zai.ModelGLM47Flash,
		toolModel:        zai.ModelGLM47Flash,
		timeout:          60 * time.Second,
		responseIDPolicy: conformanceNote,
		responseIDNote:   "Response ID is empty (Z.ai may not return IDs)",
		usagePolicy:      conformanceRequire,
		strictMaxTokens:  true,
		beforeEach:       zaiTestSetup,
	}
}
