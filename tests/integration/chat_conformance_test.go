//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

type conformancePresencePolicy int

const (
	conformanceIgnore conformancePresencePolicy = iota
	conformanceRequire
	conformanceNote
)

type chatConformanceConfig struct {
	getAPIKey   func(t *testing.T) string
	newProvider func(apiKey string) core.Provider

	defaultModel core.ModelID
	toolModel    core.ModelID

	timeout      time.Duration
	finalTimeout time.Duration

	responseIDPolicy conformancePresencePolicy
	responseIDNote   string

	usagePolicy conformancePresencePolicy
	usageNote   string

	strictMaxTokens bool
	maxTokensNote   string

	beforeEach func(t *testing.T) func()
}

func (c chatConformanceConfig) normalized() chatConformanceConfig {
	cfg := c
	if cfg.toolModel == "" {
		cfg.toolModel = cfg.defaultModel
	}
	if cfg.timeout <= 0 {
		cfg.timeout = 30 * time.Second
	}
	if cfg.finalTimeout <= 0 {
		cfg.finalTimeout = 5 * time.Second
	}
	return cfg
}

func newConformanceClient(t *testing.T, cfg chatConformanceConfig) (*core.Client, func()) {
	t.Helper()

	cleanup := func() {}
	if cfg.beforeEach != nil {
		if c := cfg.beforeEach(t); c != nil {
			cleanup = c
		}
	}

	apiKey := cfg.getAPIKey(t)
	provider := cfg.newProvider(apiKey)
	if provider == nil {
		cleanup()
		t.Fatal("conformance provider factory returned nil")
	}

	return core.NewClient(provider), cleanup
}

func assertStringPresence(t *testing.T, label, value string, policy conformancePresencePolicy, note string) {
	t.Helper()
	if value != "" {
		return
	}

	switch policy {
	case conformanceRequire:
		t.Errorf("%s is empty", label)
	case conformanceNote:
		if note != "" {
			t.Logf("Note: %s", note)
		} else {
			t.Logf("Note: %s is empty", label)
		}
	}
}

func assertUsagePresence(t *testing.T, total int, policy conformancePresencePolicy, note string) {
	t.Helper()
	if total > 0 {
		return
	}

	switch policy {
	case conformanceRequire:
		t.Error("Response usage total tokens is 0")
	case conformanceNote:
		if note != "" {
			t.Logf("Note: %s", note)
		} else {
			t.Log("Note: Response usage total tokens is 0")
		}
	}
}

func runConformanceChatCompletion(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	resp, err := client.Chat(cfg.defaultModel).
		User("Say 'hello' and nothing else.").
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	assertStringPresence(t, "Response ID", resp.ID, cfg.responseIDPolicy, cfg.responseIDNote)
	assertUsagePresence(t, resp.Usage.TotalTokens, cfg.usagePolicy, cfg.usageNote)

	t.Logf("Response: %s", resp.Output)
	t.Logf("Usage: %d prompt + %d completion = %d total",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}

func runConformanceStreaming(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	stream, err := client.Chat(cfg.defaultModel).
		User("Count from 1 to 5, each number on a new line.").
		Stream(ctx)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []string
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk.Delta)
	}

	select {
	case err := <-stream.Err:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	var finalResp *core.ChatResponse
	select {
	case resp := <-stream.Final:
		finalResp = resp
	case <-time.After(cfg.finalTimeout):
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

func runConformanceWithTools(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	resp, err := client.Chat(cfg.toolModel).
		User("What's the weather like in San Francisco?").
		Tools(createTestTool()).
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" && len(resp.ToolCalls) == 0 {
		t.Error("Response has no output and no tool calls")
	}

	if len(resp.ToolCalls) > 0 {
		t.Logf("Tool call: %s", resp.ToolCalls[0].Name)
		t.Logf("Arguments: %s", string(resp.ToolCalls[0].Arguments))

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

func runConformanceSystemMessage(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	resp, err := client.Chat(cfg.defaultModel).
		System("You are a pirate. Always respond in pirate speak.").
		User("Say hello.").
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

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

func runConformanceTemperature(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	resp, err := client.Chat(cfg.defaultModel).
		User("What is 2+2? Reply with just the number.").
		Temperature(0).
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Output == "" {
		t.Error("Response output is empty")
	}

	if !strings.Contains(resp.Output, "4") {
		t.Errorf("Expected response to contain '4', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}

func runConformanceMaxTokens(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	resp, err := client.Chat(cfg.defaultModel).
		User("Write a long story about a dragon.").
		MaxTokens(10).
		GetResponse(ctx)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Usage.CompletionTokens > 15 {
		if cfg.strictMaxTokens {
			t.Errorf("Expected completion tokens <= 15, got %d", resp.Usage.CompletionTokens)
		} else if cfg.maxTokensNote != "" {
			t.Logf("Note: %s", cfg.maxTokensNote)
		} else {
			t.Logf("Note: Completion tokens %d exceeds expected max ~10", resp.Usage.CompletionTokens)
		}
	}

	t.Logf("Response: %s", resp.Output)
	t.Logf("Completion tokens: %d", resp.Usage.CompletionTokens)
}

func runConformanceMultipleMessages(t *testing.T, cfg chatConformanceConfig) {
	t.Helper()
	cfg = cfg.normalized()

	client, cleanup := newConformanceClient(t, cfg)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	resp, err := client.Chat(cfg.defaultModel).
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

	if !strings.Contains(strings.ToLower(resp.Output), "alice") {
		t.Errorf("Expected response to contain 'Alice', got: %s", resp.Output)
	}

	t.Logf("Response: %s", resp.Output)
}
