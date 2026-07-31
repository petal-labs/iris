# Design: Core-Level LLM Execution Timeout

- **Date:** 2026-07-31
- **Status:** Approved
- **Branch:** `feat/core-execution-timeout`

## Problem

Iris applies **no execution timeout** to LLM calls by default. Concretely:

- Every provider defaults its HTTP client to `http.DefaultClient`, which has `Timeout: 0`
  (`providers/openai/provider.go:45-50`).
- OpenAI (and every other provider except `azurefoundry`) exposes a `WithTimeout` option whose
  `Config.Timeout` field is **never read** — it is dead code (`providers/openai/options.go:79-84`).
- The core only imposes a deadline in the non-streaming `GetResponse`, and only if the caller
  explicitly calls `ChatBuilder.Timeout()` (`core/client.go:461-468`). Streaming has no core-level
  timeout at all, by deliberate design (`core/client.go:539-545`).

Result: a slow or hung provider call blocks indefinitely unless the caller happens to pass a
`context` with a deadline. In the field, an application with its own short timeout window cancelled
the context while Iris was still blocked in the OpenAI call. Iris produced no error of its own — it
inherited the app's opaque cancellation, so the real failure was invisible.

This is a single root cause: **no Iris-owned execution timeout → long call → external cancellation →
opaque failure.**

## Goals

1. Iris fails fast with a **legible, typed error** before an external short timeout can mask it.
2. The mechanism covers **all providers** (chat + streaming) without per-provider duplication.
3. A **sensible default** is applied out of the box so a caller who configures nothing is still safe.
4. No regression for callers who already pass a `context` deadline.

## Non-Goals

- The separate embeddings path (`EmbeddingProvider`) is **out of scope** — see Deferred.
- Idle/stall ("time to first byte") streaming semantics are out of scope; streaming uses an overall
  deadline (see Streaming below).
- No live OpenAI round-trip verification — credentials live in Azure Key Vault, unreachable from the
  dev/CI environment. Verification is via `httptest` mock servers.

## Chosen Approach

**Centralize the timeout in `core`.** This aligns with Iris's stated design principle that "Core
never imports provider packages directly" — one provider-agnostic implementation covers all current
and future providers. Rejected alternatives:

- **Per-provider wiring** (the `azurefoundry` pattern): duplicates the same `context.WithTimeout`
  block into every provider's chat/stream/embeddings methods — 10× the code and 10× the chance of
  missing a path.
- **`http.Client{Timeout: …}` default:** `http.Client.Timeout` aborts the whole request including a
  mid-stream body read, which would truncate legitimately long streams. Wrong semantics for
  streaming providers.

## Design

### 1. Client option and default

```go
// WithTimeout sets the default execution timeout applied to chat and streaming
// calls when the caller's context has no deadline of its own. Pass 0 to disable
// the default and allow calls to run without a timeout.
func WithTimeout(d time.Duration) ClientOption
```

