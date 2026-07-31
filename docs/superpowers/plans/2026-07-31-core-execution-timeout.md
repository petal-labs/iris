# Core-Level LLM Execution Timeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Iris a provider-agnostic execution timeout so hung/slow LLM calls fail fast with a legible typed error instead of blocking until an external caller cancels them opaquely.

**Architecture:** Centralize the timeout in `core` (not per-provider). A new `core.WithTimeout` Client option sets a default execution budget (120s) applied to both unary (`GetResponse`) and streaming (`Stream`) calls for every provider. The caller's own `context` deadline always wins; a per-call `ChatBuilder.Timeout()` overrides the client default. Expiry surfaces as `core.ErrTimeout` (wrapping `context.DeadlineExceeded`).

**Tech Stack:** Go 1.24, standard library only (`context`, `errors`, `fmt`, `time`, `net/http/httptest` for tests).

## Global Constraints

- Module path: `github.com/petal-labs/iris` (go 1.24.0).
- Default execution timeout: `DefaultTimeout = 120 * time.Second`.
- `core.WithTimeout(0)` disables the default (unbounded, today's behavior).
- Precedence (highest first): caller ctx already has a deadline → untouched; else `ChatBuilder.Timeout()`; else Client default.
- API is additive only — no existing exported signature changes. New exports: `WithTimeout`, `DefaultTimeout`, `ErrTimeout`.
- Only the Iris-applied deadline is remapped to `ErrTimeout`; a caller-owned deadline or `context.Canceled` is never remapped.
- Conventional commits, subject ≤ 72 chars, no trailing period, no backticks/emoji.
- Tests: Go standard `testing`, table-driven where natural. Use `filepath.Join` for any paths (none expected here).

---

### Task 1: Typed timeout error

**Files:**
- Modify: `core/errors.go`
- Test: `core/errors_test.go`

**Interfaces:**
- Produces:
  - `var ErrTimeout = errors.New("execution timeout")`
  - `func newTimeoutError(d time.Duration) error` — returns an error where `errors.Is(err, ErrTimeout)` and `errors.Is(err, context.DeadlineExceeded)` are both true, message `iris: execution timeout after <d>: context deadline exceeded`.

- [ ] **Step 1: Write the failing test**

Add to `core/errors_test.go`:

```go
func TestNewTimeoutError(t *testing.T) {
	err := newTimeoutError(120 * time.Second)

	if !errors.Is(err, ErrTimeout) {
		t.Errorf("errors.Is(err, ErrTimeout) = false, want true")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, want true")
	}
	if !strings.Contains(err.Error(), "2m0s") {
		t.Errorf("err.Error() = %q, want it to contain the duration", err.Error())
	}
}
```

Ensure the test file imports `context`, `errors`, `strings`, `testing`, `time` (add any missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run TestNewTimeoutError -v`
Expected: FAIL — `undefined: newTimeoutError` (and `ErrTimeout`).

- [ ] **Step 3: Write minimal implementation**

In `core/errors.go`, add `context` and `time` to the import block, then append:

```go
// ErrTimeout indicates an Iris-imposed execution timeout elapsed before the
// provider call completed. It wraps context.DeadlineExceeded, so
// errors.Is(err, context.DeadlineExceeded) also holds.
var ErrTimeout = errors.New("execution timeout")

// newTimeoutError builds a timeout error carrying the elapsed budget. The
// returned error satisfies errors.Is for both ErrTimeout and
// context.DeadlineExceeded.
func newTimeoutError(d time.Duration) error {
	return fmt.Errorf("iris: %w after %s: %w", ErrTimeout, d, context.DeadlineExceeded)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./core/ -run TestNewTimeoutError -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/errors.go core/errors_test.go
git commit -m "feat(core): add ErrTimeout typed execution timeout error"
```

---

### Task 2: Client timeout field, WithTimeout option, effectiveTimeout helper

**Files:**
- Modify: `core/client.go` (Client struct ~44-49; NewClient ~59-70; add option near other `With*` options ~72-98; add helper near `validate` ~433)
- Test: `core/timeout_test.go` (new)

**Interfaces:**
- Consumes: `ErrTimeout`, `newTimeoutError` from Task 1 (not yet, used in Tasks 3-4).
- Produces:
  - `const DefaultTimeout = 120 * time.Second`
  - `func WithTimeout(d time.Duration) ClientOption`
  - new field `timeout time.Duration` on `Client`
  - `func (b *ChatBuilder) effectiveTimeout(ctx context.Context) time.Duration` — returns 0 when ctx already has a deadline; else `b.timeout` if > 0; else `b.client.timeout`.

- [ ] **Step 1: Write the failing test**

Create `core/timeout_test.go`:

```go
package core

import (
	"context"
	"testing"
	"time"
)

func TestNewClientDefaultTimeout(t *testing.T) {
	c := NewClient(&mockProvider{})
	if c.timeout != DefaultTimeout {
		t.Errorf("default timeout = %v, want %v", c.timeout, DefaultTimeout)
	}
}

func TestWithTimeoutOverridesDefault(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(5*time.Second))
	if c.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", c.timeout)
	}
}

func TestWithTimeoutZeroDisables(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(0))
	if c.timeout != 0 {
		t.Errorf("timeout = %v, want 0 (disabled)", c.timeout)
	}
}

