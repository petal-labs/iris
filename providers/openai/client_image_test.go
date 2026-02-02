// providers/openai/client_image_test.go
package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestGenerateImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Errorf("path = %s, want /images/generations", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		var req openAIImageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		if req.Model != "gpt-image-1" {
			t.Errorf("model = %s, want gpt-image-1", req.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIImageResponse{
			Created: 1234567890,
			Data: []openAIImageData{
				{B64JSON: "dGVzdGltYWdl"},
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	resp, err := p.GenerateImage(context.Background(), &core.ImageGenerateRequest{
		Model:  "gpt-image-1",
		Prompt: "A cat",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "dGVzdGltYWdl" {
		t.Errorf("B64JSON = %s, want dGVzdGltYWdl", resp.Data[0].B64JSON)
	}
}

func TestGenerateImageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid prompt",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	_, err := p.GenerateImage(context.Background(), &core.ImageGenerateRequest{
		Model:  "gpt-image-1",
		Prompt: "",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
}

func TestEditImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			t.Errorf("path = %s, want /images/edits", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		// Verify it's multipart
		contentType := r.Header.Get("Content-Type")
		if len(contentType) < 19 || contentType[:19] != "multipart/form-data" {
			t.Errorf("Content-Type = %s, want multipart/form-data", contentType)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm failed: %v", err)
		}

		if r.FormValue("model") != "gpt-image-1" {
			t.Errorf("model = %s, want gpt-image-1", r.FormValue("model"))
		}
		if r.FormValue("prompt") != "Add a hat" {
			t.Errorf("prompt = %s, want 'Add a hat'", r.FormValue("prompt"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIImageResponse{
			Created: 1234567890,
			Data: []openAIImageData{
				{B64JSON: "ZWRpdGVk"},
			},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	resp, err := p.EditImage(context.Background(), &core.ImageEditRequest{
		Model:  "gpt-image-1",
		Prompt: "Add a hat",
		Images: []core.ImageInput{
			{Data: []byte("test image data"), Filename: "test.png"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].B64JSON != "ZWRpdGVk" {
		t.Errorf("B64JSON = %s, want ZWRpdGVk", resp.Data[0].B64JSON)
	}
}

func TestEditImageNoImages(t *testing.T) {
	p := New("test-key")

	_, err := p.EditImage(context.Background(), &core.ImageEditRequest{
		Model:  "gpt-image-1",
		Prompt: "Add a hat",
		Images: []core.ImageInput{}, // No images
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
}
