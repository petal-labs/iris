package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/petal-labs/iris/v2/core"
)

// StreamSynthesize converts text to audio using ElevenLabs' HTTP streaming
// POST /v1/text-to-speech/{voice_id}/stream endpoint.
//
// Audio bytes are streamed incrementally on Ch in bounded chunks (configured by
// [WithStreamChunkSize] and [WithStreamBufferSize]) without buffering the full
// audio payload in memory. Final carries response metadata and usage with
// Audio.Data empty; callers that want a single assembled buffer can use
// [core.DrainSpeechStream].
//
// Channel contract:
//   - Ch, Err, and Final are always closed before the background goroutine exits.
//   - Cancelling ctx promptly closes the underlying HTTP response body,
//     unblocks any pending read or channel send, reports ctx.Err() on Err,
//     and exits without leaking goroutines.
func (p *ElevenLabs) StreamSynthesize(ctx context.Context, req *core.SpeechRequest) (*core.SpeechStream, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	outputFormat, resolvedFormat, err := resolveOutputFormat(req.Format)
	if err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = DefaultTTSModel
	}

	payload, err := json.Marshal(buildTTSRequestBody(req, model))
	if err != nil {
		return nil, newDecodeError(fmt.Errorf("marshal streaming tts request: %w", err), nil)
	}

	path := fmt.Sprintf("/v1/text-to-speech/%s/stream?output_format=%s",
		url.PathEscape(req.VoiceID),
		url.QueryEscape(outputFormat),
	)

	resp, reqCtx, err := p.doStreamRequest(ctx, http.MethodPost, path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	resolvedFormat.MIMEType = responseMIMEType(resp.Header.Get("Content-Type"), resolvedFormat.EffectiveMIMEType())
	requestID := extractRequestID(resp.Header)
	charCost := parseCharacterCost(resp.Header)

	chunkSize := p.config.StreamChunkSize
	if chunkSize <= 0 {
		chunkSize = DefaultStreamChunkSize
	}
	bufCap := p.config.StreamBufferSize
	if bufCap <= 0 {
		bufCap = DefaultStreamBufferSize
	}

	ch := make(chan core.AudioChunk, bufCap)
	errCh := make(chan error, 1)
	finalCh := make(chan *core.SpeechResponse, 1)

	go func() {
		defer close(ch)
		defer close(errCh)
		defer close(finalCh)
		defer resp.Body.Close()

		// Ensure a blocked resp.Body.Read unblocks promptly when reqCtx is cancelled
		// or times out, even on custom transports or pipes.
		stopWatch := context.AfterFunc(reqCtx, func() {
			_ = resp.Body.Close()
		})
		defer stopWatch()

		readBuf := make([]byte, chunkSize)
		var seq int64
		var totalBytes int64

		for {
			n, readErr := resp.Body.Read(readBuf)
			if n > 0 {
				chunkData := make([]byte, n)
				copy(chunkData, readBuf[:n])

				chunk := core.AudioChunk{
					Data:     chunkData,
					Format:   resolvedFormat,
					Sequence: seq,
				}
				seq++
				totalBytes += int64(n)

				select {
				case ch <- chunk:
				case <-reqCtx.Done():
					errCh <- reqCtx.Err()
					return
				}
			}

			if readErr == nil {
				continue
			}

			if errors.Is(readErr, io.EOF) {
				if totalBytes == 0 {
					errCh <- newDecodeError(errors.New("empty audio stream from ElevenLabs"), nil)
					return
				}

				finalMarker := core.AudioChunk{
					Format:   resolvedFormat,
					Sequence: seq,
					IsFinal:  true,
				}
				select {
				case ch <- finalMarker:
				case <-reqCtx.Done():
					errCh <- reqCtx.Err()
					return
				}

				usage := core.SpeechUsage{
					OutputBytes: totalBytes,
				}
				if charCost > 0 {
					usage.InputCharacters = charCost
				}

				finalCh <- &core.SpeechResponse{
					ID:      requestID,
					Model:   model,
					VoiceID: req.VoiceID,
					Audio: core.AudioData{
						Format: resolvedFormat,
					},
					Usage: usage,
				}
				return
			}

			// Non-EOF read failure: prefer reqCtx.Err() when cancellation or timeout
			// closed the body underneath Read.
			if ctxErr := reqCtx.Err(); ctxErr != nil {
				errCh <- ctxErr
				return
			}
			if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
				errCh <- readErr
				return
			}
			errCh <- newNetworkError(readErr)
			return
		}
	}()

	return &core.SpeechStream{
		Ch:    ch,
		Err:   errCh,
		Final: finalCh,
	}, nil
}
