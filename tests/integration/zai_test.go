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
		// Small delay between tests to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
		zaiTestMutex.Unlock()
	}
}

func TestZai_ChatCompletion(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(zai.ModelGLM47Flash).
		User("Say 'hello' and nothing else.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	if resp.ID == "" {
		t.Log("Note: Response ID is empty (Z.ai may not return IDs)")
	}

	// Verify usage is populated
	if resp.Usage.TotalTokens == 0 {
		t.Error("Response usage total tokens is 0")
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Usage: %d prompt + %d completion = %d total",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}

func TestZai_ChatCompletion_Streaming(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.Chat(zai.ModelGLM47Flash).
		User("Count from 1 to 5, each number on a new line.").
		Stream(ctx)

	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	// Collect deltas
	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	// Check for errors
	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	// Wait for final response
	var finalResp *core.ChatResponse
	select {
	case resp := <-stream.Final:
		finalResp = resp
	case <-time.After(5 * time.Second):
		t.Log("No final response received (may be expected)")
	}

	if len(chunks) == 0 {
		t.Error("No chunks received")
	}

	combined := strings.Join(chunks, "")
	if combined == "" {
		t.Error("Combined output is empty")
	}

	t.Logf("Received %d chunks", len(chunks))
	t.Logf("Combined output: %s", combined)

	if finalResp != nil {
		t.Logf("Final response ID: %s", finalResp.ID)
	}
}

func TestZai_ChatCompletion_WithTools(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tool := createTestTool()

	resp, err := client.Chat(zai.ModelGLM47Flash).
		User("What's the weather like in San Francisco?").
		Tools(tool).
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// The model should either call the tool or respond with text
	if resp.Output == "" && len(resp.ToolCalls) == 0 {
		t.Error("Response has no output and no tool calls")
	}

	if len(resp.ToolCalls) > 0 {
		t.Logf("Tool call: %s", resp.ToolCalls[0].Name)
		t.Logf("Arguments: %s", string(resp.ToolCalls[0].Arguments))

		// Verify tool call has expected structure
		if resp.ToolCalls[0].Name != "get_weather" {
			t.Logf("Note: Model called %s instead of get_weather", resp.ToolCalls[0].Name)
		}

		if resp.ToolCalls[0].ID == "" {
			t.Error("Tool call ID is empty")
		}
	} else {
		t.Logf("Model responded with text: %s", resp.Output)
	}
}

func TestZai_ChatCompletion_SystemMessage(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(zai.ModelGLM47Flash).
		System("You are a pirate. Always respond in pirate speak.").
		User("Say hello.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// The output should contain pirate-like language
	output := strings.ToLower(resp.Output)
	pirateWords := []string{"ahoy", "matey", "arr", "aye", "ye", "ship", "sail", "sea"}

	hasPirateWord := false
	for _, word := range pirateWords {
		if strings.Contains(output, word) {
			hasPirateWord = true
			break
		}
	}

	if !hasPirateWord {
		t.Logf("Note: Response may not be in pirate speak: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func TestZai_ChatCompletion_Temperature(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Low temperature should give more deterministic output
	resp, err := client.Chat(zai.ModelGLM47Flash).
		User("What is 2+2? Reply with just the number.").
		Temperature(0.01). // Z.ai may not support exactly 0
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// Should contain "4"
	if !strings.Contains(resp.Output, "4") {
		t.Errorf("Expected response to contain '4', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func TestZai_ChatCompletion_MaxTokens(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Very low max tokens should truncate response
	resp, err := client.Chat(zai.ModelGLM47Flash).
		User("Write a long story about a dragon.").
		MaxTokens(10).
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Response should be short due to max tokens
	if resp.Usage.CompletionTokens > 15 { // Allow some buffer
		t.Errorf("Expected completion tokens <= 15, got %d", resp.Usage.CompletionTokens)
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Completion tokens: %d", resp.Usage.CompletionTokens)
}

func TestZai_ChatCompletion_MultipleMessages(t *testing.T) {
	skipIfNoZaiKey(t)
	cleanup := zaiTestSetup(t)
	defer cleanup()

	apiKey := getZaiKey(t)
	provider := zai.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(zai.ModelGLM47Flash).
		User("My name is Alice.").
		Assistant("Nice to meet you, Alice!").
		User("What's my name?").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// The response should remember the user's name
	output := strings.ToLower(resp.Output)
	if !strings.Contains(output, "alice") {
		t.Errorf("Expected response to contain 'Alice', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
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

	// Test with GLM-4.7 flagship model
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

	// Test with GLM-4.5 Flash model (different series)
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
