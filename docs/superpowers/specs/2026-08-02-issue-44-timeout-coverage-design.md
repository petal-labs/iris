# Design: Timeout Coverage & Error Improvements (Issue #44)

- **Date:** 2026-08-02
- **Status:** Approved (pending spec review)
- **Branch:** `feat/44-timeout-coverage`

## Problem

GitHub issue #44 collects five deferred timeout follow-ups:

1. **Non-chat calls have no timeout.** Embeddings, batch, files, vector-stores, and images bypass `core.Client` (callers invoke `provider.CreateEmbeddings(...)` directly) and use the shared `HTTPClient` (`http.DefaultClient`, no timeout). `core.WithTimeout` cannot reach them.
2. **No idle/stall streaming timeout.** Streaming uses an overall deadline only; a stream that stalls mid-flight (or never sends a first byte) is bounded only by the overall deadline.
3. **`ErrTimeout` message lacks provider/model.** `newTimeoutError(d)` produces `iris: execution timeout after 120s` with no indication of which provider/model was in flight.
4. **`tools.WithTimeout` error is untyped.** It returns a bare `fmt.Errorf("tool execution timeout after %v")` — `errors.Is` matches neither `context.DeadlineExceeded` nor `core.ErrTimeout`.
5. **Batch `DeadlineExceeded` misclassification.** `BatchWaiter.Wait` maps *any* `errors.Is(err, context.DeadlineExceeded)` to `ErrBatchTimeout`, so a provider's own transport error that wraps `DeadlineExceeded` loses its detail.

## Decisions (confirmed with maintainer)

- **Item 1:** Revive the per-provider `WithTimeout`/`Config.Timeout` (deprecated in v0.15.0) and make it actually apply to non-chat **unary** operations, generalizing azurefoundry's existing working pattern. `core.WithTimeout` remains the knob for chat/stream.
- **Item 2:** Include the idle/stall streaming timeout as a new `core.WithStreamIdleTimeout(d)` client option.

## Goals

- Every non-chat unary provider call is bounded by a timeout by default.
- Streams that stall are detected and terminated with a legible error.
- Timeout errors name the provider/model; tool timeouts are typed; batch preserves provider errors.

## Non-Goals / Deferred

