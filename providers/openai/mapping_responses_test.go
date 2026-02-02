package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/tools"
)

// mockTool is a test implementation of the tools.Tool interface.
type mockTool struct {
	name        string
	description string
	schema      tools.ToolSchema
}

func (m *mockTool) Name() string             { return m.name }
func (m *mockTool) Description() string      { return m.description }
func (m *mockTool) Schema() tools.ToolSchema { return m.schema }
func (m *mockTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return nil, nil
}

func TestBuildResponsesRequest(t *testing.T) {
	temp := float32(0.7)
	maxTokens := 100

	req := &core.ChatRequest{
		Model:           ModelGPT52,
		Temperature:     &temp,
		MaxTokens:       &maxTokens,
		ReasoningEffort: core.ReasoningEffortHigh,
		Instructions:    "Be helpful",
		BuiltInTools: []core.BuiltInTool{
			{Type: "web_search"},
		},
		PreviousResponseID: "prev-123",
		Truncation:         "auto",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildResponsesRequest(req, false)

	if result.Model != "gpt-5.2" {
		t.Errorf("Model = %q, want %q", result.Model, "gpt-5.2")
	}

	if result.Instructions != "Be helpful" {
		t.Errorf("Instructions = %q, want %q", result.Instructions, "Be helpful")
	}

	if result.Temperature == nil || *result.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", result.Temperature)
	}

	if result.MaxOutputTokens == nil || *result.MaxOutputTokens != 100 {
		t.Errorf("MaxOutputTokens = %v, want 100", result.MaxOutputTokens)
	}

	if result.Reasoning == nil || result.Reasoning.Effort != "high" {
		t.Errorf("Reasoning.Effort = %v, want high", result.Reasoning)
	}

	if result.PreviousResponseID != "prev-123" {
		t.Errorf("PreviousResponseID = %q, want %q", result.PreviousResponseID, "prev-123")
	}

	if result.Truncation != "auto" {
		t.Errorf("Truncation = %q, want %q", result.Truncation, "auto")
	}

	if result.Stream {
		t.Error("Stream should be false")
	}

	// Check tools
	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}

	if result.Tools[0].Type != "web_search" {
		t.Errorf("Tools[0].Type = %q, want %q", result.Tools[0].Type, "web_search")
	}
}

func TestBuildResponsesRequestStreaming(t *testing.T) {
	req := &core.ChatRequest{
		Model: ModelGPT52,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildResponsesRequest(req, true)

	if !result.Stream {
		t.Error("Stream should be true")
	}

	if result.StreamOptions == nil || !result.StreamOptions.IncludeUsage {
		t.Error("StreamOptions.IncludeUsage should be true")
	}
}

func TestBuildResponsesInputSimpleText(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleUser, Content: "Hello"},
	}

	result := buildResponsesInput(msgs, "")

	if result.Text != "Hello" {
		t.Errorf("Text = %q, want %q", result.Text, "Hello")
	}

	// Marshal and check it becomes a string
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if string(data) != `"Hello"` {
		t.Errorf("Marshaled = %s, want \"Hello\"", data)
	}
}

func TestBuildResponsesInputMessages(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "You are helpful"},
		{Role: core.RoleUser, Content: "Hello"},
		{Role: core.RoleAssistant, Content: "Hi there"},
	}

	result := buildResponsesInput(msgs, "")

	if result.Text != "" {
		t.Error("Text should be empty for multiple messages")
	}

	if len(result.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(result.Messages))
	}

	// System role becomes "developer" in Responses API
	if result.Messages[0].Role != "developer" {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, "developer")
	}

	if result.Messages[1].Role != "user" {
		t.Errorf("Messages[1].Role = %q, want %q", result.Messages[1].Role, "user")
	}

	if result.Messages[2].Role != "assistant" {
		t.Errorf("Messages[2].Role = %q, want %q", result.Messages[2].Role, "assistant")
	}
}

func TestBuildResponsesInputWithInstructions(t *testing.T) {
	// When instructions are provided, system messages should be filtered
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "Old system message"},
		{Role: core.RoleUser, Content: "Hello"},
	}

	result := buildResponsesInput(msgs, "New instructions")

	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1 (system should be filtered)", len(result.Messages))
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, "user")
	}
}

func TestMapResponsesToolsBuiltIn(t *testing.T) {
	builtIn := []core.BuiltInTool{
		{Type: "web_search"},
		{Type: "code_interpreter"},
	}

	result := mapResponsesTools(nil, builtIn)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	if result[0].Type != "web_search" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "web_search")
	}

	if result[1].Type != "code_interpreter" {
		t.Errorf("result[1].Type = %q, want %q", result[1].Type, "code_interpreter")
	}
}

