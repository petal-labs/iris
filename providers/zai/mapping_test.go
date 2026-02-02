package zai

import (
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

func TestMapThinking(t *testing.T) {
	tests := []struct {
		input   core.ReasoningEffort
		wantNil bool
		wantTyp string
	}{
		{core.ReasoningEffortNone, true, ""},
		{"", true, ""},
		{core.ReasoningEffortLow, false, "enabled"},
		{core.ReasoningEffortMedium, false, "enabled"},
		{core.ReasoningEffortHigh, false, "enabled"},
		{core.ReasoningEffortXHigh, false, "enabled"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := mapThinking(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("mapThinking(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("mapThinking(%q) = nil, want non-nil", tt.input)
			}
			if got.Type != tt.wantTyp {
				t.Errorf("mapThinking(%q).Type = %q, want %q", tt.input, got.Type, tt.wantTyp)
			}
		})
	}
}

func TestBuildRequest(t *testing.T) {
	temp := float32(0.7)
	maxTok := 100

	req := &core.ChatRequest{
		Model: "glm-4.7",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
		Temperature:     &temp,
		MaxTokens:       &maxTok,
		ReasoningEffort: core.ReasoningEffortHigh,
	}

	result := buildRequest(req, false)

	if result.Model != "glm-4.7" {
		t.Errorf("Model = %q, want glm-4.7", result.Model)
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

	if result.Thinking == nil {
		t.Error("Thinking should not be nil")
	} else if result.Thinking.Type != "enabled" {
		t.Errorf("Thinking.Type = %q, want enabled", result.Thinking.Type)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(result.Messages))
	}
}

func TestBuildRequestStream(t *testing.T) {
	req := &core.ChatRequest{
		Model: "glm-4.7",
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
		Model: "glm-4.7",
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

	if result.Thinking != nil {
		t.Error("Thinking should be nil")
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

// mockTool implements core.Tool for testing
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