- Streaming non-chat operations (`StreamImage`, ollama's raw stream) do **not** get an overall op-timeout (an overall deadline would truncate legitimate streams). They rely on the idle timeout (item 2) where wired, or a caller deadline.
- No change to how chat/stream timeouts work (that is `core.WithTimeout`, unchanged).

---

## Item 1 — Non-chat unary timeout coverage

### Shared helper

New package `providers/internal/timeoutx`:

```go
package timeoutx

// Apply returns a context bounded by d, unless d <= 0 or ctx already carries a
// deadline (in which case the caller's deadline wins and the original ctx is
// returned with a no-op cancel). Mirrors core's effectiveTimeout precedence.
func Apply(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
```

### Per-provider wiring

- **Un-deprecate** the `WithTimeout(d) Option` and `Config.Timeout` field on every provider. Replace the "Deprecated: ... inert" doc comments with: *"Sets the timeout applied to this provider's direct (non-chat) operations — embeddings, files, images, batch, vector stores. Chat and streaming honor `core.WithTimeout` and context deadlines instead."*
- **Default:** each provider's `New` initializes `Config.Timeout` to `core.DefaultTimeout` (120s) — matching the chat default and closing the gap by default. `WithTimeout(0)` disables.
- **Apply the guard** at the top of every unary non-chat method (before building the request):
  ```go
  ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout)
  defer cancel()
  ```
  Methods, per provider:
  - **openai:** `CreateEmbeddings`; files (`UploadFile`, `GetFile`, `ListFiles`, `DownloadFile`, `DeleteFile`); images (`GenerateImage`, `EditImage` — NOT `StreamImage`); batch (`CreateBatch`, `GetBatchStatus`, `GetBatchResults`, `CancelBatch`, `ListBatches`); vector-stores (all methods in `client_vectorstores.go`).
  - **ollama:** `CreateEmbeddings` (NOT the raw stream).
  - **voyageai:** `CreateEmbeddings`, `CreateContextualizedEmbeddings`, `Rerank`.
  - **anthropic:** files methods (`client_files.go`).
  - **gemini:** files methods; images `GenerateImage`, `EditImage` (NOT `StreamImage`).
  - **huggingface:** discovery methods (`GetModelStatus`, `GetModelProviders`, `ListModels`).
  - **azurefoundry:** `CreateEmbeddings` — refactor its existing inline `if p.config.Timeout > 0 { ... }` in `client_chat.go`/`embeddings.go` to use `timeoutx.Apply` (which adds the caller-deadline-wins behavior it currently lacks). Leave azurefoundry's chat/stream timeout wiring behavior intact (it may keep applying to chat, which is azurefoundry-specific and harmless).

- **Chat/StreamChat** methods are NOT touched by this item — they go through `core.Client`/`core.WithTimeout`. (azurefoundry is the historical exception and keeps its behavior.)

### Behavior change

Non-chat unary calls now carry a **120s default timeout**. A caller doing a large file upload or batch-result download that legitimately exceeds 120s must either pass a `context` with a longer deadline or construct the provider with `provider.WithTimeout(0)` (disable) or a higher value. This mirrors the v0.15.0 chat-timeout tradeoff and is documented in the change doc's Breaking Changes.

---

## Item 2 — Idle/stall streaming timeout

### API

New `core.WithStreamIdleTimeout(d time.Duration) ClientOption`, stored as `Client.streamIdleTimeout` (default `0` = off). Applies only to `Stream`.

### Mechanism

When `streamIdleTimeout > 0`, `Stream` wraps the provider stream with an idle watchdog:

- A goroutine reads chunks from the provider's `Ch`, forwards each to a new output channel, and resets an idle timer (`time.Timer`, reset to `d`) on every chunk.
- If the timer fires before the next chunk (no data for `d`), the watchdog cancels the stream's context (terminating the provider) and delivers a timeout error on the wrapped `Err` channel.
- The error is `newStreamIdleError(d)` — wraps `core.ErrTimeout` (and `context.DeadlineExceeded`), message `iris: stream idle timeout after <d>: context deadline exceeded`. `errors.Is(err, core.ErrTimeout)` holds.
- The overall deadline (existing `effectiveTimeout`) still applies independently — whichever fires first wins.

This composes with the existing `wrapStreamWithTelemetry`: the idle wrapper sits between the provider stream and the telemetry wrapper (idle wrapper intercepts `Ch`; telemetry wrapper observes `Err`/`Final`). Cancellation is tied to stream lifetime as today (no premature cancel).

### Interaction with item 1

Non-chat streaming (`StreamImage`) is out of scope for the idle timeout in this pass (documented) — it would require the same wrapper at the provider level. Chat streaming (`core.Stream`) is the target.

---

## Item 3 — Provider/model in `ErrTimeout` message

Change `newTimeoutError`:

```go
func newTimeoutError(d time.Duration, provider string, model ModelID) error {
	return fmt.Errorf("iris: %w after %s (provider=%s, model=%s): %w",
		ErrTimeout, d, provider, model, context.DeadlineExceeded)
}
```

Update the 3 call sites in `core/client.go` (lines ~614, ~698, ~728) to pass `providerID` and `b.req.Model` (both already in scope). `errors.Is` semantics unchanged (still wraps `ErrTimeout` + `DeadlineExceeded`).

---

## Item 4 — Typed `tools.WithTimeout` error

`tools` already imports `core` (no cycle). Change the timeout return:

```go
case <-ctx.Done():
	return nil, fmt.Errorf("tool execution timeout after %v: %w: %w",
		d, core.ErrTimeout, context.DeadlineExceeded)
```

Now `errors.Is(err, core.ErrTimeout)` and `errors.Is(err, context.DeadlineExceeded)` both hold.

---

## Item 5 — Batch `DeadlineExceeded` misclassification

In `BatchWaiter.Wait`, after a poll returns an error, key the `ErrBatchTimeout` classification on the **wall-clock `maxWait` deadline**, not on `errors.Is(err, context.DeadlineExceeded)`:

```go
if err != nil {
	if ctx.Err() != nil {
		return nil, ctx.Err() // caller cancellation wins
	}
	if !time.Now().Before(deadline) {
		return nil, ErrBatchTimeout // our maxWait budget is exhausted
	}
	return nil, err // provider's own error — preserve its detail
}
```

A provider transport error that wraps `DeadlineExceeded` but arrives before `maxWait` is exhausted is now returned as-is (with its `ProviderError`/`Body`), instead of being flattened to `ErrBatchTimeout`.

---

## Testing

- **Item 1:** per representative provider, a unary method (e.g. `CreateEmbeddings`) with a short `WithTimeout` against a slow/blocking `httptest` server returns a `DeadlineExceeded`-satisfying error and does not hang; a caller-supplied ctx deadline still wins; `WithTimeout(0)` disables. `timeoutx.Apply` unit tests (d<=0 no-op, caller-deadline wins, applies otherwise).
- **Item 2:** a mock stream that sends one chunk then stalls, with `WithStreamIdleTimeout(50ms)`, surfaces `ErrTimeout` on the stream within ~the idle window (not the overall deadline); a steady stream that keeps sending within the window completes normally; `-race`.
- **Item 3:** `newTimeoutError(d, "openai", "gpt-4o")` message contains provider + model; `errors.Is` for `ErrTimeout` and `DeadlineExceeded` still hold.
- **Item 4:** a tool that exceeds the middleware timeout yields an error satisfying `errors.Is(_, core.ErrTimeout)` and `errors.Is(_, context.DeadlineExceeded)`.
- **Item 5:** a stub batch provider returning a `ProviderError` wrapping `DeadlineExceeded` *before* `maxWait` → `Wait` returns that `ProviderError` (not `ErrBatchTimeout`); a genuinely stuck poll past `maxWait` → `ErrBatchTimeout`.

## Breaking Changes & Migration

- **120s default op-timeout on non-chat unary calls.** Large uploads/downloads exceeding 120s now fail with a deadline error unless the caller passes a longer ctx deadline or uses `provider.WithTimeout(0)`/a higher value. Per-provider `WithTimeout`/`Config.Timeout` are **un-deprecated** and now functional for these paths.
- New exported symbols: `core.WithStreamIdleTimeout`. No existing signatures change (`newTimeoutError` is unexported).

## Deferred / Out of Scope

- Idle timeout for non-chat streaming (`StreamImage`).
- Per-method timeout overrides (only a per-provider default is added).

## Rollout

Minor release (`v0.17.0`, next after v0.16.0). Land the small items (3, 4, 5) first, then item 2, then item 1 (largest).
