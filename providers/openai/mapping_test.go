package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

func TestMapMessagesSystem(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "You are a helpful assistant."},
	}

	result := mapMessages(msgs)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("Role = %q, want %q", result[0].Role, "system")
	}

	if result[0].Content != "You are a helpful assistant." {
		t.Errorf("Content = %q, want %q", result[0].Content, "You are a helpful assistant.")
	}
}

func TestMapMessagesUser(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleUser, Content: "Hello!"},
	}

	result := mapMessages(msgs)

	if result[0].Role != "user" {
		t.Errorf("Role = %q, want %q", result[0].Role, "user")
	}
}

func TestMapMessagesAssistant(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleAssistant, Content: "Hi there!"},
	}

	result := mapMessages(msgs)

	if result[0].Role != "assistant" {
		t.Errorf("Role = %q, want %q", result[0].Role, "assistant")
	}
}

func TestMapMessagesMultiple(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "System prompt"},
		{Role: core.RoleUser, Content: "User message"},
		{Role: core.RoleAssistant, Content: "Assistant reply"},
	}

	result := mapMessages(msgs)

	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}

	expected := []struct {
		role    string
		content string
	}{
		{"system", "System prompt"},
		{"user", "User message"},
		{"assistant", "Assistant reply"},
	}

	for i, exp := range expected {
		if result[i].Role != exp.role {
			t.Errorf("result[%d].Role = %q, want %q", i, result[i].Role, exp.role)
		}
		if result[i].Content != exp.content {
			t.Errorf("result[%d].Content = %q, want %q", i, result[i].Content, exp.content)
		}
	}
}

func TestMapMessagesEmpty(t *testing.T) {
	result := mapMessages(nil)

	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

// mockFullTool implements both core.Tool and tools.Tool (with Schema)
type mockFullTool struct {
	name        string
	description string
	schema      tools.ToolSchema
}

func (m *mockFullTool) Name() string                                                { return m.name }
func (m *mockFullTool) Description() string                                         { return m.description }
func (m *mockFullTool) Schema() tools.ToolSchema                                    { return m.schema }
func (m *mockFullTool) Call(ctx context.Context, args json.RawMessage) (any, error) { return nil, nil }

// mockBasicTool implements only core.Tool (no Schema)
type mockBasicTool struct {
	name        string
	description string
}

func (m *mockBasicTool) Name() string        { return m.name }
func (m *mockBasicTool) Description() string { return m.description }

func TestMapToolsWithSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)
	tool := &mockFullTool{
		name:        "get_weather",
		description: "Get the weather for a location",
		schema:      tools.ToolSchema{JSONSchema: schema},
	}

	result := mapTools([]core.Tool{tool})

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Type != "function" {
		t.Errorf("Type = %q, want %q", result[0].Type, "function")
	}

	if result[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", result[0].Function.Name, "get_weather")
	}

	if result[0].Function.Description != "Get the weather for a location" {
		t.Errorf("Function.Description = %q, want %q", result[0].Function.Description, "Get the weather for a location")
	}

	if string(result[0].Function.Parameters) != string(schema) {
		t.Errorf("Function.Parameters = %s, want %s", result[0].Function.Parameters, schema)
	}
}

func TestMapToolsWithoutSchema(t *testing.T) {
	tool := &mockBasicTool{
		name:        "simple_tool",
		description: "A simple tool",
	}

	result := mapTools([]core.Tool{tool})

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// Should default to empty object
	if string(result[0].Function.Parameters) != "{}" {
		t.Errorf("Function.Parameters = %s, want {}", result[0].Function.Parameters)
	}
}

func TestMapToolsMultiple(t *testing.T) {
	tools := []core.Tool{
		&mockFullTool{
			name:        "tool1",
			description: "First tool",
			schema:      tools.ToolSchema{JSONSchema: json.RawMessage(`{"type":"object"}`)},
		},
		&mockFullTool{
			name:        "tool2",
			description: "Second tool",
			schema:      tools.ToolSchema{JSONSchema: json.RawMessage(`{"type":"string"}`)},
		},
	}

	result := mapTools(tools)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	if result[0].Function.Name != "tool1" {
		t.Errorf("result[0].Function.Name = %q, want %q", result[0].Function.Name, "tool1")
	}

	if result[1].Function.Name != "tool2" {
		t.Errorf("result[1].Function.Name = %q, want %q", result[1].Function.Name, "tool2")
	}
}

