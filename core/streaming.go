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
//
// Behavior:
//  1. Read all chunks from Ch, accumulating Delta into output string
//  2. Check Err channel for any errors
//  3. Wait for Final to get complete response with usage/tool calls
//  4. If Final includes Output, use it; otherwise use accumulated deltas
//  5. Handle context cancellation gracefully
func DrainStream(ctx context.Context, s *ChatStream) (*ChatResponse, error) {
	if s == nil {
		return nil, ErrBadRequest
	}

	var accumulated strings.Builder
	var streamErr error
	var finalResp *ChatResponse

	// Read all chunks, checking for cancellation
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case chunk, ok := <-s.Ch:
			if !ok {
				// Channel closed, move on
				goto checkErr
			}
			accumulated.WriteString(chunk.Delta)

		case err, ok := <-s.Err:
			if ok && err != nil {
				streamErr = err
			}
			// Continue draining Ch even after error

		case resp, ok := <-s.Final:
			if ok {
				finalResp = resp
			}
		}
	}

checkErr:
	// Drain any remaining error
	select {
	case err, ok := <-s.Err:
		if ok && err != nil {
			streamErr = err
		}
	default:
	}

	// If we have an error, return it
	if streamErr != nil {
		return nil, streamErr
	}

	// Wait for final response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp, ok := <-s.Final:
		if ok {
			finalResp = resp
		}
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
