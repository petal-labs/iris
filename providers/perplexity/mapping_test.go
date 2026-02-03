package perplexity

import (
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestMapMessages(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "You are a helpful assistant."},
		{Role: core.RoleUser, Content: "Hello"},
		{Role: core.RoleAssistant, Content: "Hi there!"},
	}

	result := mapMessages(msgs)

	if len(result) != 3 {
		t.Fatalf("mapMessages() returned %d messages, want 3", len(result))
	}

	tests := []struct {
		index   int
		role    string
		content string
	}{
		{0, "system", "You are a helpful assistant."},
		{1, "user", "Hello"},
		{2, "assistant", "Hi there!"},
	}

	for _, tt := range tests {
		if result[tt.index].Role != tt.role {
			t.Errorf("result[%d].Role = %q, want %q", tt.index, result[tt.index].Role, tt.role)
		}
		if result[tt.index].Content != tt.content {
			t.Errorf("result[%d].Content = %q, want %q", tt.index, result[tt.index].Content, tt.content)
		}
	}
}

func TestMapTools(t *testing.T) {
	t.Run("empty tools", func(t *testing.T) {
		result := mapTools(nil)
		if result != nil {
			t.Errorf("mapTools(nil) = %v, want nil", result)
		}

		result = mapTools([]core.Tool{})
		if result != nil {
			t.Errorf("mapTools([]) = %v, want nil", result)
		}
	})

	t.Run("with tools", func(t *testing.T) {
		tools := []core.Tool{
			&mockTool{name: "get_weather", description: "Get weather for a city"},
			&mockTool{name: "search", description: "Search the web"},
		}

		result := mapTools(tools)

		if len(result) != 2 {
			t.Fatalf("mapTools() returned %d tools, want 2", len(result))
		}

		if result[0].Type != "function" {
			t.Errorf("result[0].Type = %q, want %q", result[0].Type, "function")
		}
		if result[0].Function.Name != "get_weather" {
			t.Errorf("result[0].Function.Name = %q, want %q", result[0].Function.Name, "get_weather")
		}
		if result[0].Function.Description != "Get weather for a city" {
			t.Errorf("result[0].Function.Description = %q, want %q", result[0].Function.Description, "Get weather for a city")
		}

		if result[1].Function.Name != "search" {
			t.Errorf("result[1].Function.Name = %q, want %q", result[1].Function.Name, "search")
		}
	})
}

func TestMapReasoningEffort(t *testing.T) {
	tests := []struct {
		effort core.ReasoningEffort
		want   string
	}{
		{core.ReasoningEffortNone, ""},
		{core.ReasoningEffortLow, "low"},
		{core.ReasoningEffortMedium, "medium"},
		{core.ReasoningEffortHigh, "high"},
		{core.ReasoningEffortXHigh, "high"}, // maps to high
		{core.ReasoningEffort("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.effort), func(t *testing.T) {
			got := mapReasoningEffort(tt.effort)
			if got != tt.want {
				t.Errorf("mapReasoningEffort(%q) = %q, want %q", tt.effort, got, tt.want)
			}
		})
	}
}

func TestBuildRequest(t *testing.T) {
	t.Run("basic request", func(t *testing.T) {
		req := &core.ChatRequest{
			Model: "sonar",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		}

		result := buildRequest(req, false)

		if result.Model != "sonar" {
			t.Errorf("Model = %q, want %q", result.Model, "sonar")
		}
		if result.Stream {
			t.Error("Stream should be false")
		}
		if len(result.Messages) != 1 {
			t.Fatalf("Messages count = %d, want 1", len(result.Messages))
		}
	})

	t.Run("with stream", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hello"}},
		}

		result := buildRequest(req, true)

		if !result.Stream {
			t.Error("Stream should be true")
		}
	})

	t.Run("with temperature and max tokens", func(t *testing.T) {
		temp := float32(0.7)
		maxTokens := 100

		req := &core.ChatRequest{
			Model:       "sonar",
			Messages:    []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		}

		result := buildRequest(req, false)

		if result.Temperature == nil || *result.Temperature != 0.7 {
			t.Errorf("Temperature = %v, want 0.7", result.Temperature)
		}
		if result.MaxTokens == nil || *result.MaxTokens != 100 {
			t.Errorf("MaxTokens = %v, want 100", result.MaxTokens)
		}
	})

	t.Run("with tools", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:    "sonar",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			Tools: []core.Tool{
				&mockTool{name: "test_tool", description: "A test tool"},
			},
		}

		result := buildRequest(req, false)

		if len(result.Tools) != 1 {
			t.Fatalf("Tools count = %d, want 1", len(result.Tools))
		}
		if result.ToolChoice != "auto" {
			t.Errorf("ToolChoice = %q, want %q", result.ToolChoice, "auto")
		}
	})

	t.Run("with reasoning effort", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:           "sonar-reasoning-pro",
			Messages:        []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			ReasoningEffort: core.ReasoningEffortHigh,
		}

		result := buildRequest(req, false)

		if result.ReasoningEffort != "high" {
			t.Errorf("ReasoningEffort = %q, want %q", result.ReasoningEffort, "high")
		}
	})

	t.Run("reasoning effort none is omitted", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:           "sonar",
			Messages:        []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			ReasoningEffort: core.ReasoningEffortNone,
		}

		result := buildRequest(req, false)

		if result.ReasoningEffort != "" {
			t.Errorf("ReasoningEffort should be empty for None, got %q", result.ReasoningEffort)
		}
	})
}