func TestMapToolsEmpty(t *testing.T) {
	result := mapTools(nil)

	if result != nil {
		t.Errorf("mapTools(nil) = %v, want nil", result)
	}
}

func TestMapToolsSchemaPassedUnchanged(t *testing.T) {
	// Complex schema to verify it's passed through unchanged
	originalSchema := json.RawMessage(`{"type":"object","properties":{"lat":{"type":"number"},"lon":{"type":"number"}},"required":["lat","lon"]}`)
	tool := &mockFullTool{
		name:        "geo_tool",
		description: "Geo tool",
		schema:      tools.ToolSchema{JSONSchema: originalSchema},
	}

	result := mapTools([]core.Tool{tool})

	// Verify exact match (no reformatting)
	if string(result[0].Function.Parameters) != string(originalSchema) {
		t.Errorf("Schema was modified:\ngot:  %s\nwant: %s", result[0].Function.Parameters, originalSchema)
	}
}

func TestBuildRequestBasic(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildRequest(req, false)

	if result.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", result.Model, "gpt-4o")
	}

	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(result.Messages))
	}

	if result.Stream != false {
		t.Error("Stream = true, want false")
	}

	if result.Temperature != nil {
		t.Errorf("Temperature = %v, want nil", result.Temperature)
	}

	if result.MaxTokens != nil {
		t.Errorf("MaxTokens = %v, want nil", result.MaxTokens)
	}
}

func TestBuildRequestStreaming(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildRequest(req, true)

	if result.Stream != true {
		t.Error("Stream = false, want true")
	}
}

func TestBuildRequestWithTemperature(t *testing.T) {
	temp := float32(0.7)
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
		Temperature: &temp,
	}

	result := buildRequest(req, false)

	if result.Temperature == nil {
		t.Fatal("Temperature = nil, want non-nil")
	}

	if *result.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", *result.Temperature)
	}
}

func TestBuildRequestWithMaxTokens(t *testing.T) {
	maxTokens := 100
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
		MaxTokens: &maxTokens,
	}

	result := buildRequest(req, false)

	if result.MaxTokens == nil {
		t.Fatal("MaxTokens = nil, want non-nil")
	}

	if *result.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", *result.MaxTokens)
	}
}

func TestBuildRequestWithTools(t *testing.T) {
	tool := &mockFullTool{
		name:        "my_tool",
		description: "My tool",
		schema:      tools.ToolSchema{JSONSchema: json.RawMessage(`{}`)},
	}

	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
		Tools: []core.Tool{tool},
	}

	result := buildRequest(req, false)

	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}

	if result.ToolChoice != "auto" {
		t.Errorf("ToolChoice = %q, want %q", result.ToolChoice, "auto")
	}
}

func TestBuildRequestWithoutTools(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildRequest(req, false)

	if len(result.Tools) != 0 {
		t.Errorf("len(Tools) = %d, want 0", len(result.Tools))
	}

	if result.ToolChoice != "" {
		t.Errorf("ToolChoice = %q, want empty", result.ToolChoice)
	}
}

func TestBuildRequestJSONOutput(t *testing.T) {
	temp := float32(0.5)
	maxTokens := 50
	tool := &mockFullTool{
		name:        "test_func",
		description: "Test function",
		schema:      tools.ToolSchema{JSONSchema: json.RawMessage(`{"type":"object"}`)},
	}

	req := &core.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []core.Message{
			{Role: core.RoleSystem, Content: "System"},
			{Role: core.RoleUser, Content: "User"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Tools:       []core.Tool{tool},
	}

	result := buildRequest(req, true)

	// Marshal to JSON to verify structure
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// Unmarshal back to verify
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// Verify required fields
	if parsed["model"] != "gpt-4o-mini" {
		t.Errorf("JSON model = %v, want gpt-4o-mini", parsed["model"])
	}

	if parsed["stream"] != true {
		t.Errorf("JSON stream = %v, want true", parsed["stream"])
	}

	if parsed["tool_choice"] != "auto" {
		t.Errorf("JSON tool_choice = %v, want auto", parsed["tool_choice"])
	}

	messages, ok := parsed["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Errorf("JSON messages length = %v, want 2", len(messages))
	}

	toolsList, ok := parsed["tools"].([]any)
	if !ok || len(toolsList) != 1 {
		t.Errorf("JSON tools length = %v, want 1", len(toolsList))
	}
}
