package core

import (
	"context"
	"time"
)

// BatchProvider is an optional interface for providers supporting batch operations.
// Batch processing enables 50% cost savings for high-throughput workloads by
// submitting multiple requests that are processed asynchronously.
//
// Not all providers support batch operations. Check Supports(FeatureBatch)
// before using batch methods, or use the AsBatchProvider helper.
//
// Typical workflow:
//  1. CreateBatch to submit requests
//  2. Poll GetBatchStatus until IsComplete() returns true
//  3. GetBatchResults to retrieve responses
//
// Example:
//
//	bp, ok := core.AsBatchProvider(provider)
//	if !ok {
//	    return errors.New("provider does not support batch operations")
//	}
//
//	batchID, err := bp.CreateBatch(ctx, requests)
//	if err != nil {
//	    return err
//	}
//
//	// Poll for completion
//	for {
//	    info, _ := bp.GetBatchStatus(ctx, batchID)
//	    if info.IsComplete() {
//	        break
//	    }
//	    time.Sleep(30 * time.Second)
//	}
//
//	results, err := bp.GetBatchResults(ctx, batchID)
type BatchProvider interface {
	// CreateBatch submits requests for asynchronous batch processing.
	// Returns a BatchID that can be used to track progress and retrieve results.
	// Each request's CustomID must be unique within the batch.
	CreateBatch(ctx context.Context, requests []BatchRequest) (BatchID, error)

	// GetBatchStatus returns the current status of a batch.
	// Poll this method to check if the batch has completed processing.
	GetBatchStatus(ctx context.Context, id BatchID) (*BatchInfo, error)

	// GetBatchResults retrieves completed batch results.
	// Should only be called after GetBatchStatus indicates completion.
	// Returns results for all requests, including failures.
	GetBatchResults(ctx context.Context, id BatchID) ([]BatchResult, error)

	// CancelBatch cancels a pending or in-progress batch.
	// Already completed requests may still return results.
	CancelBatch(ctx context.Context, id BatchID) error

	// ListBatches returns all batches for the account.
	// Use limit to control pagination (0 for default).
	ListBatches(ctx context.Context, limit int) ([]BatchInfo, error)
}

// AsBatchProvider attempts to cast a Provider to BatchProvider.
// Returns the BatchProvider and true if the provider supports batch operations,
// or nil and false otherwise.
//
// Example:
//
//	if bp, ok := core.AsBatchProvider(provider); ok {
//	    batchID, err := bp.CreateBatch(ctx, requests)
//	    // ...
//	}
func AsBatchProvider(p Provider) (BatchProvider, bool) {
	bp, ok := p.(BatchProvider)
	return bp, ok
}

// BatchWaiter provides utilities for waiting on batch completion.
type BatchWaiter struct {
	provider     BatchProvider
	pollInterval time.Duration
	maxWait      time.Duration
}

// NewBatchWaiter creates a waiter with default settings.
// Default poll interval is 30 seconds, max wait is 24 hours.
func NewBatchWaiter(provider BatchProvider) *BatchWaiter {
	return &BatchWaiter{
		provider:     provider,
		pollInterval: 30 * time.Second,
		maxWait:      24 * time.Hour,
	}
}

// WithPollInterval sets the interval between status checks.
func (w *BatchWaiter) WithPollInterval(d time.Duration) *BatchWaiter {
	w.pollInterval = d
	return w
}

// WithMaxWait sets the maximum time to wait for batch completion.
func (w *BatchWaiter) WithMaxWait(d time.Duration) *BatchWaiter {
	w.maxWait = d
	return w
}

// Wait blocks until the batch completes or the context is cancelled.
// Returns the final batch info, or an error if the wait times out or fails.
func (w *BatchWaiter) Wait(ctx context.Context, id BatchID) (*BatchInfo, error) {
	deadline := time.Now().Add(w.maxWait)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		info, err := w.provider.GetBatchStatus(ctx, id)
		if err != nil {
			return nil, err
		}

		if info.IsComplete() {
			return info, nil
		}

		if time.Now().After(deadline) {
			return info, ErrBatchTimeout
		}

		select {
		case <-ctx.Done():
			return info, ctx.Err()
		case <-ticker.C:
			// Continue polling
		}
	}
}

// WaitAndCollect waits for completion and retrieves all results.
// This is a convenience method combining Wait and GetBatchResults.
func (w *BatchWaiter) WaitAndCollect(ctx context.Context, id BatchID) ([]BatchResult, error) {
	info, err := w.Wait(ctx, id)
	if err != nil {
		return nil, err
	}

	if info.Status == BatchStatusFailed || info.Status == BatchStatusCancelled {
		// Still try to get partial results
	}

	return w.provider.GetBatchResults(ctx, id)
}