func TestMapResponse(t *testing.T) {
	t.Run("basic response", func(t *testing.T) {
		resp := &perplexityResponse{
			ID:    "resp-123",
			Model: "sonar",
			Choices: []perplexityChoice{
				{
					Index: 0,
					Message: &perplexityRespMsg{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: &perplexityUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		result, err := mapResponse(resp)
		if err != nil {
			t.Fatalf("mapResponse() error = %v", err)
		}

		if result.ID != "resp-123" {
			t.Errorf("ID = %q, want %q", result.ID, "resp-123")
		}
		if result.Model != "sonar" {
			t.Errorf("Model = %q, want %q", result.Model, "sonar")
		}
		if result.Output != "Hello! How can I help you?" {
			t.Errorf("Output = %q, want %q", result.Output, "Hello! How can I help you?")
		}
		if result.Usage.PromptTokens != 10 {
			t.Errorf("PromptTokens = %d, want 10", result.Usage.PromptTokens)
		}
		if result.Usage.CompletionTokens != 20 {
			t.Errorf("CompletionTokens = %d, want 20", result.Usage.CompletionTokens)
		}
		if result.Usage.TotalTokens != 30 {
			t.Errorf("TotalTokens = %d, want 30", result.Usage.TotalTokens)
		}
	})

	t.Run("with tool calls", func(t *testing.T) {
		resp := &perplexityResponse{
			ID:    "resp-456",
			Model: "sonar",
			Choices: []perplexityChoice{
				{
					Index: 0,
					Message: &perplexityRespMsg{
						Role:    "assistant",
						Content: "",
						ToolCalls: []perplexityToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: perplexityFunctionCall{
									Name:      "get_weather",
									Arguments: `{"city": "Tokyo"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		result, err := mapResponse(resp)
		if err != nil {
			t.Fatalf("mapResponse() error = %v", err)
		}

		if len(result.ToolCalls) != 1 {
			t.Fatalf("ToolCalls count = %d, want 1", len(result.ToolCalls))
		}
		if result.ToolCalls[0].ID != "call_1" {
			t.Errorf("ToolCall.ID = %q, want %q", result.ToolCalls[0].ID, "call_1")
		}
		if result.ToolCalls[0].Name != "get_weather" {
			t.Errorf("ToolCall.Name = %q, want %q", result.ToolCalls[0].Name, "get_weather")
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		resp := &perplexityResponse{
			ID:      "resp-789",
			Model:   "sonar",
			Choices: []perplexityChoice{},
		}

		result, err := mapResponse(resp)
		if err != nil {
			t.Fatalf("mapResponse() error = %v", err)
		}

		if result.Output != "" {
			t.Errorf("Output should be empty, got %q", result.Output)
		}
	})

	t.Run("no usage", func(t *testing.T) {
		resp := &perplexityResponse{
			ID:    "resp-abc",
			Model: "sonar",
			Choices: []perplexityChoice{
				{Message: &perplexityRespMsg{Content: "Hi"}},
			},
			Usage: nil,
		}

		result, err := mapResponse(resp)
		if err != nil {
			t.Fatalf("mapResponse() error = %v", err)
		}

		if result.Usage.TotalTokens != 0 {
			t.Errorf("Usage should be zero, got %+v", result.Usage)
		}
	})
}

func TestMapToolCalls(t *testing.T) {
	t.Run("valid tool calls", func(t *testing.T) {
		calls := []perplexityToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: perplexityFunctionCall{
					Name:      "get_weather",
					Arguments: `{"city": "NYC"}`,
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: perplexityFunctionCall{
					Name:      "search",
					Arguments: `{"query": "news"}`,
				},
			},
		}

		result, err := mapToolCalls(calls)
		if err != nil {
			t.Fatalf("mapToolCalls() error = %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("Result length = %d, want 2", len(result))
		}

		if result[0].ID != "call_1" {
			t.Errorf("result[0].ID = %q, want %q", result[0].ID, "call_1")
		}
		if result[0].Name != "get_weather" {
			t.Errorf("result[0].Name = %q, want %q", result[0].Name, "get_weather")
		}

		// Verify arguments are valid JSON
		var args map[string]string
		if err := json.Unmarshal(result[0].Arguments, &args); err != nil {
			t.Errorf("Failed to unmarshal arguments: %v", err)
		}
		if args["city"] != "NYC" {
			t.Errorf("args[city] = %q, want %q", args["city"], "NYC")
		}
	})

	t.Run("invalid JSON arguments", func(t *testing.T) {
		calls := []perplexityToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: perplexityFunctionCall{
					Name:      "test",
					Arguments: `{invalid json}`,
				},
			},
		}

		_, err := mapToolCalls(calls)
		if err == nil {
			t.Fatal("mapToolCalls() should return error for invalid JSON")
		}
		if err != ErrToolArgsInvalidJSON {
			t.Errorf("err = %v, want ErrToolArgsInvalidJSON", err)
		}
	})
}

// mockTool is a simple tool implementation for testing.
type mockTool struct {
	name        string
	description string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }
