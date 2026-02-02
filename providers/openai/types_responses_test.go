package openai

import (
	"encoding/json"
	"testing"
)

func TestResponsesContentPartMarshal(t *testing.T) {
	tests := []struct {
		name     string
		part     responsesContentPart
		expected string
	}{
		{
			name: "input_text with text",
			part: responsesContentPart{
				Type: "input_text",
				Text: "What's in this image?",
			},
			expected: `{"type":"input_text","text":"What's in this image?"}`,
		},
		{
			name: "input_image with image_url",
			part: responsesContentPart{
				Type:     "input_image",
				ImageURL: "https://example.com/cat.jpg",
			},
			expected: `{"type":"input_image","image_url":"https://example.com/cat.jpg"}`,
		},
		{
			name: "input_image with file_id",
			part: responsesContentPart{
				Type:   "input_image",
				FileID: "file-abc123",
			},
			expected: `{"type":"input_image","file_id":"file-abc123"}`,
		},
		{
			name: "input_image with detail parameter",
			part: responsesContentPart{
				Type:     "input_image",
				ImageURL: "https://example.com/cat.jpg",
				Detail:   "high",
			},
			expected: `{"type":"input_image","image_url":"https://example.com/cat.jpg","detail":"high"}`,
		},
		{
			name: "input_file with file_url",
			part: responsesContentPart{
				Type:    "input_file",
				FileURL: "https://example.com/doc.pdf",
			},
			expected: `{"type":"input_file","file_url":"https://example.com/doc.pdf"}`,
		},
		{
			name: "input_file with file_id",
			part: responsesContentPart{
				Type:   "input_file",
				FileID: "file-xyz789",
			},
			expected: `{"type":"input_file","file_id":"file-xyz789"}`,
		},
		{
			name: "input_file with file_data and filename",
			part: responsesContentPart{
				Type:     "input_file",
				FileData: "base64encodeddata",
				Filename: "doc.pdf",
			},
			expected: `{"type":"input_file","file_data":"base64encodeddata","filename":"doc.pdf"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.part)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("got %s, want %s", string(data), tt.expected)
			}
		})
	}
}

func TestResponsesInputMessageMarshal(t *testing.T) {
	tests := []struct {
		name string
		msg  responsesInputMessage
		want string
	}{
		{
			name: "simple text content",
			msg: responsesInputMessage{
				Role:    "user",
				Content: responsesContent{Text: "Hello"},
			},
			want: `{"role":"user","content":"Hello"}`,
		},
		{
			name: "multimodal content",
			msg: responsesInputMessage{
				Role: "user",
				Content: responsesContent{
					Parts: []responsesContentPart{
						{Type: "input_text", Text: "What's in this image?"},
						{Type: "input_image", ImageURL: "https://example.com/cat.jpg"},
					},
				},
			},
			want: `{"role":"user","content":[{"type":"input_text","text":"What's in this image?"},{"type":"input_image","image_url":"https://example.com/cat.jpg"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestResponsesContentUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantText  string
		wantParts int
	}{
		{
			name:      "string content",
			input:     `"Hello world"`,
			wantText:  "Hello world",
			wantParts: 0,
		},
		{
			name:      "array content",
			input:     `[{"type":"input_text","text":"Hi"},{"type":"input_image","image_url":"https://example.com/img.jpg"}]`,
			wantText:  "",
			wantParts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content responsesContent
			if err := json.Unmarshal([]byte(tt.input), &content); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if content.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", content.Text, tt.wantText)
			}
			if len(content.Parts) != tt.wantParts {
				t.Errorf("len(Parts) = %d, want %d", len(content.Parts), tt.wantParts)
			}
		})
	}
}
