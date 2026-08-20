package core

import (
	"context"
	"strings"
)

// ChatStream represents a streaming response from a provider.
//
// Channel Rules:
//   - Providers MUST close Ch, Err, and Final when finished
//   - On context cancellation, providers MUST terminate promptly and close channels
//   - Err channel emits at most one error
//   - Final channel emits exactly once on success (or zero times on setup failure)
//   - If providers cannot compute Usage for streaming, they MAY leave it zeroed
type ChatStream struct {
	// Ch emits text deltas in order. Closed when stream ends.
	Ch <-chan ChatChunk

	// Err emits at most one error. MUST be closed when stream ends.
	// If error occurs after setup, send on Err then close all channels.
	Err <-chan error

	// Final sent exactly once (or zero if setup fails) after stream completion.
	// Includes usage and tool calls if available.
	// Providers may send partial ChatResponse with Output empty.
	Final <-chan *ChatResponse
}

// DrainStream accumulates all deltas and returns the final ChatResponse.
// Blocks until stream completes or context cancels.
// Returns [ErrNilStream] when s is nil.
//
// Behavior:
//  1. Read all chunks from Ch, accumulating Delta into output string
//  2. Keep reading after Ch closes until BOTH Err and Final have resolved
//     (closed or delivered a value) — mirrors wrapStreamWithTelemetry's
//     non-lossy discipline so a real stream error delivered after Ch
//     closes is never missed.
//  3. If Final closes without a value, that's not itself an error; Err is
//     still checked independently.
//  4. If Final includes Output, use it; otherwise use accumulated deltas.
//  5. On context cancellation, do one final non-blocking drain of Err
//     (and Final) so a concrete provider error wins over a generic
//     context error, then return ctx.Err().
func DrainStream(ctx context.Context, s *ChatStream) (*ChatResponse, error) {
	if s == nil {
		return nil, ErrNilStream
	}

	var accumulated strings.Builder
	var streamErr error
	var finalResp *ChatResponse
	chClosed := false
	errResolved := false
	finalResolved := false

	for !chClosed || !errResolved || !finalResolved {
		var chRecv <-chan ChatChunk
		if !chClosed {
			chRecv = s.Ch
		}
		var errRecv <-chan error
		if !errResolved {
			errRecv = s.Err
		}
		var finalRecv <-chan *ChatResponse
		if !finalResolved {
			finalRecv = s.Final
		}

		select {
		case <-ctx.Done():
			// A concrete provider error should win over the generic
			// context error, so do a final non-blocking drain of both
			// Err and Final before giving up.
			select {
			case err, ok := <-s.Err:
				if ok && err != nil {
					streamErr = err
				}
			default:
			}
			select {
			case <-s.Final:
			default:
			}
			if streamErr != nil {
				return nil, streamErr
			}
			return nil, ctx.Err()

		case chunk, ok := <-chRecv:
			if !ok {
				chClosed = true
				continue
			}
			accumulated.WriteString(chunk.Delta)

		case err, ok := <-errRecv:
			errResolved = true
			if ok && err != nil {
				streamErr = err
			}
			// If Err closed without a value, keep waiting on Ch/Final.

		case resp, ok := <-finalRecv:
			finalResolved = true
			if ok {
				finalResp = resp
			}
			// If Final closed without a value, that alone isn't an
			// error — Err is tracked independently above.
		}
	}

	// If we have an error, return it
	if streamErr != nil {
		return nil, streamErr
	}

	// Build response
	if finalResp == nil {
		// No final response, create one from accumulated content
		finalResp = &ChatResponse{
			Output: accumulated.String(),
		}
	} else if finalResp.Output == "" {
		// Final has no output, use accumulated deltas
		finalResp.Output = accumulated.String()
	}

	return finalResp, nil
}
