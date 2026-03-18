package azurefoundry

import (
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestMapMessages(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "You are helpful."},
		{Role: core.RoleUser, Content: "Hello"},
		{Role: core.RoleAssistant, Content: "Hi there!"},
	}

	result := mapMessages(msgs)

	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("result[0].Role = %q, want system", result[0].Role)
	}
	if result[0].Content != "You are helpful." {
		t.Errorf("result[0].Content = %q, want 'You are helpful.'", result[0].Content)
	}

	if result[1].Role != "user" {
		t.Errorf("result[1].Role = %q, want user", result[1].Role)
	}

	if result[2].Role != "assistant" {
		t.Errorf("result[2].Role = %q, want assistant", result[2].Role)
	}
}

func TestMapMessagesEmpty(t *testing.T) {
	result := mapMessages([]core.Message{})

	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestMapMessagesWithToolCalls(t *testing.T) {
	msgs := []core.Message{
		{
			Role:    core.RoleAssistant,
			Content: "",
			ToolCalls: []core.ToolCall{
				{
					ID:        "call_123",
					Name:      "get_weather",
					Arguments: json.RawMessage(`{"location":"NYC"}`),
				},
			},
		},
	}

	result := mapMessages(msgs)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("len(result[0].ToolCalls) = %d, want 1", len(result[0].ToolCalls))
	}

	tc := result[0].ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("ToolCalls[0].ID = %q, want call_123", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Function.Name = %q, want get_weather", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"location":"NYC"}` {
		t.Errorf("ToolCalls[0].Function.Arguments = %q, want {\"location\":\"NYC\"}", tc.Function.Arguments)
	}
}

func TestMapMessagesWithToolResults(t *testing.T) {
	msgs := []core.Message{
		{
			Role: core.RoleTool,
			ToolResults: []core.ToolResult{
				{
					CallID:  "call_123",
					Content: "Sunny, 72°F",
				},
				{
					CallID:  "call_456",
					Content: map[string]interface{}{"temp": 72, "condition": "sunny"},
				},
			},
		},
	}

	result := mapMessages(msgs)

	// Each tool result should become a separate message
	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	if result[0].Role != "tool" {
		t.Errorf("result[0].Role = %q, want tool", result[0].Role)
	}
	if result[0].ToolCallID != "call_123" {
		t.Errorf("result[0].ToolCallID = %q, want call_123", result[0].ToolCallID)
	}
	if result[0].Content != "Sunny, 72°F" {
		t.Errorf("result[0].Content = %q, want 'Sunny, 72°F'", result[0].Content)
	}

	if result[1].ToolCallID != "call_456" {
		t.Errorf("result[1].ToolCallID = %q, want call_456", result[1].ToolCallID)
	}
}

func TestBuildRequest(t *testing.T) {
	temp := float32(0.7)
	maxTok := 100

	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTok,
	}

	result := buildRequest(req, false)

	if result.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", result.Model)
	}

	if result.Stream {
		t.Error("Stream = true, want false")
	}

	if *result.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", *result.Temperature)
	}

	if *result.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want 100", *result.MaxTokens)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(result.Messages))
	}
}

func TestBuildRequestStream(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildRequest(req, true)

	if !result.Stream {
		t.Error("Stream = false, want true")
	}
}

func TestBuildRequestNoOptionalFields(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildRequest(req, false)

	if result.Temperature != nil {
		t.Error("Temperature should be nil")
	}

	if result.MaxTokens != nil {
		t.Error("MaxTokens should be nil")
	}

	if result.Tools != nil {
		t.Error("Tools should be nil")
	}
}

func TestMapToolsEmpty(t *testing.T) {
	result := mapTools(nil)
	if result != nil {
		t.Errorf("mapTools(nil) = %v, want nil", result)
	}

	result = mapTools([]core.Tool{})
	if result != nil {
		t.Errorf("mapTools([]) = %v, want nil", result)
	}
}

// mockTool implements core.Tool for testing.
type mockTool struct {
	name        string
	description string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }

