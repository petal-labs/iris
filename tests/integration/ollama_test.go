//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/ollama"
)

// TestOllama_ChatCompletion tests basic chat completion with local Ollama.
// Requires: ollama running locally with gemma3 model pulled.
func TestOllama_ChatCompletion(t *testing.T) {
	skipIfNoOllama(t)

	provider := ollama.NewLocal()
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("gemma3").
		User("Say 'hello' and nothing else.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Usage: %d prompt + %d completion = %d total",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}

// TestOllama_ChatCompletion_Streaming tests streaming chat completion.
// Requires: ollama running locally with gemma3 model pulled.
func TestOllama_ChatCompletion_Streaming(t *testing.T) {
	skipIfNoOllama(t)

	provider := ollama.NewLocal()
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.Chat("gemma3").
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
		t.Log("No final response received (may be expected for Ollama)")
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
		t.Logf("Final response model: %s", finalResp.Model)
	}
}

// TestOllama_ChatCompletion_SystemMessage tests chat with a system message.
// Requires: ollama running locally with gemma3 model pulled.
func TestOllama_ChatCompletion_SystemMessage(t *testing.T) {
	skipIfNoOllama(t)

	provider := ollama.NewLocal()
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("gemma3").
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
	pirateWords := []string{"ahoy", "matey", "arr", "aye", "ye", "ship", "sail", "sea", "yo ho"}

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

// TestOllama_ChatCompletion_Temperature tests chat with temperature parameter.
// Requires: ollama running locally with gemma3 model pulled.
func TestOllama_ChatCompletion_Temperature(t *testing.T) {
	skipIfNoOllama(t)

	provider := ollama.NewLocal()
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Low temperature should give more deterministic output
	resp, err := client.Chat("gemma3").
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

// TestOllama_ChatCompletion_MultiTurn tests multi-turn conversation.
// Requires: ollama running locally with gemma3 model pulled.
func TestOllama_ChatCompletion_MultiTurn(t *testing.T) {
	skipIfNoOllama(t)

	provider := ollama.NewLocal()
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// First turn: set context
	resp1, err := client.Chat("gemma3").
		User("My name is Alice. Remember this.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("First turn error = %v", err)
	}

	t.Logf("First response: %s", resp1.Output)

	// Second turn: recall context
	resp2, err := client.Chat("gemma3").
		User("My name is Alice. Remember this.").
		Assistant(resp1.Output).
		User("What is my name?").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Second turn error = %v", err)
	}

	// Response should mention "Alice"
	if !strings.Contains(strings.ToLower(resp2.Output), "alice") {
		t.Logf("Note: Response may not have remembered the name: %s", resp2.Output)
	}

	t.Logf("Second response: %s", resp2.Output)
}

// TestOllama_NewLocalWithOptions tests provider creation with options.
func TestOllama_NewLocalWithOptions(t *testing.T) {
	skipIfNoOllama(t)

	// Test that we can create with custom timeout option
	provider := ollama.NewLocal(
		ollama.WithTimeout(60*time.Second),
	)

	if provider.ID() != "ollama" {
		t.Errorf("ID() = %q, want %q", provider.ID(), "ollama")
	}

	// Verify provider works
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("gemma3").
		User("Say 'test' and nothing else.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	t.Logf("Response: %s", resp.Output)
}

// TestOllama_SupportsFeatures tests provider feature support.
func TestOllama_SupportsFeatures(t *testing.T) {
	provider := ollama.NewLocal()

	tests := []struct {
		feature core.Feature
		want    bool
	}{
		{core.FeatureChat, true},
		{core.FeatureChatStreaming, true},
		{core.FeatureToolCalling, true},
		{core.FeatureReasoning, true},
		{core.FeatureEmbeddings, false},       // Not supported
		{core.FeatureImageGeneration, false},  // Not supported
	}

	for _, tt := range tests {
		t.Run(string(tt.feature), func(t *testing.T) {
			if got := provider.Supports(tt.feature); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}
