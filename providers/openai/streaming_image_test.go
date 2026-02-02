// providers/openai/streaming_image_test.go
package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestStreamImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send partial images
		events := []openAIImageStreamEvent{
			{Type: "image_generation.partial_image", PartialImageIndex: 0, B64JSON: "cGFydGlhbDE="},
			{Type: "image_generation.partial_image", PartialImageIndex: 1, B64JSON: "cGFydGlhbDI="},
		}

		for _, event := range events {
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		// Send completed event with final image
		completedEvent := openAIImageCompletedEvent{
			Type:    "image_generation.completed",
			B64JSON: "ZmluYWw=",
		}
		data, _ := json.Marshal(completedEvent)
		fmt.Fprintf(w, "data: %s\n\n", data)
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	stream, err := p.StreamImage(context.Background(), &core.ImageGenerateRequest{
		Model:         "gpt-image-1",
		Prompt:        "A cat",
		PartialImages: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	var chunks []core.ImageChunk
	for chunk := range stream.Ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Errorf("len(chunks) = %d, want 2", len(chunks))
	}

	select {
	case err := <-stream.Err:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	default:
	}

	final := <-stream.Final
	if final == nil {
		t.Fatal("expected final response, got nil")
	}
	if len(final.Data) != 1 {
		t.Errorf("len(Data) = %d, want 1", len(final.Data))
	}
}

func TestStreamImageError(t *testing.T) {
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

	_, err := p.StreamImage(context.Background(), &core.ImageGenerateRequest{
		Model:         "gpt-image-1",
		Prompt:        "",
		PartialImages: 2,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStreamImageContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send one partial then wait
		event := openAIImageStreamEvent{Type: "image_generation.partial_image", PartialImageIndex: 0, B64JSON: "cGFydGlhbA=="}
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// Wait for context cancellation (simulated by client closing)
		<-r.Context().Done()
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())

	stream, err := p.StreamImage(ctx, &core.ImageGenerateRequest{
		Model:         "gpt-image-1",
		Prompt:        "A cat",
		PartialImages: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Read one chunk then cancel
	<-stream.Ch
	cancel()

	// Should eventually get context error or channels close
	select {
	case err := <-stream.Err:
		// Accept nil or context.Canceled (possibly wrapped)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error: %v", err)
		}
	case <-stream.Final:
		// Final may be nil, that's ok
	}
}
