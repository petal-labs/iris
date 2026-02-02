package openai

import (
	"encoding/json"
	"testing"
)

func TestOpenAIEmbeddingRequest_JSON(t *testing.T) {
	dims := 1024
	req := openAIEmbeddingRequest{
		Model:          "text-embedding-3-small",
		Input:          []string{"hello", "world"},
		EncodingFormat: "float",
		Dimensions:     &dims,
		User:           "user-123",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed openAIEmbeddingRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Model != "text-embedding-3-small" {
		t.Errorf("Model = %q, want text-embedding-3-small", parsed.Model)
	}
	if len(parsed.Input) != 2 {
		t.Errorf("len(Input) = %d, want 2", len(parsed.Input))
	}
}

func TestOpenAIEmbeddingResponse_JSON(t *testing.T) {
	input := `{
		"object": "list",
		"data": [
			{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]},
			{"object": "embedding", "index": 1, "embedding": [0.4, 0.5, 0.6]}
		],
		"model": "text-embedding-3-small",
		"usage": {"prompt_tokens": 5, "total_tokens": 5}
	}`

	var resp openAIEmbeddingResponse
	if err := json.Unmarshal([]byte(input), &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("Object = %q, want list", resp.Object)
	}
	if len(resp.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].Index != 0 {
		t.Errorf("Data[0].Index = %d, want 0", resp.Data[0].Index)
	}
	if resp.Usage.PromptTokens != 5 {
		t.Errorf("Usage.PromptTokens = %d, want 5", resp.Usage.PromptTokens)
	}
}
