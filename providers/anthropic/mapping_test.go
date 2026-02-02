package anthropic

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

func TestMapMessages(t *testing.T) {
	tests := []struct {
		name          string
		messages      []core.Message
		wantSystem    string
		wantMsgCount  int
		wantFirstRole string
		wantFirstText string
	}{
		{
			name: "user message only",
			messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
			wantSystem:    "",
			wantMsgCount:  1,
			wantFirstRole: "user",
			wantFirstText: "Hello",
		},
		{
			name: "system and user",
			messages: []core.Message{
				{Role: core.RoleSystem, Content: "You are helpful"},
				{Role: core.RoleUser, Content: "Hello"},
			},
			wantSystem:    "You are helpful",
			wantMsgCount:  1,
			wantFirstRole: "user",
			wantFirstText: "Hello",
		},
		{
			name: "multiple system messages",
			messages: []core.Message{
				{Role: core.RoleSystem, Content: "Be concise"},
				{Role: core.RoleSystem, Content: "Be helpful"},
				{Role: core.RoleUser, Content: "Hi"},
			},
			wantSystem:    "Be concise\n\nBe helpful",
			wantMsgCount:  1,
			wantFirstRole: "user",
			wantFirstText: "Hi",
		},
		{
			name: "conversation",
			messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
				{Role: core.RoleAssistant, Content: "Hi there!"},
				{Role: core.RoleUser, Content: "How are you?"},
			},
			wantSystem:   "",
			wantMsgCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, messages := mapMessages(tt.messages)

			if system != tt.wantSystem {
				t.Errorf("system = %q, want %q", system, tt.wantSystem)
			}

			if len(messages) != tt.wantMsgCount {
				t.Errorf("message count = %d, want %d", len(messages), tt.wantMsgCount)
			}

			if tt.wantMsgCount > 0 && tt.wantFirstRole != "" {
				if messages[0].Role != tt.wantFirstRole {
					t.Errorf("first message role = %q, want %q", messages[0].Role, tt.wantFirstRole)
				}
			}

			if tt.wantMsgCount > 0 && tt.wantFirstText != "" {
				if len(messages[0].Content) == 0 {
					t.Fatal("first message has no content blocks")
				}
				if messages[0].Content[0].Text != tt.wantFirstText {
					t.Errorf("first message text = %q, want %q", messages[0].Content[0].Text, tt.wantFirstText)
				}
			}
		})
	}
}

func TestBuildRequest(t *testing.T) {
	temp := float32(0.7)
	maxTokens := 500

	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleSystem, Content: "Be helpful"},
			{Role: core.RoleUser, Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	antReq := buildRequest(req, false)

	if antReq.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want 'claude-sonnet-4-5'", antReq.Model)
	}

	if antReq.System != "Be helpful" {
		t.Errorf("System = %q, want 'Be helpful'", antReq.System)
	}

	if antReq.MaxTokens != 500 {
		t.Errorf("MaxTokens = %d, want 500", antReq.MaxTokens)
	}

	if antReq.Temperature == nil || *antReq.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", antReq.Temperature)
	}

	if antReq.Stream {
		t.Error("Stream should be false")
	}

	if len(antReq.Messages) != 1 {
		t.Errorf("message count = %d, want 1", len(antReq.Messages))
	}
}

func TestBuildRequestDefaultMaxTokens(t *testing.T) {
	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	antReq := buildRequest(req, false)

	if antReq.MaxTokens != defaultMaxTokens {
		t.Errorf("MaxTokens = %d, want %d", antReq.MaxTokens, defaultMaxTokens)
	}
}

