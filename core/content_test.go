package core

import "testing"

func TestImageDetailConstants(t *testing.T) {
	tests := []struct {
		name  string
		value ImageDetail
		want  string
	}{
		{"auto", ImageDetailAuto, "auto"},
		{"low", ImageDetailLow, "low"},
		{"high", ImageDetailHigh, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("ImageDetail%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestInputTextContentType(t *testing.T) {
	text := InputText{Text: "Hello, world!"}
	got := text.ContentType()
	want := "input_text"

	if got != want {
		t.Errorf("InputText.ContentType() = %q, want %q", got, want)
	}
}

func TestInputImageContentType(t *testing.T) {
	img := InputImage{ImageURL: "https://example.com/image.jpg"}
	got := img.ContentType()
	want := "input_image"

	if got != want {
		t.Errorf("InputImage.ContentType() = %q, want %q", got, want)
	}
}

func TestInputFileContentType(t *testing.T) {
	file := InputFile{FileID: "file-abc123"}
	got := file.ContentType()
	want := "input_file"

	if got != want {
		t.Errorf("InputFile.ContentType() = %q, want %q", got, want)
	}
}

func TestContentPartInterface(t *testing.T) {
	// Verify all types implement ContentPart interface
	parts := []ContentPart{
		InputText{Text: "test"},
		InputImage{ImageURL: "https://example.com/img.png"},
		InputFile{FileID: "file-123"},
	}

	expected := []string{"input_text", "input_image", "input_file"}

	for i, part := range parts {
		if got := part.ContentType(); got != expected[i] {
			t.Errorf("parts[%d].ContentType() = %q, want %q", i, got, expected[i])
		}
	}
}

func TestInputImageFields(t *testing.T) {
	img := InputImage{
		ImageURL: "https://example.com/photo.jpg",
		FileID:   "file-xyz789",
		Detail:   ImageDetailHigh,
	}

	if img.ImageURL != "https://example.com/photo.jpg" {
		t.Errorf("ImageURL = %q, want %q", img.ImageURL, "https://example.com/photo.jpg")
	}
	if img.FileID != "file-xyz789" {
		t.Errorf("FileID = %q, want %q", img.FileID, "file-xyz789")
	}
	if img.Detail != ImageDetailHigh {
		t.Errorf("Detail = %q, want %q", img.Detail, ImageDetailHigh)
	}
}

func TestInputFileFields(t *testing.T) {
	file := InputFile{
		FileID:   "file-abc123",
		FileURL:  "https://example.com/doc.pdf",
		FileData: "SGVsbG8gV29ybGQ=",
		Filename: "document.pdf",
	}

	if file.FileID != "file-abc123" {
		t.Errorf("FileID = %q, want %q", file.FileID, "file-abc123")
	}
	if file.FileURL != "https://example.com/doc.pdf" {
		t.Errorf("FileURL = %q, want %q", file.FileURL, "https://example.com/doc.pdf")
	}
	if file.FileData != "SGVsbG8gV29ybGQ=" {
		t.Errorf("FileData = %q, want %q", file.FileData, "SGVsbG8gV29ybGQ=")
	}
	if file.Filename != "document.pdf" {
		t.Errorf("Filename = %q, want %q", file.Filename, "document.pdf")
	}
}

func TestMessageParts(t *testing.T) {
	// Test that Message can hold multimodal Parts
	msg := Message{
		Role: RoleUser,
		Parts: []ContentPart{
			InputText{Text: "What's in this image?"},
			InputImage{ImageURL: "https://example.com/photo.jpg", Detail: ImageDetailHigh},
		},
	}

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want %q", msg.Role, RoleUser)
	}
	if len(msg.Parts) != 2 {
		t.Fatalf("len(Parts) = %d, want 2", len(msg.Parts))
	}

	// Verify first part is InputText
	textPart, ok := msg.Parts[0].(InputText)
	if !ok {
		t.Fatalf("Parts[0] is not InputText, got %T", msg.Parts[0])
	}
	if textPart.Text != "What's in this image?" {
		t.Errorf("Parts[0].Text = %q, want %q", textPart.Text, "What's in this image?")
	}

	// Verify second part is InputImage
	imgPart, ok := msg.Parts[1].(InputImage)
	if !ok {
		t.Fatalf("Parts[1] is not InputImage, got %T", msg.Parts[1])
	}
	if imgPart.ImageURL != "https://example.com/photo.jpg" {
		t.Errorf("Parts[1].ImageURL = %q, want %q", imgPart.ImageURL, "https://example.com/photo.jpg")
	}
	if imgPart.Detail != ImageDetailHigh {
		t.Errorf("Parts[1].Detail = %q, want %q", imgPart.Detail, ImageDetailHigh)
	}
}

func TestMessageBackwardCompatibility(t *testing.T) {
	// Test that simple text messages still work with Content field
	msg := Message{
		Role:    RoleUser,
		Content: "Hello, world!",
	}

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want %q", msg.Role, RoleUser)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", msg.Content, "Hello, world!")
	}
	if msg.Parts != nil {
		t.Errorf("Parts should be nil for simple text messages, got %v", msg.Parts)
	}
}
