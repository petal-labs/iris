package gemini

import (
	"encoding/json"
	"testing"
)

func TestGeminiImageRequestJSON(t *testing.T) {
	req := &geminiImageGenConfig{
		AspectRatio: "16:9",
		ImageSize:   "2K",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["aspectRatio"] != "16:9" {
		t.Errorf("aspectRatio = %v, want 16:9", parsed["aspectRatio"])
	}
	if parsed["imageSize"] != "2K" {
		t.Errorf("imageSize = %v, want 2K", parsed["imageSize"])
	}
}
