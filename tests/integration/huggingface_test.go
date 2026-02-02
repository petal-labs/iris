//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/huggingface"
)

// skipIfNoHFToken skips the test if HF_TOKEN is not set.
// HuggingFace tests are always skipped in CI due to rate limiting and reliability issues.
func skipIfNoHFToken(t *testing.T) {
	t.Helper()
	// Always skip HuggingFace tests in CI - they are flaky due to rate limiting
	skipInCI(t, "HuggingFace tests are flaky in CI due to rate limiting")
	if os.Getenv("HF_TOKEN") == "" && os.Getenv("IRIS_HF_TOKEN") == "" {
		t.Skip("HF_TOKEN or IRIS_HF_TOKEN not set")
	}
}

// getHFToken returns the Hugging Face token from environment.
func getHFToken(t *testing.T) string {
	t.Helper()
	key := os.Getenv("HF_TOKEN")
	if key == "" {
		key = os.Getenv("IRIS_HF_TOKEN")
	}
	if key == "" {
		t.Fatal("HF_TOKEN or IRIS_HF_TOKEN not set")
	}
	return key
}

func TestHuggingFace_ChatCompletion(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use a commonly available model
	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
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

func TestHuggingFace_ChatCompletion_Streaming(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
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

func TestHuggingFace_ChatCompletion_WithProviderPolicy(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	// Use fastest provider policy
	provider := huggingface.New(token, huggingface.WithProviderPolicy(huggingface.PolicyFastest))
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
		User("What is 2+2? Reply with just the number.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// Should contain "4"
	if !strings.Contains(resp.Output, "4") {
		t.Logf("Note: Expected response to contain '4', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func TestHuggingFace_ChatCompletion_WithTools(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tool := createTestTool()

	// Use a model known to support tool calling
	resp, err := client.Chat("Qwen/Qwen2.5-7B-Instruct").
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

		if resp.ToolCalls[0].ID == "" {
			t.Error("Tool call ID is empty")
		}
	} else {
		t.Logf("Model responded with text: %s", resp.Output)
	}
}

func TestHuggingFace_GetModelStatus(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with a model known to have inference providers
	status, err := provider.GetModelStatus(ctx, "meta-llama/Llama-3.1-8B-Instruct")
	if err != nil {
		t.Fatalf("GetModelStatus() error = %v", err)
	}

	t.Logf("Model status for meta-llama/Llama-3.1-8B-Instruct: %s", status)

	if status != huggingface.ModelStatusWarm {
		t.Logf("Note: Model status is %s (expected warm)", status)
	}
}

func TestHuggingFace_GetModelProviders(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providers, err := provider.GetModelProviders(ctx, "meta-llama/Llama-3.1-8B-Instruct")
	if err != nil {
		t.Fatalf("GetModelProviders() error = %v", err)
	}

	t.Logf("Found %d providers for meta-llama/Llama-3.1-8B-Instruct", len(providers))

	for _, p := range providers {
		t.Logf("  - %s (status=%s, task=%s)", p.Name, p.Status, p.Task)
	}
}

func TestHuggingFace_ListModels(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := provider.ListModels(ctx, huggingface.ListModelsOptions{
		Provider:    "all",
		PipelineTag: "text-generation",
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	if len(models) == 0 {
		t.Error("No models returned")
	}

	t.Logf("Found %d models with text-generation pipeline", len(models))

	for _, m := range models {
		t.Logf("  - %s (pipeline=%s)", m.ID, m.PipelineTag)
	}
}

func TestHuggingFace_ListModels_ByProvider(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List models from a specific provider
	models, err := provider.ListModels(ctx, huggingface.ListModelsOptions{
		Provider: "cerebras",
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	t.Logf("Found %d models from Cerebras provider", len(models))

	for _, m := range models {
		t.Logf("  - %s", m.ID)
	}
}

func TestHuggingFace_ChatCompletion_SystemMessage(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
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

func TestHuggingFace_ChatCompletion_Temperature(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Low temperature should give more deterministic output
	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
		User("What is 2+2? Reply with just the number.").
		Temperature(0.01). // Some models don't support exactly 0
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// Should contain "4"
	if !strings.Contains(resp.Output, "4") {
		t.Logf("Note: Expected response to contain '4', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func TestHuggingFace_ChatCompletion_MaxTokens(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Very low max tokens should truncate response
	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
		User("Write a long story about a dragon.").
		MaxTokens(20).
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// Response should be short due to max tokens
	if resp.Usage.CompletionTokens > 25 { // Allow some buffer
		t.Logf("Note: Completion tokens %d exceeds expected max ~20", resp.Usage.CompletionTokens)
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Completion tokens: %d", resp.Usage.CompletionTokens)
}

func TestHuggingFace_ChatCompletion_MultipleMessages(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
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
		t.Logf("Note: Expected response to contain 'Alice', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func TestHuggingFace_ChatCompletion_WithCheapestPolicy(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	// Use cheapest provider policy
	provider := huggingface.New(token, huggingface.WithProviderPolicy(huggingface.PolicyCheapest))
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat("meta-llama/Llama-3.1-8B-Instruct").
		User("What is 5+5? Reply with just the number.").
		GetResponse(ctx)

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	// Should contain "10"
	if !strings.Contains(resp.Output, "10") {
		t.Logf("Note: Expected response to contain '10', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func TestHuggingFace_ChatCompletion_QwenModel(t *testing.T) {
	skipIfNoHFToken(t)

	token := getHFToken(t)
	provider := huggingface.New(token)
	client := core.NewClient(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with a different model family (Qwen)
	resp, err := client.Chat("Qwen/Qwen2.5-7B-Instruct").
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
		t.Logf("Note: Expected response to contain 'paris', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Model: Qwen/Qwen2.5-7B-Instruct")
}
