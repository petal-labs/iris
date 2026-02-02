package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
)

// StreamImage generates images with streaming partial results.
func (p *OpenAI) StreamImage(ctx context.Context, req *core.ImageGenerateRequest) (*core.ImageStream, error) {
	openaiReq := mapImageGenerateRequest(req)
	openaiReq.Stream = true
	if openaiReq.PartialImages == 0 {
		openaiReq.PartialImages = 1 // Default to at least 1 partial
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + "/images/generations"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range p.buildHeaders() {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	resp, err := p.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &core.ProviderError{
			Provider: "openai",
			Code:     "network_error",
			Message:  err.Error(),
			Err:      core.ErrNetwork,
		}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseImageError(resp)
	}

	chunkCh := make(chan core.ImageChunk, 10)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.ImageResponse, 1)

	go p.processImageStream(ctx, resp, chunkCh, errCh, finalCh)

	return &core.ImageStream{
		Ch:    chunkCh,
		Err:   errCh,
		Final: finalCh,
	}, nil
}

// processImageStream reads SSE events and dispatches to channels.
func (p *OpenAI) processImageStream(
	ctx context.Context,
	resp *http.Response,
	chunkCh chan<- core.ImageChunk,
	errCh chan<- error,
	finalCh chan<- *core.ImageResponse,
) {
	defer resp.Body.Close()
	defer close(chunkCh)
	defer close(errCh)
	defer close(finalCh)

	scanner := bufio.NewScanner(resp.Body)
	// Base64-encoded images can be several MB, increase buffer from default 64KB
	const maxImageSize = 10 * 1024 * 1024 // 10MB
	scanner.Buffer(make([]byte, 64*1024), maxImageSize)
	var completedEvent *openAIImageCompletedEvent

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			break
		}

		// Try to parse as stream event
		var event openAIImageStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err == nil {
			switch event.Type {
			case "image_generation.partial_image":
				select {
				case chunkCh <- mapImageChunk(&event):
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
				continue
			case "image_generation.completed":
				// Parse the completed event with full details
				var completed openAIImageCompletedEvent
				if err := json.Unmarshal([]byte(data), &completed); err == nil {
					completedEvent = &completed
				}
				continue
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errCh <- fmt.Errorf("stream read error: %w", err)
		return
	}

	// Send final response from completed event
	if completedEvent != nil {
		finalCh <- &core.ImageResponse{
			Data: []core.ImageData{{B64JSON: completedEvent.B64JSON}},
		}
	}
}