func TestMapResponsesToolsCustom(t *testing.T) {
	customTool := &mockTool{
		name:        "test_func",
		description: "A test function",
		schema: tools.ToolSchema{
			JSONSchema: json.RawMessage(`{"type":"object","properties":{"param1":{"type":"string"}}}`),
		},
	}

	result := mapResponsesTools([]core.Tool{customTool}, nil)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Type != "function" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "function")
	}

	if result[0].Name != "test_func" {
		t.Errorf("result[0].Name = %q, want %q", result[0].Name, "test_func")
	}

	if result[0].Description != "A test function" {
		t.Errorf("result[0].Description = %q, want %q", result[0].Description, "A test function")
	}
}

func TestMapResponsesToolsMixed(t *testing.T) {
	customTool := &mockTool{
		name:        "my_func",
		description: "My function",
		schema: tools.ToolSchema{
			JSONSchema: json.RawMessage(`{}`),
		},
	}

	builtIn := []core.BuiltInTool{
		{Type: "web_search"},
	}

	result := mapResponsesTools([]core.Tool{customTool}, builtIn)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	// Built-in tools come first
	if result[0].Type != "web_search" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "web_search")
	}

	// Custom tools come second
	if result[1].Type != "function" {
		t.Errorf("result[1].Type = %q, want %q", result[1].Type, "function")
	}
}

func TestMapResponsesResponseBasic(t *testing.T) {
	resp := &responsesResponse{
		ID:         "resp-123",
		Model:      "gpt-5.2",
		Status:     "completed",
		OutputText: "Hello world!",
		Usage: &responsesUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}

	result, err := mapResponsesResponse(resp)
	if err != nil {
		t.Fatalf("mapResponsesResponse() error = %v", err)
	}

	if result.ID != "resp-123" {
		t.Errorf("ID = %q, want %q", result.ID, "resp-123")
	}

	if result.Model != "gpt-5.2" {
		t.Errorf("Model = %q, want %q", result.Model, "gpt-5.2")
	}

	if result.Status != "completed" {
		t.Errorf("Status = %q, want %q", result.Status, "completed")
	}

	if result.Output != "Hello world!" {
		t.Errorf("Output = %q, want %q", result.Output, "Hello world!")
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("Usage.PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}
}

func TestMapResponsesResponseWithReasoning(t *testing.T) {
	resp := &responsesResponse{
		ID:         "resp-reason",
		Model:      "gpt-5.2",
		Status:     "completed",
		OutputText: "The answer is 42.",
		Output: []responsesOutput{
			{
				Type: "reasoning",
				ID:   "rs_123",
				Summary: []responsesReasoningSummary{
					{Type: "text", Text: "First thought"},
					{Type: "text", Text: "Second thought"},
				},
			},
			{
				Type: "message",
				Role: "assistant",
			},
		},
	}

	result, err := mapResponsesResponse(resp)
	if err != nil {
		t.Fatalf("mapResponsesResponse() error = %v", err)
	}

	if result.Reasoning == nil {
		t.Fatal("Expected reasoning output")
	}

	if len(result.Reasoning.Summary) != 2 {
		t.Fatalf("len(Reasoning.Summary) = %d, want 2", len(result.Reasoning.Summary))
	}

	if result.Reasoning.Summary[0] != "First thought" {
		t.Errorf("Reasoning.Summary[0] = %q, want %q", result.Reasoning.Summary[0], "First thought")
	}
}

func TestMapResponsesResponseWithToolCalls(t *testing.T) {
	resp := &responsesResponse{
		ID:     "resp-tool",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []responsesOutput{
			{
				Type:      "function_call",
				CallID:    "call_abc",
				Name:      "get_weather",
				Arguments: `{"location":"NYC"}`,
			},
		},
	}

	result, err := mapResponsesResponse(resp)
	if err != nil {
		t.Fatalf("mapResponsesResponse() error = %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", tc.ID, "call_abc")
	}

	if tc.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", tc.Name, "get_weather")
	}

	expected := `{"location":"NYC"}`
	if string(tc.Arguments) != expected {
		t.Errorf("ToolCalls[0].Arguments = %s, want %s", tc.Arguments, expected)
	}
}

func TestMapResponsesResponseInvalidToolArgs(t *testing.T) {
	resp := &responsesResponse{
		ID:     "resp-bad-tool",
		Model:  "gpt-5.2",
		Status: "completed",
		Output: []responsesOutput{
			{
				Type:      "function_call",
				CallID:    "call_bad",
				Name:      "broken",
				Arguments: `{invalid json`,
			},
		},
	}

	_, err := mapResponsesResponse(resp)
	if err != ErrToolArgsInvalidJSON {
		t.Errorf("expected ErrToolArgsInvalidJSON, got %v", err)
	}
}

func TestResponsesToolInputMarshalText(t *testing.T) {
	input := responsesInput{Text: "Hello world"}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	expected := `"Hello world"`
	if string(data) != expected {
		t.Errorf("Marshaled = %s, want %s", data, expected)
	}
}

func TestResponsesToolInputMarshalMessages(t *testing.T) {
	input := responsesInput{
		Messages: []responsesInputMessage{
			{Role: "user", Content: responsesContent{Text: "Hi"}},
			{Role: "assistant", Content: responsesContent{Text: "Hello"}},
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var result []responsesInputMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	if result[0].Role != "user" {
		t.Errorf("result[0].Role = %q, want %q", result[0].Role, "user")
	}
}

func TestBuildResponsesRequestWithToolResources(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4.1-mini",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Search my files"},
		},
		BuiltInTools: []core.BuiltInTool{{Type: "file_search"}},
		ToolResources: &core.ToolResources{
			FileSearch: &core.FileSearchResources{
				VectorStoreIDs: []string{"vs_abc123", "vs_def456"},
			},
		},
	}

	result := buildResponsesRequest(req, false)

	if result.ToolResources == nil {
		t.Fatal("expected ToolResources to be set")
	}
	if result.ToolResources.FileSearch == nil {
		t.Fatal("expected FileSearch to be set")
	}
	if len(result.ToolResources.FileSearch.VectorStoreIDs) != 2 {
		t.Errorf("expected 2 vector store IDs, got %d", len(result.ToolResources.FileSearch.VectorStoreIDs))
	}
	if result.ToolResources.FileSearch.VectorStoreIDs[0] != "vs_abc123" {
		t.Errorf("expected vs_abc123, got %s", result.ToolResources.FileSearch.VectorStoreIDs[0])
	}
}

