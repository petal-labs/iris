// Package timeoutx applies per-provider timeouts to direct (non-chat) calls.
package timeoutx

import (
	"context"
	"time"
)

// Default is the default per-provider timeout for non-chat unary operations.
const Default = 600 * time.Second

// Apply bounds ctx by d unless d <= 0 or ctx already has a deadline (caller
// wins). Returns the ctx and a cancel func (a no-op when nothing was applied).
func Apply(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
