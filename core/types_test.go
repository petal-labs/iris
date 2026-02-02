package core

import (
	"encoding/json"
	"testing"
)

func TestMessageJSONMarshal(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{
			name: "system role",
			msg:  Message{Role: RoleSystem, Content: "You are a helpful assistant."},
			want: `{"role":"system","content":"You are a helpful assistant."}`,
		},
		{
			name: "user role",
			msg:  Message{Role: RoleUser, Content: "Hello"},
			want: `{"role":"user","content":"Hello"}`,
		},
		{
			name: "assistant role",
			msg:  Message{Role: RoleAssistant, Content: "Hi there!"},
			want: `{"role":"assistant","content":"Hi there!"}`,
		},
		{
			name: "empty content",
			msg:  Message{Role: RoleUser, Content: ""},
			want: `{"role":"user"}`, // Content omitted when empty (omitempty)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMessageJSONUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Message
		wantErr bool
	}{
		{
			name:  "system role",
			input: `{"role":"system","content":"You are helpful."}`,
			want:  Message{Role: RoleSystem, Content: "You are helpful."},
		},
		{
			name:  "user role",
			input: `{"role":"user","content":"Hello"}`,
			want:  Message{Role: RoleUser, Content: "Hello"},
		},
		{
			name:  "assistant role",
			input: `{"role":"assistant","content":"Hi!"}`,
			want:  Message{Role: RoleAssistant, Content: "Hi!"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Message
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Role != tt.want.Role || got.Content != tt.want.Content {
				t.Errorf("Unmarshal() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestChatRequestJSONMarshal(t *testing.T) {
	temp := float32(0.7)
	maxTok := 100

	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTok,
	}

	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Unmarshal to verify structure
	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", result["model"])
	}
	if result["temperature"] != float64(0.7) {
		t.Errorf("temperature = %v, want 0.7", result["temperature"])
	}
	if result["max_tokens"] != float64(100) {
		t.Errorf("max_tokens = %v, want 100", result["max_tokens"])
	}
}

func TestChatRequestOmitsNilFields(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
		// Temperature and MaxTokens are nil
	}

	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if _, ok := result["temperature"]; ok {
		t.Error("temperature should be omitted when nil")
	}
	if _, ok := result["max_tokens"]; ok {
		t.Error("max_tokens should be omitted when nil")
	}
}

func TestChatResponseJSONMarshal(t *testing.T) {
	resp := ChatResponse{
		ID:     "chatcmpl-123",
		Model:  "gpt-4",
		Output: "Hello! How can I help?",
		Usage: TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	got, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result ChatResponse
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result.ID != resp.ID {
		t.Errorf("ID = %v, want %v", result.ID, resp.ID)
	}
	if result.Output != resp.Output {
		t.Errorf("Output = %v, want %v", result.Output, resp.Output)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %v, want 30", result.Usage.TotalTokens)
	}
}

func TestToolCallPreservesRawJSON(t *testing.T) {
	// Raw JSON arguments - json.RawMessage preserves the data structure
	rawArgs := json.RawMessage(`{"key":"value","num":42}`)

	tc := ToolCall{
		ID:        "call_123",
		Name:      "get_weather",
		Arguments: rawArgs,
	}

	got, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result ToolCall
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Verify Arguments data is preserved (json.RawMessage maintains the JSON structure)
	var originalData, resultData map[string]any
	if err := json.Unmarshal(rawArgs, &originalData); err != nil {
		t.Fatalf("Unmarshal original args: %v", err)
	}
	if err := json.Unmarshal(result.Arguments, &resultData); err != nil {
		t.Fatalf("Unmarshal result args: %v", err)
	}

	if originalData["key"] != resultData["key"] {
		t.Errorf("key = %v, want %v", resultData["key"], originalData["key"])
	}
	if originalData["num"] != resultData["num"] {
		t.Errorf("num = %v, want %v", resultData["num"], originalData["num"])
	}
}

func TestToolCallArgumentsNotParsed(t *testing.T) {
	// Verify that Arguments remain as raw bytes and are not parsed into Go types
	rawArgs := json.RawMessage(`{"complex":{"nested":"value"},"array":[1,2,3]}`)

	tc := ToolCall{
		ID:        "call_456",
		Name:      "complex_tool",
		Arguments: rawArgs,
	}

	// Marshal and unmarshal
	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result ToolCall
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Verify it's still valid JSON that can be parsed
	var parsed map[string]any
	if err := json.Unmarshal(result.Arguments, &parsed); err != nil {
		t.Errorf("Arguments should be valid JSON: %v", err)
	}

	// Verify nested structure is preserved
	if nested, ok := parsed["complex"].(map[string]any); !ok {
		t.Error("nested object should be preserved")
	} else if nested["nested"] != "value" {
		t.Error("nested value should be preserved")
	}
}

func TestChatChunkJSONMarshal(t *testing.T) {
	chunk := ChatChunk{Delta: "Hello"}

	got, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	want := `{"delta":"Hello"}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestTokenUsageJSONRoundTrip(t *testing.T) {
	usage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result TokenUsage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result != usage {
		t.Errorf("RoundTrip = %+v, want %+v", result, usage)
	}
}

func TestMessageOrderingPreserved(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "System"},
		{Role: RoleUser, Content: "User 1"},
		{Role: RoleAssistant, Content: "Assistant 1"},
		{Role: RoleUser, Content: "User 2"},
	}

	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result []Message
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(result) != len(messages) {
		t.Fatalf("len(result) = %d, want %d", len(result), len(messages))
	}

	for i, msg := range result {
		if msg.Role != messages[i].Role || msg.Content != messages[i].Content {
			t.Errorf("messages[%d] = %+v, want %+v", i, msg, messages[i])
		}
	}
}

func TestToolResourcesJSONMarshal(t *testing.T) {
	tr := &ToolResources{
		FileSearch: &FileSearchResources{
			VectorStoreIDs: []string{"vs_abc123", "vs_def456"},
		},
	}

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	expected := `{"file_search":{"vector_store_ids":["vs_abc123","vs_def456"]}}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, data)
	}
}

func TestChatRequestWithToolResources(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4.1-mini",
		Messages: []Message{
			{Role: RoleUser, Content: "hello"},
		},
		BuiltInTools: []BuiltInTool{{Type: "file_search"}},
		ToolResources: &ToolResources{
			FileSearch: &FileSearchResources{
				VectorStoreIDs: []string{"vs_abc123"},
			},
		},
	}

	if req.ToolResources == nil {
		t.Error("expected ToolResources to be set")
	}
	if len(req.ToolResources.FileSearch.VectorStoreIDs) != 1 {
		t.Errorf("expected 1 vector store ID, got %d", len(req.ToolResources.FileSearch.VectorStoreIDs))
	}
}
