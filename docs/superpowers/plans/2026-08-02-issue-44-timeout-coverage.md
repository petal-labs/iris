# Issue #44 Timeout Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Resolve issue #44 — non-chat timeout coverage, idle/stall streaming timeout, provider/model in `ErrTimeout`, typed `tools.WithTimeout`, and batch error preservation.

**Architecture:** A shared `providers/internal/timeoutx` helper applies a per-provider op-timeout to non-chat unary methods (revived `WithTimeout`). A core streaming idle watchdog enforces `WithStreamIdleTimeout`. Three small in-place fixes for the error message, tool error, and batch classification.

**Tech Stack:** Go 1.24, stdlib + `net/http/httptest`.

## Global Constraints

- Module `github.com/petal-labs/iris` (go 1.24). Branch: `feat/44-timeout-coverage`.
- New exports: `core.WithStreamIdleTimeout(d) ClientOption`; `providers/internal/timeoutx` package (`Default` const = 600s, `Apply`).
- Un-deprecate per-provider `WithTimeout`/`Config.Timeout`; default `Config.Timeout` to `timeoutx.Default` (600s) in every provider `New`.
- **Op-timeout applies to UNARY non-chat methods only.** Exclude: any streaming method (`StreamImage`, ollama raw stream) and **`openai.DownloadFile`** (returns `io.ReadCloser` consumed after return — `defer cancel()` would truncate it). Chat/StreamChat are NOT touched (they use `core.WithTimeout`).
- Conventional commits, subject ≤ 72 chars, no trailing period/backticks/emoji.
- Every task ends green: `go test ./...` (`-race` where noted), `go build ./...`, `gofmt -l` clean.

---

### Task 1: Provider/model in ErrTimeout message (item 3)

**Files:** `core/errors.go`, `core/client.go`, `core/errors_test.go`

- [ ] **Step 1: Failing test** — update `core/errors_test.go` `TestNewTimeoutError` (currently calls `newTimeoutError(120*time.Second)`): change to `newTimeoutError(120*time.Second, "openai", "gpt-4o")` and assert the message contains `"openai"` and `"gpt-4o"`, plus the existing `errors.Is(err, ErrTimeout)` and `errors.Is(err, context.DeadlineExceeded)` checks.
- [ ] **Step 2: Run — expect FAIL** (signature mismatch).
- [ ] **Step 3: Implement** — change `core/errors.go`:

```go
func newTimeoutError(d time.Duration, provider string, model ModelID) error {
	return fmt.Errorf("iris: %w after %s (provider=%s, model=%s): %w",
		ErrTimeout, d, provider, model, context.DeadlineExceeded)
}
```

Update the 3 call sites in `core/client.go` (grep `rg -n "newTimeoutError(" core/client.go`) to pass `providerID` and `b.req.Model` (both in scope at each site — verify by reading each). `ModelID` is `core.ModelID`; the file is package `core` so use `ModelID`.

- [ ] **Step 4: Run** `go test ./core/ -run 'Timeout' -v` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(core): name provider and model in timeout errors"`

---

### Task 2: Typed tools.WithTimeout error (item 4)

**Files:** `tools/middleware_timeout.go`, `tools/middleware_timeout_test.go` (create if absent)

- [ ] **Step 1: Failing test** — add a test: a tool func that blocks past a short `WithTimeout`, wrapped via the middleware, returns an error where `errors.Is(err, core.ErrTimeout)` AND `errors.Is(err, context.DeadlineExceeded)` both hold. (Read an existing tools test for the `Middleware`/`ToolCallFunc` shapes; `tools` imports `core` already.)
- [ ] **Step 2: Run — expect FAIL.**
- [ ] **Step 3: Implement** — in `tools/middleware_timeout.go`, change the `case <-ctx.Done():` return to:

```go
	case <-ctx.Done():
		return nil, fmt.Errorf("tool execution timeout after %v: %w: %w",
			d, core.ErrTimeout, context.DeadlineExceeded)
```

Add the `core` import (`github.com/petal-labs/iris/core`).

- [ ] **Step 4: Run** `go test ./tools/ -run 'Timeout' -v` then `go test ./tools/` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(tools): make WithTimeout error satisfy ErrTimeout"`

---

### Task 3: Batch DeadlineExceeded classification (item 5)

**Files:** `core/batch.go`, `core/batch_test.go`