func TestBuildRequestStream(t *testing.T) {
	req := &core.ChatRequest{
		Model: "claude-sonnet-4-5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	antReq := buildRequest(req, true)

	if !antReq.Stream {
		t.Error("Stream should be true")
	}
}

// mockTool implements core.Tool for testing
type mockTool struct {
	name        string
	description string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }

// mockToolWithSchema implements core.Tool and schemaProvider
type mockToolWithSchema struct {
	name        string
	description string
	schema      tools.ToolSchema
}

func (t *mockToolWithSchema) Name() string             { return t.name }
func (t *mockToolWithSchema) Description() string      { return t.description }
func (t *mockToolWithSchema) Schema() tools.ToolSchema { return t.schema }

func TestMapTools(t *testing.T) {
	tests := []struct {
		name      string
		tools     []core.Tool
		wantCount int
	}{
		{
			name:      "nil tools",
			tools:     nil,
			wantCount: 0,
		},
		{
			name:      "empty tools",
			tools:     []core.Tool{},
			wantCount: 0,
		},
		{
			name: "single tool without schema",
			tools: []core.Tool{
				&mockTool{name: "get_weather", description: "Get weather"},
			},
			wantCount: 1,
		},
		{
			name: "single tool with schema",
			tools: []core.Tool{
				&mockToolWithSchema{
					name:        "get_weather",
					description: "Get weather",
					schema: tools.ToolSchema{
						JSONSchema: json.RawMessage(`{"type":"object"}`),
					},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapTools(tt.tools)

			if len(result) != tt.wantCount {
				t.Errorf("tool count = %d, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestMapToolsSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)
	tool := &mockToolWithSchema{
		name:        "get_weather",
		description: "Get weather",
		schema:      tools.ToolSchema{JSONSchema: schema},
	}

	result := mapTools([]core.Tool{tool})

	if len(result) != 1 {
		t.Fatalf("tool count = %d, want 1", len(result))
	}

	if result[0].Name != "get_weather" {
		t.Errorf("Name = %q, want 'get_weather'", result[0].Name)
	}

	if result[0].Description != "Get weather" {
		t.Errorf("Description = %q, want 'Get weather'", result[0].Description)
	}

	if !reflect.DeepEqual(result[0].InputSchema, schema) {
		t.Errorf("InputSchema = %s, want %s", result[0].InputSchema, schema)
	}
}

func TestMapResponse(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_123",
		Model: "claude-sonnet-4-5",
		Content: []anthropicResponseContent{
			{Type: "text", Text: "Hello there!"},
		},
		StopReason: "end_turn",
		Usage: anthropicUsage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	result, err := mapResponse(resp)
	if err != nil {
		t.Fatalf("mapResponse() error = %v", err)
	}

	if result.ID != "msg_123" {
		t.Errorf("ID = %q, want 'msg_123'", result.ID)
	}

	if result.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want 'claude-sonnet-4-5'", result.Model)
	}

	if result.Output != "Hello there!" {
		t.Errorf("Output = %q, want 'Hello there!'", result.Output)
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}

	if result.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", result.Usage.CompletionTokens)
	}

	if result.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", result.Usage.TotalTokens)
	}
}

func TestMapResponseWithToolCalls(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_456",
		Model: "claude-sonnet-4-5",
		Content: []anthropicResponseContent{
			{Type: "text", Text: "Let me check that."},
			{
				Type:  "tool_use",
				ID:    "tool_123",
				Name:  "get_weather",
				Input: json.RawMessage(`{"location":"NYC"}`),
			},
		},
		StopReason: "tool_use",
		Usage: anthropicUsage{
			InputTokens:  20,
			OutputTokens: 15,
		},
	}

	result, err := mapResponse(resp)
	if err != nil {
		t.Fatalf("mapResponse() error = %v", err)
	}

	if result.Output != "Let me check that." {
		t.Errorf("Output = %q, want 'Let me check that.'", result.Output)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "tool_123" {
		t.Errorf("ToolCall ID = %q, want 'tool_123'", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want 'get_weather'", tc.Name)
	}
	if string(tc.Arguments) != `{"location":"NYC"}` {
		t.Errorf("ToolCall Arguments = %s, want '{\"location\":\"NYC\"}'", tc.Arguments)
	}
}

func TestMapResponseInvalidToolJSON(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_789",
		Model: "claude-sonnet-4-5",
		Content: []anthropicResponseContent{
			{
				Type:  "tool_use",
				ID:    "tool_123",
				Name:  "get_weather",
				Input: json.RawMessage(`{invalid json`),
			},
		},
	}

	_, err := mapResponse(resp)
	if err == nil {
		t.Fatal("mapResponse() should return error for invalid JSON")
	}

	if err != ErrToolArgsInvalidJSON {
		t.Errorf("error = %v, want ErrToolArgsInvalidJSON", err)
	}
}

func TestMapResponseMultipleTextBlocks(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_multi",
		Model: "claude-sonnet-4-5",
		Content: []anthropicResponseContent{
			{Type: "text", Text: "First "},
			{Type: "text", Text: "Second"},
		},
	}

	result, err := mapResponse(resp)
	if err != nil {
		t.Fatalf("mapResponse() error = %v", err)
	}

	if result.Output != "First Second" {
		t.Errorf("Output = %q, want 'First Second'", result.Output)
	}
}
