package core

import (
	"context"
	"time"
)

// wrapStreamWithIdleTimeout wraps stream with a stall/idle watchdog: if no
// chunk arrives on stream.Ch within d, the watchdog calls cancel (which MUST
// terminate the underlying provider call per the ChatStream contract in
// streaming.go) and delivers newStreamIdleError(d) on the returned stream's
// Err channel.
//
// The watchdog runs a single goroutine that forwards Ch/Err/Final losslessly,
// mirroring the fully-symmetric chClosed/errResolved/finalResolved discipline
// used by wrapStreamWithTelemetry and DrainStream: whichever of a genuine
// provider error/final response or the idle timer resolves first wins, and
// none of the three channels is ever assumed "done" just because another one
// fired (the earlier DrainStream bug this guards against was concluding the
// stream was over as soon as Ch closed, dropping an Err that arrived a moment
// later).
//
// cancel is invoked by the watchdog at most once, exactly when the idle timer
// fires. The caller (ChatBuilder.Stream) remains responsible for also
// invoking it once the stream completes normally so the derived context is
// always released; context.CancelFunc is idempotent, so a redundant call is
// safe and does not double-fire anything here.
func wrapStreamWithIdleTimeout(stream *ChatStream, d time.Duration, cancel context.CancelFunc) *ChatStream {
	outCh := make(chan ChatChunk)
	errCh := make(chan error, 1)
	finalCh := make(chan *ChatResponse, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)
		defer close(finalCh)

		timer := time.NewTimer(d)
		defer timer.Stop()

		chClosed := false
		errResolved := false
		finalResolved := false

		for !chClosed || !errResolved || !finalResolved {
			var chRecv <-chan ChatChunk
			if !chClosed {
				chRecv = stream.Ch
			}
			var errRecv <-chan error
			if !errResolved {
				errRecv = stream.Err
			}
			var finalRecv <-chan *ChatResponse
			if !finalResolved {
				finalRecv = stream.Final
			}

			select {
			case chunk, ok := <-chRecv:
				if !ok {
					chClosed = true
					// No more chunks are coming, so the idle watchdog no
					// longer applies. Stop the timer (draining a pending
					// fire if one raced the close) so it cannot spuriously
					// fire while we wait out any remaining Err/Final.
					if !timer.Stop() {
						<-timer.C
					}
					continue
				}
				outCh <- chunk
				// Reset the timer using the standard stop-drain-reset
				// dance: Stop reports false when the timer already fired
				// (its value sitting unread in timer.C, since this
				// goroutine is the channel's only reader) or was already
				// stopped, in which case the pending value must be
				// drained before Reset to avoid a stale, immediate fire.
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(d)

			case err, ok := <-errRecv:
				errResolved = true
				if ok {
					errCh <- err
				}
				// If Err closed without a value, keep waiting on Ch/Final.

			case resp, ok := <-finalRecv:
				finalResolved = true
				if ok {
					finalCh <- resp
				}
				// If Final closed without a value, that alone isn't an
				// error; Err is tracked independently above.

			case <-timer.C:
				cancel()
				errCh <- newStreamIdleError(d)
				return
			}
		}
	}()

	return &ChatStream{Ch: outCh, Err: errCh, Final: finalCh}
}