- [ ] **Step 1: Failing test** — add a stub `BatchProvider` whose `GetBatchStatus` returns a `*core.ProviderError` wrapping `context.DeadlineExceeded` (e.g. `&ProviderError{Provider:"x", Message:"transport", Err: fmt.Errorf("%w: %w", ErrNetwork, context.DeadlineExceeded)}`) IMMEDIATELY (before `maxWait`). Assert `Wait` returns THAT ProviderError (errors.As to `*ProviderError`), NOT `ErrBatchTimeout`. Keep/confirm the existing "stuck poll past maxWait → ErrBatchTimeout" test still passes.
- [ ] **Step 2: Run — expect FAIL** (currently flattened to `ErrBatchTimeout`).
- [ ] **Step 3: Implement** — in `Wait`, replace the post-poll error block:

```go
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !time.Now().Before(deadline) {
			return nil, ErrBatchTimeout
		}
		return nil, err
	}
```

(Removes the `errors.Is(err, context.DeadlineExceeded)` → `ErrBatchTimeout` mapping; keys on the wall-clock `deadline` instead. `errors` import may become unused in batch.go — remove it if so.)

- [ ] **Step 4: Run** `go test ./core/ -run 'Batch' -v -timeout 30s` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "fix(core): preserve provider error over batch timeout"`

---

### Task 4: Idle/stall streaming timeout (item 2)

**Files:** `core/client.go` (option + Stream wiring), `core/streaming.go` or a new `core/stream_idle.go`, `core/errors.go` (idle error helper), test file

**Interfaces:**
- Produces: `func WithStreamIdleTimeout(d time.Duration) ClientOption`; `Client.streamIdleTimeout time.Duration` (default 0); an idle-wrapping helper.

- [ ] **Step 1: Failing test** — add to a core test: a mock provider whose `StreamChat` returns a `*ChatStream` that sends one chunk then STALLS (never closes, never sends more) until ctx is cancelled. With `NewClient(prov, WithStreamIdleTimeout(50*time.Millisecond))`, `Stream(ctx)` then `DrainStream` returns an error satisfying `errors.Is(err, ErrTimeout)` within ~the idle window. Add a second case: a stream that sends chunks every ~10ms for a few chunks then closes cleanly completes with NO error under a 50ms idle timeout. Run under `-race`.
- [ ] **Step 2: Run — expect FAIL** (option undefined; stall hangs).
- [ ] **Step 3: Implement**
  - `core/errors.go`: add `newStreamIdleError(d time.Duration) error` → `fmt.Errorf("iris: stream idle %w after %s: %w", ErrTimeout, d, context.DeadlineExceeded)` (so `errors.Is(_, ErrTimeout)` holds).
  - `core/client.go`: add `streamIdleTimeout time.Duration` to `Client`; `WithStreamIdleTimeout(d)` option; in `Stream`, when `b.client.streamIdleTimeout > 0`, derive a cancelable `ctx, cancel := context.WithCancel(ctx)` passed to `StreamChat`, and wrap the returned stream with the idle watchdog BEFORE `wrapStreamWithTelemetry`.
  - Idle watchdog (new helper `wrapStreamWithIdleTimeout(stream *ChatStream, d time.Duration, cancel context.CancelFunc) *ChatStream`): spawn a goroutine that `select`s on the original `stream.Ch` and a `time.NewTimer(d)`: on a chunk, forward it to a new `outCh` and reset the timer; on timer fire, call `cancel()`, send `newStreamIdleError(d)` on a new `errCh`, and stop; on `stream.Ch` close, forward remaining `Err`/`Final` and close. Return a `*ChatStream` with the new `outCh`/`errCh` and the original `Final` (or a forwarded one). Model the loop on `wrapStreamWithTelemetry` (`core/client.go`) for the Ch/Err/Final forwarding discipline, and ensure the timer is stopped and no goroutine leaks (test with `-race`).
  > This is the subtle part — study `wrapStreamWithTelemetry` and `DrainStream` first. The idle wrapper must not swallow a real late error (same class of bug as the earlier DrainStream fix) and must cancel exactly once.
- [ ] **Step 4: Run** `go test ./core/ -run 'Idle|Stream' -v` then `go test -race ./core/ -run 'Idle|Stream' -count=10` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(core): add stream idle timeout option"`

---

### Task 5: timeoutx helper + un-deprecate + default wiring (item 1 foundation)

