package huggingface

import (
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
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
		toolsList := []core.Tool{
			&mockTool{name: "get_weather", description: "Get weather for a city"},
			&mockTool{name: "search", description: "Search the web"},
		}

		result := mapTools(toolsList)

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

	t.Run("with schema provider", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)
		toolsList := []core.Tool{
			&mockToolWithSchema{
				name:        "get_weather",
				description: "Get weather",
				schema:      tools.ToolSchema{JSONSchema: schema},
			},
		}

		result := mapTools(toolsList)

		if len(result) != 1 {
			t.Fatalf("mapTools() returned %d tools, want 1", len(result))
		}

		if string(result[0].Function.Parameters) != string(schema) {
			t.Errorf("Parameters = %s, want %s", result[0].Function.Parameters, schema)
		}
	})
}

func TestBuildRequest(t *testing.T) {
	t.Run("basic request", func(t *testing.T) {
		req := &core.ChatRequest{
			Model: "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
		}

		result := buildRequest(req, "meta-llama/Llama-3-8B-Instruct", false)

		if result.Model != "meta-llama/Llama-3-8B-Instruct" {
			t.Errorf("Model = %q, want %q", result.Model, "meta-llama/Llama-3-8B-Instruct")
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
			Model:    "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hello"}},
		}

		result := buildRequest(req, "meta-llama/Llama-3-8B-Instruct", true)

		if !result.Stream {
			t.Error("Stream should be true")
		}
	})

	t.Run("with temperature and max tokens", func(t *testing.T) {
		temp := float32(0.7)
		maxTokens := 100

		req := &core.ChatRequest{
			Model:       "meta-llama/Llama-3-8B-Instruct",
			Messages:    []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			Temperature: &temp,
			MaxTokens:   &maxTokens,
		}

		result := buildRequest(req, "meta-llama/Llama-3-8B-Instruct", false)

		if result.Temperature == nil || *result.Temperature != 0.7 {
			t.Errorf("Temperature = %v, want 0.7", result.Temperature)
		}
		if result.MaxTokens == nil || *result.MaxTokens != 100 {
			t.Errorf("MaxTokens = %v, want 100", result.MaxTokens)
		}
	})

	t.Run("with tools", func(t *testing.T) {
		req := &core.ChatRequest{
			Model:    "meta-llama/Llama-3-8B-Instruct",
			Messages: []core.Message{{Role: core.RoleUser, Content: "Hello"}},
			Tools: []core.Tool{
				&mockTool{name: "test_tool", description: "A test tool"},
			},
		}

		result := buildRequest(req, "meta-llama/Llama-3-8B-Instruct", false)

		if len(result.Tools) != 1 {
			t.Fatalf("Tools count = %d, want 1", len(result.Tools))
		}
		if result.ToolChoice != "auto" {
			t.Errorf("ToolChoice = %q, want %q", result.ToolChoice, "auto")
		}
	})
}

func TestMapResponse(t *testing.T) {
	t.Run("basic response", func(t *testing.T) {
		resp := &hfResponse{
			ID:    "resp-123",
			Model: "meta-llama/Llama-3-8B-Instruct",
			Choices: []hfChoice{
				{
					Index: 0,
					Message: hfRespMsg{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: hfUsage{
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
		if result.Model != "meta-llama/Llama-3-8B-Instruct" {
			t.Errorf("Model = %q, want %q", result.Model, "meta-llama/Llama-3-8B-Instruct")
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
		resp := &hfResponse{
			ID:    "resp-456",
			Model: "meta-llama/Llama-3-8B-Instruct",
			Choices: []hfChoice{
				{
					Index: 0,
					Message: hfRespMsg{
						Role:    "assistant",
						Content: "",
						ToolCalls: []hfToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: hfFunctionCall{
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
		resp := &hfResponse{
			ID:      "resp-789",
			Model:   "meta-llama/Llama-3-8B-Instruct",
			Choices: []hfChoice{},
		}

		result, err := mapResponse(resp)
		if err != nil {
			t.Fatalf("mapResponse() error = %v", err)
		}

		if result.Output != "" {
			t.Errorf("Output should be empty, got %q", result.Output)
		}
	})
}

func TestMapToolCalls(t *testing.T) {
	t.Run("valid tool calls", func(t *testing.T) {
		calls := []hfToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: hfFunctionCall{
					Name:      "get_weather",
					Arguments: `{"city": "NYC"}`,
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: hfFunctionCall{
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
		calls := []hfToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: hfFunctionCall{
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

// mockToolWithSchema is a tool that provides a schema.
type mockToolWithSchema struct {
	name        string
	description string
	schema      tools.ToolSchema
}

func (t *mockToolWithSchema) Name() string             { return t.name }
func (t *mockToolWithSchema) Description() string      { return t.description }
func (t *mockToolWithSchema) Schema() tools.ToolSchema { return t.schema }
