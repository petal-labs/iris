package gemini

import (
	"encoding/json"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestMapMessages(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleSystem, Content: "You are helpful."},
		{Role: core.RoleSystem, Content: "Be concise."},
		{Role: core.RoleUser, Content: "Hello"},
		{Role: core.RoleAssistant, Content: "Hi there"},
		{Role: core.RoleUser, Content: "How are you?"},
	}

	system, contents := mapMessages(msgs)

	// Check system concatenation
	if system != "You are helpful.\n\nBe concise." {
		t.Errorf("system = %q, want 'You are helpful.\\n\\nBe concise.'", system)
	}

	// Check message count (excluding system)
	if len(contents) != 3 {
		t.Fatalf("contents count = %d, want 3", len(contents))
	}

	// Check roles
	if contents[0].Role != "user" {
		t.Errorf("contents[0].Role = %q, want 'user'", contents[0].Role)
	}
	if contents[1].Role != "model" {
		t.Errorf("contents[1].Role = %q, want 'model'", contents[1].Role)
	}
	if contents[2].Role != "user" {
		t.Errorf("contents[2].Role = %q, want 'user'", contents[2].Role)
	}

	// Check content
	if contents[0].Parts[0].Text != "Hello" {
		t.Errorf("contents[0].Parts[0].Text = %q, want 'Hello'", contents[0].Parts[0].Text)
	}
}

func TestMapMessagesNoSystem(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleUser, Content: "Hello"},
	}

	system, contents := mapMessages(msgs)

	if system != "" {
		t.Errorf("system = %q, want empty", system)
	}

	if len(contents) != 1 {
		t.Fatalf("contents count = %d, want 1", len(contents))
	}
}

func TestBuildThinkingConfigGemini3(t *testing.T) {
	tests := []struct {
		model     string
		effort    core.ReasoningEffort
		wantLevel string
	}{
		{string(ModelGemini3Flash), core.ReasoningEffortNone, "minimal"},
		{string(ModelGemini3Flash), core.ReasoningEffortLow, "low"},
		{string(ModelGemini3Flash), core.ReasoningEffortMedium, "medium"},
		{string(ModelGemini3Flash), core.ReasoningEffortHigh, "high"},
		{string(ModelGemini3Pro), core.ReasoningEffortNone, "low"}, // Pro can't disable
		{string(ModelGemini3Pro), core.ReasoningEffortHigh, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.model+"_"+string(tt.effort), func(t *testing.T) {
			cfg := buildThinkingConfig(tt.model, tt.effort)
			if cfg == nil {
				t.Fatal("cfg is nil")
			}
			if cfg.ThinkingLevel != tt.wantLevel {
				t.Errorf("ThinkingLevel = %q, want %q", cfg.ThinkingLevel, tt.wantLevel)
			}
			if cfg.ThinkingBudget != nil {
				t.Error("ThinkingBudget should be nil for Gemini 3")
			}
		})
	}
}

func TestBuildThinkingConfigGemini25(t *testing.T) {
	tests := []struct {
		effort     core.ReasoningEffort
		wantBudget int
	}{
		{core.ReasoningEffortNone, 0},
		{core.ReasoningEffortLow, 1024},
		{core.ReasoningEffortMedium, 8192},
		{core.ReasoningEffortHigh, 24576},
		{core.ReasoningEffortXHigh, 32768},
	}

	for _, tt := range tests {
		t.Run(string(tt.effort), func(t *testing.T) {
			cfg := buildThinkingConfig(string(ModelGemini25Flash), tt.effort)
			if cfg == nil {
				t.Fatal("cfg is nil")
			}
			if cfg.ThinkingLevel != "" {
				t.Errorf("ThinkingLevel = %q, want empty for Gemini 2.5", cfg.ThinkingLevel)
			}
			if cfg.ThinkingBudget == nil {
				t.Fatal("ThinkingBudget is nil")
			}
			if *cfg.ThinkingBudget != tt.wantBudget {
				t.Errorf("ThinkingBudget = %d, want %d", *cfg.ThinkingBudget, tt.wantBudget)
			}
		})
	}
}

func TestBuildThinkingConfigNoReasoning(t *testing.T) {
	cfg := buildThinkingConfig(string(ModelGemini25Flash), "")
	if cfg != nil {
		t.Error("cfg should be nil when no reasoning effort specified")
	}
}

func TestMapResponse(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Role: "model",
					Parts: []geminiPart{
						{Text: "Hello "},
						{Text: "world!"},
					},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: &geminiUsage{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
		},
	}

	result, err := mapResponse(resp, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("mapResponse error = %v", err)
	}

	if result.Output != "Hello world!" {
		t.Errorf("Output = %q, want 'Hello world!'", result.Output)
	}

	if result.Model != "gemini-2.5-flash" {
		t.Errorf("Model = %q, want 'gemini-2.5-flash'", result.Model)
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}

	if result.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", result.Usage.CompletionTokens)
	}
}

