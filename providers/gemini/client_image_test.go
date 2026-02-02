package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestGenerateImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []geminiCandidate{{
				Content: geminiContent{
					Parts: []geminiPart{{
						InlineData: &geminiInlineData{
							MimeType: "image/png",
							Data:     "aW1hZ2VkYXRh",
						},
					}},
				},
			}},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	resp, err := p.GenerateImage(context.Background(), &core.ImageGenerateRequest{
		Model:  "gemini-2.5-flash-image",
		Prompt: "A sunset",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "aW1hZ2VkYXRh" {
		t.Errorf("B64JSON = %s, want aW1hZ2VkYXRh", resp.Data[0].B64JSON)
	}
}