func TestMapToolsBasic(t *testing.T) {
	tools := []core.Tool{
		&mockTool{name: "get_weather", description: "Get weather info"},
	}

	result := mapTools(tools)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Type != "function" {
		t.Errorf("Type = %q, want function", result[0].Type)
	}

	if result[0].Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want get_weather", result[0].Function.Name)
	}

	if result[0].Function.Description != "Get weather info" {
		t.Errorf("Function.Description = %q, want 'Get weather info'", result[0].Function.Description)
	}

	// Default empty schema
	if string(result[0].Function.Parameters) != "{}" {
		t.Errorf("Function.Parameters = %s, want {}", result[0].Function.Parameters)
	}
}

func TestMapResponseFormatText(t *testing.T) {
	req := &core.ChatRequest{
		ResponseFormat: core.ResponseFormatText,
	}

	result := mapResponseFormat(req)

	if result != nil {
		t.Errorf("mapResponseFormat(text) = %v, want nil", result)
	}
}

func TestMapResponseFormatJSON(t *testing.T) {
	req := &core.ChatRequest{
		ResponseFormat: core.ResponseFormatJSON,
	}

	result := mapResponseFormat(req)

	if result == nil {
		t.Fatal("mapResponseFormat(json) = nil, want non-nil")
	}

	if result.Type != "json_object" {
		t.Errorf("Type = %q, want json_object", result.Type)
	}
}

func TestMapResponseFormatJSONSchema(t *testing.T) {
	req := &core.ChatRequest{
		ResponseFormat: core.ResponseFormatJSONSchema,
		JSONSchema: &core.JSONSchemaDefinition{
			Name:        "my_schema",
			Description: "A test schema",
			Schema:      json.RawMessage(`{"type":"object"}`),
			Strict:      true,
		},
	}

	result := mapResponseFormat(req)

	if result == nil {
		t.Fatal("mapResponseFormat(json_schema) = nil, want non-nil")
	}

	if result.Type != "json_schema" {
		t.Errorf("Type = %q, want json_schema", result.Type)
	}

	if result.JSONSchema == nil {
		t.Fatal("JSONSchema is nil")
	}

	if result.JSONSchema.Name != "my_schema" {
		t.Errorf("JSONSchema.Name = %q, want my_schema", result.JSONSchema.Name)
	}

	if !result.JSONSchema.Strict {
		t.Error("JSONSchema.Strict = false, want true")
	}
}

func TestMapResponseFormatJSONSchemaNoSchema(t *testing.T) {
	req := &core.ChatRequest{
		ResponseFormat: core.ResponseFormatJSONSchema,
		JSONSchema:     nil,
	}

	result := mapResponseFormat(req)

	if result != nil {
		t.Errorf("mapResponseFormat(json_schema, nil) = %v, want nil", result)
	}
}

func TestMapUsage(t *testing.T) {
	usage := azureUsage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}

	result := mapUsage(usage)

	if result.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.PromptTokens)
	}
	if result.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", result.CompletionTokens)
	}
	if result.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", result.TotalTokens)
	}
}

func TestMapToolCallsFromAzureEmpty(t *testing.T) {
	result := mapToolCallsFromAzure(nil)
	if result != nil {
		t.Errorf("mapToolCallsFromAzure(nil) = %v, want nil", result)
	}

	result = mapToolCallsFromAzure([]azureToolCall{})
	if result != nil {
		t.Errorf("mapToolCallsFromAzure([]) = %v, want nil", result)
	}
}

func TestMapToolCallsFromAzure(t *testing.T) {
	calls := []azureToolCall{
		{
			ID:   "call_abc",
			Type: "function",
			Function: azureFunctionCall{
				Name:      "get_weather",
				Arguments: `{"location":"NYC"}`,
			},
		},
	}

	result := mapToolCallsFromAzure(calls)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].ID != "call_abc" {
		t.Errorf("ID = %q, want call_abc", result[0].ID)
	}
	if result[0].Name != "get_weather" {
		t.Errorf("Name = %q, want get_weather", result[0].Name)
	}
	if string(result[0].Arguments) != `{"location":"NYC"}` {
		t.Errorf("Arguments = %s, want {\"location\":\"NYC\"}", result[0].Arguments)
	}
}

