// providers/openai/types_image_test.go
package openai

import (
	"encoding/json"
	"testing"
)

func TestImageRequestMarshal(t *testing.T) {
	req := openAIImageRequest{
		Model:  "gpt-image-1",
		Prompt: "A cat",
		N:      1,
		Size:   "1024x1024",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got["model"] != "gpt-image-1" {
		t.Errorf("model = %v, want gpt-image-1", got["model"])
	}
	if got["prompt"] != "A cat" {
		t.Errorf("prompt = %v, want 'A cat'", got["prompt"])
	}
}

func TestImageResponseUnmarshal(t *testing.T) {
	data := `{
		"created": 1234567890,
		"data": [
			{"b64_json": "aW1hZ2VkYXRh", "revised_prompt": "A cute cat"}
		]
	}`

	var resp openAIImageResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Created != 1234567890 {
		t.Errorf("Created = %d, want 1234567890", resp.Created)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "aW1hZ2VkYXRh" {
		t.Errorf("B64JSON = %s, want aW1hZ2VkYXRh", resp.Data[0].B64JSON)
	}
}