func TestEffectiveTimeoutPrecedence(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(30*time.Second))

	// Caller ctx has a deadline -> 0 (caller wins).
	ctxWithDeadline, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b1 := c.Chat("m")
	if got := b1.effectiveTimeout(ctxWithDeadline); got != 0 {
		t.Errorf("with caller deadline = %v, want 0", got)
	}

	// Builder timeout set -> builder wins over client default.
	b2 := c.Chat("m")
	b2.timeout = 10 * time.Second
	if got := b2.effectiveTimeout(context.Background()); got != 10*time.Second {
		t.Errorf("builder override = %v, want 10s", got)
	}

	// Neither -> client default.
	b3 := c.Chat("m")
	if got := b3.effectiveTimeout(context.Background()); got != 30*time.Second {
		t.Errorf("client default = %v, want 30s", got)
	}
}
```

(`mockProvider` already exists in `core/client_test.go`, same package.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run 'Timeout' -v`
Expected: FAIL — `c.timeout undefined`, `DefaultTimeout undefined`, `WithTimeout undefined`, `effectiveTimeout undefined`.

- [ ] **Step 3: Write minimal implementation**

In `core/client.go`:

Add the constant near the top of the file (after imports):

```go
// DefaultTimeout is the execution timeout applied to chat and streaming calls
// when neither the caller's context nor a per-call Timeout() supplies one.
// Disable it per client with WithTimeout(0).
const DefaultTimeout = 120 * time.Second
```

Add the field to the `Client` struct:

```go
type Client struct {
	provider       Provider
	telemetry      TelemetryHook
	retry          RetryPolicy
	warningHandler WarningHandler
	timeout        time.Duration
}
```

Initialize it in `NewClient` before the option loop:

```go
	c := &Client{
		provider:       p,
		telemetry:      NoopTelemetryHook{},
		retry:          DefaultRetryPolicy(),
		warningHandler: func(string) {},
		timeout:        DefaultTimeout,
	}
```

Add the option alongside the other `With*` options:

```go
// WithTimeout sets the default execution timeout applied to chat and streaming
// calls when the caller's context has no deadline of its own and no per-call
// Timeout() is set. Pass 0 to disable the default and allow unbounded calls.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = d
	}
}
```

Add the helper near `validate` (around line 433):

```go
// effectiveTimeout resolves the timeout to apply for this call, or 0 for none.
// Precedence: an existing ctx deadline wins (returns 0, no wrapping); then a
// per-call builder timeout; then the client default.
func (b *ChatBuilder) effectiveTimeout(ctx context.Context) time.Duration {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return 0
	}
	if b.timeout > 0 {
		return b.timeout
	}
	return b.client.timeout
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./core/ -run 'Timeout' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/client.go core/timeout_test.go
git commit -m "feat(core): add WithTimeout client option and precedence helper"
```