**Files:** Create `providers/internal/timeoutx/timeoutx.go` + `_test.go`; modify every provider's `options.go` (un-deprecate docs) and `provider.go` (`New` default).

- [ ] **Step 1: Failing test** — `providers/internal/timeoutx/timeoutx_test.go`:

```go
func TestApply(t *testing.T) {
	// d <= 0 -> no-op, original ctx
	ctx, cancel := Apply(context.Background(), 0)
	cancel()
	if _, ok := ctx.Deadline(); ok { t.Error("d=0 should not add a deadline") }
	// caller deadline wins
	base, c := context.WithTimeout(context.Background(), time.Hour)
	defer c()
	ctx2, cancel2 := Apply(base, time.Second)
	cancel2()
	dl, _ := ctx2.Deadline()
	if time.Until(dl) < 30*time.Minute { t.Error("caller deadline should win") }
	// applies otherwise
	ctx3, cancel3 := Apply(context.Background(), time.Second)
	defer cancel3()
	if _, ok := ctx3.Deadline(); !ok { t.Error("should apply a deadline") }
}
```

- [ ] **Step 2: Run — expect FAIL.**
- [ ] **Step 3: Implement** `providers/internal/timeoutx/timeoutx.go`:

```go
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
```

- [ ] **Step 4: Un-deprecate + default** — for each provider (openai, anthropic, gemini, xai, perplexity, ollama, huggingface, voyageai, zai, azurefoundry):
  - In `options.go`: replace the `Deprecated:` note on `WithTimeout` and the `Config.Timeout` field with: `// WithTimeout sets the timeout for this provider's direct (non-chat) operations — embeddings, files, images, batch, and vector stores. Chat and streaming honor core.WithTimeout and context deadlines.`
  - In `provider.go` `New`: set `Timeout: timeoutx.Default` in the `Config` struct literal (add the `timeoutx` import). For providers whose `Config` had no explicit `Timeout` init (defaulted 0), add it. For azurefoundry (already had `Timeout`), set the default to `timeoutx.Default`.
  - **Read each file first**; only change the Timeout default and the doc comments — do not touch other fields.
- [ ] **Step 5: Run** `go build ./...`, `go test ./providers/internal/timeoutx/ -v`, `go test ./...` — expect PASS (no method applies it yet, so behavior is unchanged except the field default; confirm no test asserted `Config.Timeout == 0` after `New`).
- [ ] **Step 6: Commit** `git commit -m "feat(providers): add timeoutx helper and revive provider timeout"`

---

### Task 6: Apply op-timeout to OpenAI unary methods (item 1a)

**Files:** `providers/openai/{client_embeddings.go,client_files.go,client_image.go,client_batch.go,client_vectorstores.go}`; test `providers/openai/timeout_test.go` (new)

**Interfaces:** Consumes `timeoutx.Apply` + `p.config.Timeout` (Task 5).

