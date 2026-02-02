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
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(gemini.ModelGemini25FlashLite).
		User("Say 'hello' and nothing else.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	if resp.ID == "" {
		t.Log("Note: Response ID is empty (Gemini may not return IDs)")
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

func TestGemini_ChatCompletion_Streaming(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.Chat(gemini.ModelGemini25FlashLite).
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

func TestGemini_ChatCompletion_WithTools(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tool := createTestTool()

	resp, err := client.Chat(gemini.ModelGemini25FlashLite).
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

func TestGemini_ChatCompletion_SystemMessage(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(gemini.ModelGemini25FlashLite).
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

func TestGemini_ChatCompletion_Temperature(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Low temperature should give more deterministic output
	resp, err := client.Chat(gemini.ModelGemini25FlashLite).
		User("What is 2+2? Reply with just the number.").
		Temperature(0).
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

func TestGemini_ChatCompletion_MaxTokens(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Very low max tokens should truncate response
	resp, err := client.Chat(gemini.ModelGemini25FlashLite).
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

func TestGemini_ChatCompletion_MultipleMessages(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(gemini.ModelGemini25FlashLite).
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

func TestGemini_ChatCompletion_Flash(t *testing.T) {
	skipIfNoGeminiKey(t)

	apiKey := getGeminiKey(t)
	provider := gemini.New(apiKey)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with Flash model
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

	// Streaming should return an error for Gemini
	_, err := provider.StreamImage(ctx, &core.ImageGenerateRequest{
		Model:  gemini.ModelGemini25FlashImage,
		Prompt: "A simple blue square",
	})

	if err == nil {
		t.Fatal("Expected error for streaming image generation, got nil")
	}

	// Verify it's the expected error type
	if !strings.Contains(err.Error(), "not support") {
		t.Logf("Error message: %v", err)
	}

	t.Logf("Got expected error: %v", err)
}