---

### Task 3: Apply timeout in GetResponse (unary)

**Files:**
- Modify: `core/client.go` `GetResponse` (lines ~456-534, specifically the timeout block ~461-468 and the retry loop result ~489-510)
- Test: `core/timeout_test.go` (append)

**Interfaces:**
- Consumes: `effectiveTimeout` (Task 2), `ErrTimeout` / `newTimeoutError` (Task 1).
- Produces: `GetResponse` returns `newTimeoutError(d)` when the Iris-applied deadline elapsed.

- [ ] **Step 1: Write the failing test**

Append to `core/timeout_test.go`. Add a small blocking provider to the file:

```go
// blockingProvider blocks Chat until ctx is cancelled, then returns ctx.Err().
type blockingProvider struct{}

func (blockingProvider) ID() string { return "blocking" }
func (blockingProvider) Chat(ctx context.Context, _ *ChatRequest) (*ChatResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (blockingProvider) StreamChat(ctx context.Context, _ *ChatRequest) (*ChatStream, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestGetResponseTimesOutWithErrTimeout(t *testing.T) {
	c := NewClient(blockingProvider{}, WithTimeout(50*time.Millisecond),
		WithRetryPolicy(NewNoRetryPolicy()))

	_, err := c.Chat("m").User("hi").GetResponse(context.Background())

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap DeadlineExceeded", err)
	}
}

func TestGetResponseCallerDeadlineNotRemapped(t *testing.T) {
	c := NewClient(blockingProvider{}, WithRetryPolicy(NewNoRetryPolicy()))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Chat("m").User("hi").GetResponse(ctx)

	if errors.Is(err, ErrTimeout) {
		t.Errorf("caller deadline was remapped to ErrTimeout; want raw context error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestGetResponseDisabledTimeoutDoesNotFire(t *testing.T) {
	c := NewClient(&mockProvider{}, WithTimeout(0))
	// mockProvider returns promptly; assert no ErrTimeout on the happy path.
	_, err := c.Chat("m").User("hi").GetResponse(context.Background())
	if errors.Is(err, ErrTimeout) {
		t.Errorf("unexpected ErrTimeout with timeout disabled: %v", err)
	}
}
```

Add `"errors"` to the test file imports. Verify the no-retry policy constructor name: run `rg -n "func NewNoRetryPolicy|NoRetry" core/retry.go`; if it differs (e.g. returns via `WithRetryPolicy(RetryPolicy)`), use the actual constructor. If none exists, replace `WithRetryPolicy(NewNoRetryPolicy())` with a policy of 0 retries using the real API found in `core/retry.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run 'GetResponse' -v`
Expected: FAIL — timeout never applied (blocks) or returns raw context error instead of `ErrTimeout`.

- [ ] **Step 3: Write minimal implementation**

Replace the existing timeout block in `GetResponse` (currently lines ~461-468):

```go
	// Apply timeout if set and context has no deadline
	if b.timeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, b.timeout)
			defer cancel()
		}
	}
```

with:

```go
	// Apply the effective execution timeout (see effectiveTimeout for precedence).
	// timedOut records that any resulting DeadlineExceeded is Iris-owned and
	// should surface as ErrTimeout rather than a raw context error.
	var timedOut time.Duration
	if d := b.effectiveTimeout(ctx); d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
		timedOut = d
	}
```

Then, immediately after the retry loop ends (after the `retryLoop:` block, before the telemetry-end section), remap the error:

```go
	// Map an Iris-applied deadline to a legible typed error.
	if timedOut > 0 && err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = newTimeoutError(timedOut)
	}
```