- [ ] **Step 1: Failing test** — `providers/openai/timeout_test.go`: an httptest server that blocks (`<-r.Context().Done()`). Build `New("k", WithBaseURL(srv.URL), WithTimeout(50*time.Millisecond))`. Call `CreateEmbeddings(context.Background(), ...)`; assert it returns within ~1s with an error satisfying `errors.Is(err, context.DeadlineExceeded)` (via the network error chain) — i.e. it did not hang. Add a case asserting a caller ctx with a longer deadline is respected and one that `WithTimeout(0)` disables (call returns the server's response, not a timeout — use a fast server for that case).
- [ ] **Step 2: Run — expect FAIL** (call hangs / no timeout applied).
- [ ] **Step 3: Implement** — at the top of each of these methods (before building the HTTP request), add:

```go
	ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout)
	defer cancel()
```

Methods: `CreateEmbeddings`; `UploadFile`, `GetFile`, `ListFiles`, `DeleteFile` (files — **NOT `DownloadFile`**, which returns `io.ReadCloser`); `GenerateImage`, `EditImage` (**NOT `StreamImage`**); `CreateBatch`, `GetBatchStatus`, `GetBatchResults`, `CancelBatch`, `ListBatches`; all methods in `client_vectorstores.go`. Add the `timeoutx` import to each file. Read each method to place the guard after any `requireAPIKey()` check and before request building.

- [ ] **Step 4: Run** `go test ./providers/openai/ -run 'Timeout' -v` then `go test ./providers/openai/` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(openai): apply op timeout to non-chat unary calls"`

---

### Task 7: Apply op-timeout to other providers (item 1b)

**Files:** `providers/ollama/embeddings.go`; `providers/voyageai/{client_embeddings.go,client_contextualized.go,client_rerank.go}`; `providers/anthropic/client_files.go`; `providers/gemini/{client_files.go,client_image.go}`; `providers/huggingface/discovery.go`; `providers/azurefoundry/embeddings.go`; tests where practical

- [ ] **Step 1: Failing test** — add a timeout test for at least ollama `CreateEmbeddings` and voyageai `CreateEmbeddings` (mirror Task 6's blocking-server test). These prove the pattern; the rest rely on `go test ./...` compile coverage + the shared helper's own tests.
- [ ] **Step 2: Run — expect FAIL.**
- [ ] **Step 3: Implement** — add the `timeoutx.Apply(ctx, p.config.Timeout)` + `defer cancel()` guard at the top of:
  - ollama `CreateEmbeddings` (NOT the raw stream method).
  - voyageai `CreateEmbeddings`, `CreateContextualizedEmbeddings`, `Rerank`.
  - anthropic files methods (`UploadFile`, `GetFile`, `ListFiles`, `DeleteFile` — exclude any returning an unbuffered reader; check `DownloadFile` if present and exclude it like openai's).
  - gemini files methods (same reader-return exclusion rule) and `GenerateImage`, `EditImage` (NOT `StreamImage`).
  - huggingface discovery methods (`GetModelStatus`, `GetModelProviders`, `ListModels`).
  - azurefoundry `CreateEmbeddings`: refactor the existing inline `if p.config.Timeout > 0 { ctx, cancel = context.WithTimeout(...) }` to `ctx, cancel := timeoutx.Apply(ctx, p.config.Timeout); defer cancel()` (adds caller-deadline-wins). Leave azurefoundry chat/stream wiring as-is.
  Add the `timeoutx` import to each file. Read each method; place the guard after any auth/RequireAPIKey check, before request building. For any method returning an `io.ReadCloser`/stream, DO NOT add the guard (note it in the report).
- [ ] **Step 4: Run** `go test ./providers/... -run 'Timeout' -v` then `go test ./...` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(providers): apply op timeout to non-chat calls"`

---

### Task 8: Change doc + verification

**Files:** Create `docs/changes/2026-08-02_v0.17.0_issue-44-timeout-coverage.md`

- [ ] **Step 1:** `go test -race ./...` — expect PASS.
- [ ] **Step 2:** `go build ./...`; `gofmt -l .` (ignore pre-existing untracked files).
- [ ] **Step 3:** Change doc per repo `CLAUDE.md` structure. `product: iris`, `change_type: feature`, version `v0.17.0`. `affected_components` lists every file changed across Tasks 1-7 (use `git diff --stat main...HEAD`). Document: item 3 (error message), item 4 (tool error typing), item 5 (batch fix), item 2 (`WithStreamIdleTimeout`), item 1 (`timeoutx`, un-deprecated provider `WithTimeout`, 600s default op-timeout, per-method coverage + exclusions: streaming, `DownloadFile`). "Breaking Changes & Migration": the 600s default op-timeout on non-chat unary calls (raise/disable via `provider.WithTimeout`), and the un-deprecation. "Deferred": idle timeout for `StreamImage`; per-method timeout overrides.
- [ ] **Step 4: Commit** `git commit -m "docs: change doc for issue 44 timeout coverage"`

---

## Self-Review

**Spec coverage:** item 3 → T1, item 4 → T2, item 5 → T3, item 2 → T4, item 1 → T5 (foundation) + T6 (openai) + T7 (others), change doc → T8. Exclusions (streaming, `DownloadFile`, reader-returning methods) encoded in Global Constraints + T6/T7.

**Placeholder scan:** No TBD. Runtime lookups (call-site enumeration, reader-returning methods per provider) are called out with explicit read/grep instructions and the exclusion rule.

**Type consistency:** `timeoutx.Apply(ctx, d) (context.Context, context.CancelFunc)`, `timeoutx.Default`, `WithStreamIdleTimeout(d) ClientOption`, `newTimeoutError(d, provider, model)`, `newStreamIdleError(d)` used consistently. Op-timeout excludes chat/stream and reader-returning methods throughout.