func TestMapResponseWithToolCalls(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Role: "model",
					Parts: []geminiPart{
						{
							FunctionCall: &geminiFunctionCall{
								Name: "get_weather",
								Args: json.RawMessage(`{"location":"NYC"}`),
							},
						},
					},
				},
			},
		},
		UsageMetadata: &geminiUsage{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
		},
	}

	result, err := mapResponse(resp, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("mapResponse error = %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "call_0" {
		t.Errorf("ToolCall ID = %q, want 'call_0'", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("ToolCall Name = %q, want 'get_weather'", tc.Name)
	}
	if string(tc.Arguments) != `{"location":"NYC"}` {
		t.Errorf("ToolCall Arguments = %s, want '{\"location\":\"NYC\"}'", tc.Arguments)
	}
}

func TestMapMessages_WithMultimodalParts(t *testing.T) {
	msgs := []core.Message{
		{
			Role: core.RoleUser,
			Parts: []core.ContentPart{
				core.InputText{Text: "What's in this image?"},
				core.InputImage{
					ImageURL: "data:image/jpeg;base64,/9j/4AAQ...",
				},
			},
		},
	}

	system, contents := mapMessages(msgs)

	if system != "" {
		t.Errorf("system = %q, want empty", system)
	}

	if len(contents) != 1 {
		t.Fatalf("len(contents) = %d, want 1", len(contents))
	}

	if len(contents[0].Parts) != 2 {
		t.Fatalf("len(parts) = %d, want 2", len(contents[0].Parts))
	}

	// First part should be text
	if contents[0].Parts[0].Text != "What's in this image?" {
		t.Errorf("Part[0].Text = %q, want prompt text", contents[0].Parts[0].Text)
	}

	// Second part should be inline data
	if contents[0].Parts[1].InlineData == nil {
		t.Fatal("Part[1].InlineData is nil")
	}
}

func TestMapMessages_WithFileReference(t *testing.T) {
	msgs := []core.Message{
		{
			Role: core.RoleUser,
			Parts: []core.ContentPart{
				core.InputText{Text: "Summarize this document"},
				core.InputFile{
					FileID: "https://generativelanguage.googleapis.com/v1beta/files/abc-123",
				},
			},
		},
	}

	_, contents := mapMessages(msgs)

	if len(contents[0].Parts) != 2 {
		t.Fatalf("len(parts) = %d, want 2", len(contents[0].Parts))
	}

	if contents[0].Parts[1].FileData == nil {
		t.Fatal("Part[1].FileData is nil")
	}
	if contents[0].Parts[1].FileData.FileURI != "https://generativelanguage.googleapis.com/v1beta/files/abc-123" {
		t.Errorf("FileURI = %q, want files URI", contents[0].Parts[1].FileData.FileURI)
	}
}

func TestParseDataURL(t *testing.T) {
	tests := []struct {
		name         string
		dataURL      string
		wantMimeType string
		wantData     string
	}{
		{
			name:         "jpeg image",
			dataURL:      "data:image/jpeg;base64,/9j/4AAQ...",
			wantMimeType: "image/jpeg",
			wantData:     "/9j/4AAQ...",
		},
		{
			name:         "png image",
			dataURL:      "data:image/png;base64,iVBORw0KGgo=",
			wantMimeType: "image/png",
			wantData:     "iVBORw0KGgo=",
		},
		{
			name:         "pdf document",
			dataURL:      "data:application/pdf;base64,JVBERi0=",
			wantMimeType: "application/pdf",
			wantData:     "JVBERi0=",
		},
		{
			name:         "invalid no comma",
			dataURL:      "data:image/jpeg;base64",
			wantMimeType: "",
			wantData:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType, data := parseDataURL(tt.dataURL)
			if mimeType != tt.wantMimeType {
				t.Errorf("mimeType = %q, want %q", mimeType, tt.wantMimeType)
			}
			if data != tt.wantData {
				t.Errorf("data = %q, want %q", data, tt.wantData)
			}
		})
	}
}

func TestGuessMimeType(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"IMAGE.JPG", "image/jpeg"},
		{"photo.png", "image/png"},
		{"animation.gif", "image/gif"},
		{"image.webp", "image/webp"},
		{"document.pdf", "application/pdf"},
		{"notes.txt", "text/plain"},
		{"data.json", "application/json"},
		{"unknown.xyz", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := guessMimeType(tt.filename)
			if got != tt.want {
				t.Errorf("guessMimeType(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestMapInputImage_ExternalURL(t *testing.T) {
	img := core.InputImage{
		ImageURL: "https://example.com/image.jpg",
	}

	part := mapInputImage(img)

	if part.FileData == nil {
		t.Fatal("FileData is nil")
	}
	if part.FileData.FileURI != "https://example.com/image.jpg" {
		t.Errorf("FileURI = %q, want external URL", part.FileData.FileURI)
	}
	if part.InlineData != nil {
		t.Error("InlineData should be nil for external URL")
	}
}

func TestMapInputImage_FileID(t *testing.T) {
	img := core.InputImage{
		FileID: "files/abc-123",
	}

	part := mapInputImage(img)

	if part.FileData == nil {
		t.Fatal("FileData is nil")
	}
	if part.FileData.FileURI != "files/abc-123" {
		t.Errorf("FileURI = %q, want FileID", part.FileData.FileURI)
	}
}

func TestMapInputFile_WithBase64Data(t *testing.T) {
	file := core.InputFile{
		FileData: "JVBERi0xLjQKJ...",
		Filename: "document.pdf",
	}

	part := mapInputFile(file)

	if part.InlineData == nil {
		t.Fatal("InlineData is nil")
	}
	if part.InlineData.Data != "JVBERi0xLjQKJ..." {
		t.Errorf("Data = %q, want base64 data", part.InlineData.Data)
	}
	if part.InlineData.MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want 'application/pdf'", part.InlineData.MimeType)
	}
}

func TestMapInputFile_WithFileURL(t *testing.T) {
	file := core.InputFile{
		FileURL: "https://storage.googleapis.com/bucket/doc.pdf",
	}

	part := mapInputFile(file)

	if part.FileData == nil {
		t.Fatal("FileData is nil")
	}
	if part.FileData.FileURI != "https://storage.googleapis.com/bucket/doc.pdf" {
		t.Errorf("FileURI = %q, want FileURL", part.FileData.FileURI)
	}
}