Add `"errors"` to `core/client.go` imports if not already present (check the import block).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./core/ -run 'GetResponse' -v`
Expected: PASS.

- [ ] **Step 5: Run the full core suite (no regressions)**

Run: `go test ./core/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add core/client.go core/timeout_test.go
git commit -m "feat(core): apply execution timeout to unary GetResponse"
```

---

### Task 4: Apply timeout in Stream without premature cancellation

**Files:**
- Modify: `core/client.go` `Stream` (~546-586) and `wrapStreamWithTelemetry` (~701-776)
- Test: `core/timeout_test.go` (append)

**Interfaces:**
- Consumes: `effectiveTimeout` (Task 2), `newTimeoutError` (Task 1).
- Produces: `wrapStreamWithTelemetry` gains two trailing params `onDone func()` and `mapErr func(error) error` (both nil-safe). `Stream` applies the effective timeout with `cancel` invoked only when the stream completes or the deadline fires.

- [ ] **Step 1: Write the failing test**

Append to `core/timeout_test.go`. Add a streaming mock whose behavior is controllable:

```go
// fastStreamProvider emits one chunk then closes immediately.
type fastStreamProvider struct{}

func (fastStreamProvider) ID() string { return "faststream" }
func (fastStreamProvider) Chat(context.Context, *ChatRequest) (*ChatResponse, error) {
	return nil, ErrNotSupported
}
func (fastStreamProvider) StreamChat(_ context.Context, _ *ChatRequest) (*ChatStream, error) {
	ch := make(chan ChatChunk, 1)
	errc := make(chan error)
	final := make(chan *ChatResponse, 1)
	ch <- ChatChunk{Delta: "hello"}
	close(ch)
	final <- &ChatResponse{Output: "hello"}
	close(final)
	close(errc)
	return &ChatStream{Ch: ch, Err: errc, Final: final}, nil
}

func TestStreamFastCompletesNotCancelled(t *testing.T) {
	c := NewClient(fastStreamProvider{}, WithTimeout(50*time.Millisecond))
	stream, err := c.Chat("m").User("hi").Stream(context.Background())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	resp, err := DrainStream(context.Background(), stream)
	if err != nil {
		t.Fatalf("DrainStream error = %v", err)
	}
	if resp.Output != "hello" {
		t.Errorf("Output = %q, want %q", resp.Output, "hello")
	}
}