func TestMapToolCallsToAzure(t *testing.T) {
	calls := []core.ToolCall{
		{
			ID:        "call_xyz",
			Name:      "send_email",
			Arguments: json.RawMessage(`{"to":"test@example.com"}`),
		},
	}

	result := mapToolCallsToAzure(calls)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].ID != "call_xyz" {
		t.Errorf("ID = %q, want call_xyz", result[0].ID)
	}
	if result[0].Type != "function" {
		t.Errorf("Type = %q, want function", result[0].Type)
	}
	if result[0].Function.Name != "send_email" {
		t.Errorf("Function.Name = %q, want send_email", result[0].Function.Name)
	}
	if result[0].Function.Arguments != `{"to":"test@example.com"}` {
		t.Errorf("Function.Arguments = %q, want {\"to\":\"test@example.com\"}", result[0].Function.Arguments)
	}
}

func TestMarshalToolResultContentString(t *testing.T) {
	result := marshalToolResultContent("plain text")
	if result != "plain text" {
		t.Errorf("marshalToolResultContent(string) = %q, want 'plain text'", result)
	}
}

func TestMarshalToolResultContentObject(t *testing.T) {
	obj := map[string]interface{}{"key": "value"}
	result := marshalToolResultContent(obj)

	if result != `{"key":"value"}` {
		t.Errorf("marshalToolResultContent(object) = %q, want {\"key\":\"value\"}", result)
	}
}

func TestContentFiltersIsFiltered(t *testing.T) {
	tests := []struct {
		name    string
		filters *azureContentFilters
		want    bool
	}{
		{
			name:    "nil filters",
			filters: nil,
			want:    false,
		},
		{
			name:    "empty filters",
			filters: &azureContentFilters{},
			want:    false,
		},
		{
			name: "hate filtered",
			filters: &azureContentFilters{
				Hate: &azureFilterSeverity{Filtered: true, Severity: "high"},
			},
			want: true,
		},
		{
			name: "violence not filtered",
			filters: &azureContentFilters{
				Violence: &azureFilterSeverity{Filtered: false, Severity: "low"},
			},
			want: false,
		},
		{
			name: "jailbreak filtered",
			filters: &azureContentFilters{
				Jailbreak: &azureFilterDetected{Filtered: true, Detected: true},
			},
			want: true,
		},
		{
			name: "protected material filtered",
			filters: &azureContentFilters{
				ProtectedMaterialCode: &azureFilterDetected{Filtered: true, Detected: true},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filters.IsFiltered()
			if got != tt.want {
				t.Errorf("IsFiltered() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapResponse(t *testing.T) {
	resp := &azureResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4o",
		Choices: []azureChoice{
			{
				Index: 0,
				Message: &azureRespMsg{
					Role:    "assistant",
					Content: "Hello there!",
				},
			},
		},
		Usage: azureUsage{
			PromptTokens:     5,
			CompletionTokens: 3,
			TotalTokens:      8,
		},
	}

	result := mapResponse(resp)

	if result.ID != "chatcmpl-123" {
		t.Errorf("ID = %q, want chatcmpl-123", result.ID)
	}
	if result.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", result.Model)
	}
	if result.Output != "Hello there!" {
		t.Errorf("Output = %q, want 'Hello there!'", result.Output)
	}
	if result.Usage.TotalTokens != 8 {
		t.Errorf("Usage.TotalTokens = %d, want 8", result.Usage.TotalTokens)
	}
}

func TestMapResponseEmpty(t *testing.T) {
	resp := &azureResponse{
		ID:      "chatcmpl-empty",
		Model:   "gpt-4o",
		Choices: []azureChoice{},
	}

	result := mapResponse(resp)

	if result.ID != "chatcmpl-empty" {
		t.Errorf("ID = %q, want chatcmpl-empty", result.ID)
	}
	if result.Output != "" {
		t.Errorf("Output = %q, want empty", result.Output)
	}
}

func TestMapResponseWithToolCalls(t *testing.T) {
	resp := &azureResponse{
		ID:    "chatcmpl-tools",
		Model: "gpt-4o",
		Choices: []azureChoice{
			{
				Index: 0,
				Message: &azureRespMsg{
					Role:    "assistant",
					Content: "",
					ToolCalls: []azureToolCall{
						{
							ID:   "call_abc",
							Type: "function",
							Function: azureFunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"NYC"}`,
							},
						},
					},
				},
			},
		},
	}

	result := mapResponse(resp)

	if len(result.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("ToolCalls[0].ID = %q, want call_abc", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %q, want get_weather", tc.Name)
	}
}
