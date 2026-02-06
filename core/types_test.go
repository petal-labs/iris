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

func TestChatResponseHasToolCalls(t *testing.T) {
	tests := []struct {
		name      string
		response  *ChatResponse
		wantValue bool
	}{
		{
			name:      "no tool calls",
			response:  &ChatResponse{Output: "Hello"},
			wantValue: false,
		},
		{
			name:      "empty tool calls slice",
			response:  &ChatResponse{Output: "Hello", ToolCalls: []ToolCall{}},
			wantValue: false,
		},
		{
			name: "with tool calls",
			response: &ChatResponse{
				Output: "Let me check that",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{}`)},
				},
			},
			wantValue: true,
		},
		{
			name: "multiple tool calls",
			response: &ChatResponse{
				Output: "Let me check both",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{}`)},
					{ID: "call_2", Name: "get_time", Arguments: json.RawMessage(`{}`)},
				},
			},
			wantValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.HasToolCalls()
			if got != tt.wantValue {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

func TestChatResponseFirstToolCall(t *testing.T) {
	tests := []struct {
		name     string
		response *ChatResponse
		wantNil  bool
		wantName string
	}{
		{
			name:     "no tool calls",
			response: &ChatResponse{Output: "Hello"},
			wantNil:  true,
		},
		{
			name:     "empty tool calls slice",
			response: &ChatResponse{Output: "Hello", ToolCalls: []ToolCall{}},
			wantNil:  true,
		},
		{
			name: "single tool call",
			response: &ChatResponse{
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{}`)},
				},
			},
			wantNil:  false,
			wantName: "get_weather",
		},
		{
			name: "multiple tool calls returns first",
			response: &ChatResponse{
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "first_tool", Arguments: json.RawMessage(`{}`)},
					{ID: "call_2", Name: "second_tool", Arguments: json.RawMessage(`{}`)},
				},
			},
			wantNil:  false,
			wantName: "first_tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.FirstToolCall()
			if tt.wantNil {
				if got != nil {
					t.Errorf("FirstToolCall() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Fatal("FirstToolCall() = nil, want non-nil")
				}
				if got.Name != tt.wantName {
					t.Errorf("FirstToolCall().Name = %v, want %v", got.Name, tt.wantName)
				}
			}
		})
	}
}

func TestChatResponseHasReasoning(t *testing.T) {
	tests := []struct {
		name      string
		response  *ChatResponse
		wantValue bool
	}{
		{
			name:      "no reasoning",
			response:  &ChatResponse{Output: "Hello"},
			wantValue: false,
		},
		{
			name:      "nil reasoning",
			response:  &ChatResponse{Output: "Hello", Reasoning: nil},
			wantValue: false,
		},
		{
			name: "empty reasoning summary",
			response: &ChatResponse{
				Output:    "Hello",
				Reasoning: &ReasoningOutput{ID: "r1", Summary: []string{}},
			},
			wantValue: false,
		},
		{
			name: "with reasoning summary",
			response: &ChatResponse{
				Output: "Hello",
				Reasoning: &ReasoningOutput{
					ID:      "r1",
					Summary: []string{"Thinking about the question..."},
				},
			},
			wantValue: true,
		},
		{
			name: "multiple reasoning summaries",
			response: &ChatResponse{
				Output: "Hello",
				Reasoning: &ReasoningOutput{
					ID:      "r1",
					Summary: []string{"Step 1", "Step 2", "Conclusion"},
				},
			},
			wantValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.HasReasoning()
			if got != tt.wantValue {
				t.Errorf("HasReasoning() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

// Tool Result Tests

func TestToolResultJSONMarshal(t *testing.T) {
	tests := []struct {
		name string
		tr   ToolResult
	}{
		{
			name: "string content",
			tr:   ToolResult{CallID: "call_1", Content: "sunny, 72F", IsError: false},
		},
		{
			name: "error result",
			tr:   ToolResult{CallID: "call_2", Content: "connection timeout", IsError: true},
		},
		{
			name: "struct content",
			tr: ToolResult{
				CallID:  "call_3",
				Content: map[string]any{"temp": 72, "conditions": "sunny"},
				IsError: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.tr)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var result ToolResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if result.CallID != tt.tr.CallID {
				t.Errorf("CallID = %v, want %v", result.CallID, tt.tr.CallID)
			}
			if result.IsError != tt.tr.IsError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.tr.IsError)
			}
		})
	}
}

func TestToolResultBuilderSuccess(t *testing.T) {
	builder := NewToolResults()
	results := builder.
		Success("call_1", "result 1").
		Success("call_2", map[string]any{"key": "value"}).
		Build()

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	if results[0].CallID != "call_1" {
		t.Errorf("results[0].CallID = %v, want call_1", results[0].CallID)
	}
	if results[0].IsError {
		t.Error("results[0].IsError = true, want false")
	}
	if results[0].Content != "result 1" {
		t.Errorf("results[0].Content = %v, want 'result 1'", results[0].Content)
	}

	if results[1].CallID != "call_2" {
		t.Errorf("results[1].CallID = %v, want call_2", results[1].CallID)
	}
}

func TestToolResultBuilderError(t *testing.T) {
	builder := NewToolResults()
	testErr := errForTest("connection failed")

	results := builder.
		Error("call_1", testErr).
		Build()

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	if results[0].CallID != "call_1" {
		t.Errorf("CallID = %v, want call_1", results[0].CallID)
	}
	if !results[0].IsError {
		t.Error("IsError = false, want true")
	}
	if results[0].Content != "connection failed" {
		t.Errorf("Content = %v, want 'connection failed'", results[0].Content)
	}
}

func TestToolResultBuilderFromExecution(t *testing.T) {
	builder := NewToolResults()
	testErr := errForTest("tool failed")

	results := builder.
		FromExecution("call_1", "success result", nil).
		FromExecution("call_2", nil, testErr).
		Build()

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	// First result: success
	if results[0].IsError {
		t.Error("results[0].IsError = true, want false")
	}
	if results[0].Content != "success result" {
		t.Errorf("results[0].Content = %v, want 'success result'", results[0].Content)
	}

	// Second result: error
	if !results[1].IsError {
		t.Error("results[1].IsError = false, want true")
	}
}

func TestTypedToolResultBuilder(t *testing.T) {
	type WeatherResult struct {
		Temp       float64 `json:"temp"`
		Conditions string  `json:"conditions"`
	}

	builder := NewTypedToolResults[WeatherResult]()
	results := builder.
		Success("call_1", WeatherResult{Temp: 72.5, Conditions: "sunny"}).
		Success("call_2", WeatherResult{Temp: 65.0, Conditions: "cloudy"}).
		Build()

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	// Verify the results are untyped ToolResult
	if results[0].CallID != "call_1" {
		t.Errorf("results[0].CallID = %v, want call_1", results[0].CallID)
	}

	// Verify the content is the struct
	weather, ok := results[0].Content.(WeatherResult)
	if !ok {
		t.Fatalf("results[0].Content is not WeatherResult, got %T", results[0].Content)
	}
	if weather.Temp != 72.5 {
		t.Errorf("weather.Temp = %v, want 72.5", weather.Temp)
	}
}

func TestTypedToolResultToUntyped(t *testing.T) {
	type Result struct {
		Value int `json:"value"`
	}

	typed := TypedToolResult[Result]{
		CallID:  "call_1",
		Content: Result{Value: 42},
		IsError: false,
	}

	untyped := typed.ToUntyped()

	if untyped.CallID != "call_1" {
		t.Errorf("CallID = %v, want call_1", untyped.CallID)
	}
	if untyped.IsError {
		t.Error("IsError = true, want false")
	}

	result, ok := untyped.Content.(Result)
	if !ok {
		t.Fatalf("Content is not Result, got %T", untyped.Content)
	}
	if result.Value != 42 {
		t.Errorf("Value = %d, want 42", result.Value)
	}
}

func TestRoleToolConstant(t *testing.T) {
	if RoleTool != "tool" {
		t.Errorf("RoleTool = %v, want 'tool'", RoleTool)
	}
}

func TestMessageWithToolCalls(t *testing.T) {
	msg := Message{
		Role: RoleAssistant,
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"NYC"}`)},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result Message
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %v, want get_weather", result.ToolCalls[0].Name)
	}
}

func TestMessageWithToolResults(t *testing.T) {
	msg := Message{
		Role: RoleTool,
		ToolResults: []ToolResult{
			{CallID: "call_1", Content: "sunny, 72F", IsError: false},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result Message
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result.Role != RoleTool {
		t.Errorf("Role = %v, want %v", result.Role, RoleTool)
	}
	if len(result.ToolResults) != 1 {
		t.Fatalf("len(ToolResults) = %d, want 1", len(result.ToolResults))
	}
	if result.ToolResults[0].CallID != "call_1" {
		t.Errorf("ToolResults[0].CallID = %v, want call_1", result.ToolResults[0].CallID)
	}
}

// errForTest is a simple error type for testing
type errForTest string

func (e errForTest) Error() string { return string(e) }