func TestStreamStalledTimesOut(t *testing.T) {
	// blockingProvider.StreamChat blocks until ctx cancels, then returns ctx.Err()
	// as a setup error (StreamChat returns error, not a stream).
	c := NewClient(blockingProvider{}, WithTimeout(50*time.Millisecond))
	_, err := c.Chat("m").User("hi").Stream(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Stream() err = %v, want ErrTimeout", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run 'Stream' -v`
Expected: FAIL — `TestStreamStalledTimesOut` blocks/returns raw context error; signature mismatch once Step 3 starts.

- [ ] **Step 3: Write minimal implementation**

Update `wrapStreamWithTelemetry` signature and body. Change the signature to add the two trailing params:

```go
func wrapStreamWithTelemetry(
	ctx context.Context,
	stream *ChatStream,
	hook TelemetryHook,
	provider string,
	model ModelID,
	start time.Time,
	onDone func(),
	mapErr func(error) error,
) *ChatStream {
```

At the very top of the inner goroutine, add a deferred completion callback:

```go
	go func() {
		defer close(finalCh)
		defer close(errCh)
		if onDone != nil {
			defer onDone()
		}
```

Where the goroutine forwards an error (`errCh <- err`, two sites at ~734 and ~743), map it first. Replace each `errCh <- err` with a mapped local:

```go
			if ok {
				finalErr = err
				out := err
				if mapErr != nil {
					out = mapErr(err)
				}
				errCh <- out
			}
```

(Apply to both `case err, ok := <-stream.Err:` branches.)

Now update `Stream` (~546-586). After `validate()` and before building `startEvent`, resolve the timeout and derive the context:

```go
	var (
		cancel  context.CancelFunc
		timeout time.Duration
	)
	if d := b.effectiveTimeout(ctx); d > 0 {
		ctx, cancel = context.WithTimeout(ctx, d)
		timeout = d
	}
```

If `StreamChat` returns a setup error, cancel now and map it:

```go
	stream, err := b.client.provider.StreamChat(ctx, &b.req)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		if timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			err = newTimeoutError(timeout)
		}
		// ... existing telemetry-end emission, then:
		return nil, err
	}
```

For the success path, pass `onDone`/`mapErr` into the wrapper so the timeout ctx is released when the stream ends and mid-stream deadline errors surface as `ErrTimeout`:

```go
	onDone := func() {
		if cancel != nil {
			cancel()
		}
	}
	mapErr := func(e error) error {
		if timeout > 0 && errors.Is(e, context.DeadlineExceeded) {
			return newTimeoutError(timeout)
		}
		return e
	}
	return wrapStreamWithTelemetry(ctx, stream, b.client.telemetry, providerID, b.req.Model, start, onDone, mapErr), nil
```

- [ ] **Step 4: Update any other callers of wrapStreamWithTelemetry**

Run: `rg -n "wrapStreamWithTelemetry" core/`
For any call other than the one in `Stream`, pass `nil, nil` for the two new params. Expected: only the `Stream` caller exists.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./core/ -run 'Stream' -v`
Expected: PASS.
Run: `go test ./core/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add core/client.go core/timeout_test.go
git commit -m "feat(core): apply execution timeout to streaming calls"
```

---

### Task 5: Deprecate inert per-provider Config.Timeout

**Files:**
- Modify (doc comments only): `providers/openai/options.go`, `providers/anthropic/options.go`, `providers/gemini/options.go`, `providers/xai/options.go`, `providers/perplexity/options.go`, `providers/ollama/options.go`, `providers/huggingface/options.go`, `providers/voyageai/options.go`, `providers/zai/options.go`
- Leave unchanged: `providers/azurefoundry/options.go` (its `Timeout` is actually wired).

**Interfaces:**
- Produces: no behavior change. Godoc `Deprecated:` markers on the inert `WithTimeout` option and `Config.Timeout` field in each listed provider.

- [ ] **Step 1: Confirm which are inert**

Run: `rg -n "config.Timeout|cfg.Timeout|p.config.Timeout" providers/ | rg -v azurefoundry`
Expected: no read sites (only assignments in `options.go`), confirming they are dead everywhere except azurefoundry.

- [ ] **Step 2: Add deprecation doc comments**

For each listed provider's `WithTimeout` function, prepend a Godoc deprecation note. Example for `providers/openai/options.go`:

```go
// WithTimeout sets a per-request timeout.
//
// Deprecated: this option is currently inert for this provider and has no
// effect on requests. Use core.WithTimeout on the Client (or a context
// deadline) to bound execution. Retained for API compatibility.
func WithTimeout(d time.Duration) Option {
```

And on the `Timeout` field in each `Config` struct:

```go
	// Timeout is retained for compatibility but is not applied by this provider.
	//
	// Deprecated: use core.WithTimeout on the Client instead.
	Timeout time.Duration
```

Match each file's existing option type name (`Option` vs `ConfigOption`) and receiver exactly — read the file first.

- [ ] **Step 3: Verify build and vet**

Run: `go build ./... && go vet ./providers/...`
Expected: success, no new warnings.

- [ ] **Step 4: Commit**

```bash
git add providers/*/options.go
git commit -m "docs(providers): deprecate inert per-provider WithTimeout"
```

---

### Task 6: Verify gpt-5.4-mini routes to the Responses API

**Files:**
- Test: `providers/openai/client_responses_test.go` (append)

**Interfaces:**
- Consumes: existing `New`, `WithBaseURL`, `ModelGPT54Mini`, `responsesResponse` (all in package `openai`).

- [ ] **Step 1: Write the test**

Append to `providers/openai/client_responses_test.go`:

```go
func TestGPT54MiniRoutesToResponsesAPI(t *testing.T) {
	var gotPath, gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if m, ok := body["model"].(string); ok {
			gotModel = m
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responsesResponse{
			ID:         "resp-54mini",
			Model:      "gpt-5.4-mini",
			Status:     "completed",
			OutputText: "pong",
			Output:     []responsesOutput{{Type: "message", Role: "assistant"}},
			Usage:      &responsesUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
		})
	}))
	defer server.Close()

	p := New("test-key", WithBaseURL(server.URL))
	resp, err := p.Chat(context.Background(), &core.ChatRequest{
		Model:    ModelGPT54Mini,
		Messages: []core.Message{{Role: core.RoleUser, Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if gotPath != "/responses" {
		t.Errorf("request path = %q, want /responses", gotPath)
	}
	if gotModel != "gpt-5.4-mini" {
		t.Errorf("request model = %q, want gpt-5.4-mini", gotModel)
	}
	if resp.Output != "pong" {
		t.Errorf("Output = %q, want pong", resp.Output)
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./providers/openai/ -run TestGPT54MiniRoutesToResponsesAPI -v`
Expected: PASS — confirms the model is accepted, routed to `/responses`, sent with the correct model string, and its response parsed. (Note: this is a mock-server verification; a live OpenAI round-trip requires Azure Key Vault credentials unavailable in this environment.)

- [ ] **Step 3: Commit**

```bash
git add providers/openai/client_responses_test.go
git commit -m "test(openai): verify gpt-5.4-mini routes to Responses API"
```

---

### Task 7: Full verification + change doc

**Files:**
- Create: `docs/changes/2026-07-31_v{version}_core-execution-timeout.md` (resolve `{version}` from `go.mod`/`VERSION`; use `v0.0.0-dev` if none declared)

**Interfaces:** none.

- [ ] **Step 1: Run the whole suite with the race detector**

Run: `go test -race ./core/ ./providers/openai/`
Expected: PASS, no data races (validates the streaming goroutine + cancel handling).

- [ ] **Step 2: Build everything**

Run: `go build ./...`
Expected: success.

- [ ] **Step 3: Write the change doc**

Follow the repo `CLAUDE.md` change-doc structure (front matter + Summary, Motivation, What Changed, Technical Specification, Usage Examples, Integration Notes, Breaking Changes & Migration, Deferred / Out of Scope, Testing Notes). `product: iris`, `change_type: feature`. `affected_components` must list: `core/client.go`, `core/errors.go`, `core/timeout_test.go`, `core/errors_test.go`, and every `providers/*/options.go` touched, plus `providers/openai/client_responses_test.go`. Document `WithTimeout`, `DefaultTimeout`, `ErrTimeout`, precedence, streaming semantics, the 120s default behavior change with opt-out (`WithTimeout(0)`), and the embeddings deferral.

- [ ] **Step 4: Commit**

```bash
git add docs/changes/2026-07-31_v*_core-execution-timeout.md
git commit -m "docs(core): add change doc for execution timeout"
```

---

## Self-Review

**Spec coverage:**
- Client option + default → Task 2. Precedence → Task 2 (`effectiveTimeout`) + Tasks 3/4. Unary timeout → Task 3. Streaming w/o premature cancel → Task 4. Typed error → Task 1 (used in 3/4). Deprecate per-provider field → Task 5. gpt-5.4-mini verification → Task 6. Tests → Tasks 1-4, 6. Change doc → Task 7. Embeddings deferral documented → Task 7 doc. All spec sections covered.

**Placeholder scan:** No TBD/TODO. The one runtime lookup (`NewNoRetryPolicy` name and provider `Option` type names) is called out explicitly with a `rg` command and a fallback instruction, not left vague.

**Type consistency:** `effectiveTimeout(ctx) time.Duration`, `WithTimeout(d) ClientOption`, `ErrTimeout`, `newTimeoutError(d) error`, and the extended `wrapStreamWithTelemetry(..., onDone func(), mapErr func(error) error)` are used consistently across tasks. `Client.timeout` field name consistent. `DefaultTimeout = 120s` consistent everywhere.