- Stored as a new field on `Client`: `timeout time.Duration`.
- `NewClient` initializes it to `DefaultTimeout = 120 * time.Second`.
- `WithTimeout(0)` explicitly disables the default (restores today's unbounded behavior).

### 2. Precedence

For every `GetResponse` and `Stream` call, resolve the effective timeout in this order:

1. **Caller's `ctx` already has a deadline** → do nothing. The caller wins; no wrapping. (No regression.)
2. Else **`ChatBuilder.Timeout()`** if set (> 0).
3. Else the **Client default** (`c.timeout`, 120s unless overridden or disabled).
4. If the resolved value is 0 → no timeout applied.

A small helper centralizes this:

```go
// effectiveTimeout returns the timeout to apply, or 0 for "none".
// Returns 0 when ctx already has a deadline (caller owns cancellation).
func (b *ChatBuilder) effectiveTimeout(ctx context.Context) time.Duration
```

### 3. `GetResponse` (unary)

Replace the current `if b.timeout > 0 { … }` block with a call through `effectiveTimeout`. The
context is wrapped once and covers the whole retry loop (unchanged budget semantics — the deadline is
an overall budget across attempts, matching existing behavior). On expiry, the returned error is
mapped to `ErrTimeout` (see §5).

### 4. `Stream` (streaming) — cancel tied to stream lifetime

`provider.StreamChat(ctx, req)` returns a `*ChatStream` whose channels the caller drains *after*
`Stream()` returns. A naive `defer cancel()` in `Stream()` would fire at method return and kill the
stream immediately — which is exactly why streaming timeouts were skipped originally.

Fix: when a timeout applies, create `ctx, cancel = context.WithTimeout(parent, d)`, pass the derived
`ctx` to `StreamChat`, and hand `cancel` to a wrapper that invokes it when the stream terminates:

- The existing `wrapStreamWithTelemetry` already observes stream completion. The timeout `cancel` is
  invoked from the same completion point (all of `Ch`/`Err`/`Final` closed), OR the deadline fires on
  its own.
- Net effect: a fast/normal stream runs to completion and then `cancel` is called (releasing the
  timer); a stalled stream is torn down when the deadline fires, and the caller observes `ErrTimeout`
  on the `Err` channel / from `DrainStream`.

This keeps the `ChatStream` channel contract intact ("on context cancellation, providers MUST
terminate promptly and close channels", `core/streaming.go:10-13`).

### 5. Typed error

```go
// ErrTimeout indicates an Iris-imposed execution timeout elapsed before the
// provider call completed. It wraps context.DeadlineExceeded.
var ErrTimeout = ...
```

- `errors.Is(err, ErrTimeout)` and `errors.Is(err, context.DeadlineExceeded)` both hold.
- At the core boundary, when the applied timeout is the cause, map `context.DeadlineExceeded` →
  `ErrTimeout` with a message like `iris: execution timeout after 120s`.
- Distinguish from caller cancellation: a caller-owned deadline or `context.Canceled` is **not**
  remapped — only the Iris-applied deadline surfaces as `ErrTimeout`.
- `retry.go` already treats `context.DeadlineExceeded` as non-retryable; wrapping preserves that via
  `errors.Is`.

### 6. Deprecate dead per-provider `Config.Timeout`

The per-provider `Config.Timeout` / `WithTimeout` are inert on every provider except `azurefoundry`.
Mark them **deprecated** in doc comments, pointing callers to `core.WithTimeout`. No signatures
change — non-breaking. `azurefoundry`'s own wiring is left as-is; it composes harmlessly with the
core deadline (whichever is shorter wins).

## Data Flow

```
client.Chat(model).User("…").GetResponse(ctx)
  └─ effectiveTimeout(ctx):
       ctx has deadline?  → 0  (no wrap)
       builder.timeout>0? → builder.timeout
       else               → client.timeout (120s default / 0 disabled)
  └─ if >0: ctx, cancel = WithTimeout(ctx, d); defer cancel()
  └─ retry loop → provider.Chat(ctx, req)
  └─ on deadline: map DeadlineExceeded → ErrTimeout

client.Chat(model).User("…").Stream(ctx)
  └─ effectiveTimeout(ctx) → d
  └─ if d>0: ctx, cancel = WithTimeout(ctx, d)   (cancel NOT deferred here)
  └─ provider.StreamChat(ctx, req)
  └─ wrapStream(…, cancel): invoke cancel when Ch/Err/Final all close
  └─ on deadline: ErrTimeout delivered on Err channel
```

## Testing

Table-driven tests in `core` using a mock provider that blocks on a controllable signal:

- Unary: default (120s, shortened in test) fires → `ErrTimeout`; per-call `.Timeout()` override wins;
  caller ctx deadline respected (not remapped to `ErrTimeout`); `WithTimeout(0)` disables (no timeout);
  fast call under the deadline succeeds.
- Streaming: a fast stream completes and is **not** cancelled prematurely; a stalled stream times out
  and delivers `ErrTimeout` on `Err`; `cancel` is invoked exactly once (no goroutine leak).
- Error typing: `errors.Is(err, ErrTimeout)` and `errors.Is(err, context.DeadlineExceeded)` both true;
  a `context.Canceled` from the caller is not remapped.

`gpt-5.4-mini` verification (`providers/openai`): an `httptest` server asserts the request lands on
`/responses` (Responses API) with `"model":"gpt-5.4-mini"`, and a canned Responses payload is parsed
into a `core.ChatResponse`. This exercises the exact code path the model uses, without live
credentials.

## Deferred / Out of Scope

- **Embeddings timeout** (`EmbeddingProvider`): not routed through `core.Client`, so the core default
  does not cover it. Tracked as a follow-up; would be a small addition mirroring this design.
- **Idle/stall streaming timeout:** overall-deadline only for now.
- **Live OpenAI round-trip:** must be run in an environment with Key Vault access.

## Rollout / Compatibility

- **Behavior change:** callers who set neither a ctx deadline nor a timeout, and who make calls longer
  than 120s, will now receive `ErrTimeout` where they previously blocked. This is the intended fix.
  Opt out with `core.WithTimeout(0)` or raise the value with `core.WithTimeout(d)`.
- API is purely additive (`WithTimeout`, `ErrTimeout`, `DefaultTimeout`); no existing signatures change.