func TestBuildResponsesRequestWithoutToolResources(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4.1-mini",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	result := buildResponsesRequest(req, false)

	if result.ToolResources != nil {
		t.Error("expected ToolResources to be nil")
	}
}

func TestBuildResponsesInputBackwardCompatibility(t *testing.T) {
	// Simple text-only message should use the text format (not array)
	msgs := []core.Message{
		{Role: core.RoleUser, Content: "Hello"},
	}

	input := buildResponsesInput(msgs, "")
	got, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Should be simple string, not array
	want := `"Hello"`
	if string(got) != want {
		t.Errorf("buildResponsesInput = %s, want %s", got, want)
	}
}

func TestBuildResponsesInputMultimodal(t *testing.T) {
	tests := []struct {
		name         string
		messages     []core.Message
		instructions string
		wantJSON     string
	}{
		{
			name: "simple text unchanged",
			messages: []core.Message{
				{Role: core.RoleUser, Content: "Hello"},
			},
			wantJSON: `"Hello"`,
		},
		{
			name: "multimodal with image URL",
			messages: []core.Message{
				{
					Role: core.RoleUser,
					Parts: []core.ContentPart{
						&core.InputText{Text: "What's in this image?"},
						&core.InputImage{ImageURL: "https://example.com/cat.jpg"},
					},
				},
			},
			wantJSON: `[{"role":"user","content":[{"type":"input_text","text":"What's in this image?"},{"type":"input_image","image_url":"https://example.com/cat.jpg"}]}]`,
		},
		{
			name: "multimodal with file_id",
			messages: []core.Message{
				{
					Role: core.RoleUser,
					Parts: []core.ContentPart{
						&core.InputText{Text: "Analyze this document"},
						&core.InputFile{FileID: "file-abc123"},
					},
				},
			},
			wantJSON: `[{"role":"user","content":[{"type":"input_text","text":"Analyze this document"},{"type":"input_file","file_id":"file-abc123"}]}]`,
		},
		{
			name: "image with detail",
			messages: []core.Message{
				{
					Role: core.RoleUser,
					Parts: []core.ContentPart{
						&core.InputImage{
							ImageURL: "https://example.com/cat.jpg",
							Detail:   core.ImageDetailHigh,
						},
					},
				},
			},
			wantJSON: `[{"role":"user","content":[{"type":"input_image","image_url":"https://example.com/cat.jpg","detail":"high"}]}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := buildResponsesInput(tt.messages, tt.instructions)
			got, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("got %s, want %s", got, tt.wantJSON)
			}
		})
	}
}
