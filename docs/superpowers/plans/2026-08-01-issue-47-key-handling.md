# Issue #47 Key Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Resolve issue #47 — (13) empty-key preflight for non-OpenAI providers, (14) remove the dead `api_key_ref` config field, (15) add `WithAPIKey` consistently across providers.

**Architecture:** A shared `normalize.RequireAPIKey(provider, key)` helper produces a descriptive `core.ProviderError{Err: core.ErrUnauthorized}`. Providers that require a key call it at the top of their HTTP entry points before any request. `WithAPIKey` is a trivial functional option mirroring Ollama's existing one.

**Tech Stack:** Go 1.24, stdlib + `net/http/httptest` for tests.

## Global Constraints

- Module `github.com/petal-labs/iris` (go 1.24). Branch: `fix/47-key-handling`.
- New exported symbol: `normalize.RequireAPIKey(provider string, key core.Secret) error`; a `WithAPIKey(key string) Option` per provider (8 new).
- Behavior change: providers that require a key now fail fast with `core.ErrUnauthorized` before any HTTP call when the key is empty/whitespace.
- **Exclusions (do NOT add a hard preflight):** Ollama (key optional for local), huggingface discovery methods (`GetModelStatus`/`GetModelProviders`/`ListModels` — anonymous access by design), voyageai `Chat`/`StreamChat` (already return `core.ErrNotSupported` with no HTTP).
- `WithAPIKey` mirrors `providers/ollama/options.go`: `func WithAPIKey(key string) Option { return func(c *Config) { c.APIKey = core.NewSecret(key) } }`. Every provider's `New` sets `APIKey` before the opts loop, so last-wins works.
- Conventional commits, subject ≤ 72 chars, no trailing period/backticks/emoji.
- Every task ends green: `go test ./...`, `go build ./...`, `gofmt -l` clean.

---

### Task 1: Shared RequireAPIKey helper + refactor OpenAI

**Files:**
- Modify: `providers/internal/normalize/errors.go`
- Modify: `providers/openai/provider.go` (refactor `requireAPIKey` to delegate)
- Test: `providers/internal/normalize/errors_test.go`

**Interfaces:**
- Produces: `func RequireAPIKey(provider string, key core.Secret) error` — returns `*core.ProviderError{Provider, Message, Err: core.ErrUnauthorized}` when `key.IsEmpty()`, else nil.

- [ ] **Step 1: Failing test** in `providers/internal/normalize/errors_test.go`:

```go
func TestRequireAPIKey(t *testing.T) {
	if err := RequireAPIKey("acme", core.NewSecret("sk-x")); err != nil {
		t.Fatalf("non-empty key should pass: %v", err)
	}
	err := RequireAPIKey("acme", core.NewSecret("  "))
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("empty key err = %v, want ErrUnauthorized", err)
	}
	var pe *core.ProviderError
	if !errors.As(err, &pe) || pe.Provider != "acme" {
		t.Errorf("want *core.ProviderError with Provider=acme, got %#v", err)
	}
}
```

- [ ] **Step 2: Run — expect FAIL.**
- [ ] **Step 3: Implement** in `normalize/errors.go`:

```go
// RequireAPIKey returns a descriptive *core.ProviderError wrapping
// core.ErrUnauthorized when the API key is empty (or whitespace-only), so a
// provider can fail fast before making an unauthenticated request. Returns nil
// when the key is present.
func RequireAPIKey(provider string, key core.Secret) error {
	if key.IsEmpty() {
		return &core.ProviderError{
			Provider: provider,
			Message:  "API key is empty; provide a non-empty key to " + provider + ".New or configure your secret source",
			Err:      core.ErrUnauthorized,
		}
	}
	return nil
}
```

Refactor `providers/openai/provider.go` `requireAPIKey()` to delegate: `return normalize.RequireAPIKey("openai", p.config.APIKey)`. Confirm `normalize` is already imported in that file (it is, for error helpers). Keep the three call sites (`Chat`, `StreamChat`, `CreateEmbeddings`) unchanged.

- [ ] **Step 4: Run** `go test ./providers/internal/normalize/ ./providers/openai/ -run 'RequireAPIKey|EmptyKey|Auth' -v` — expect PASS (OpenAI's existing auth tests must still pass with the delegated message; they assert `errors.Is(_, ErrUnauthorized)` + no HTTP, not the exact string — verify by reading `providers/openai/auth_test.go`).
- [ ] **Step 5: Commit** `git commit -m "feat(providers): add shared RequireAPIKey preflight helper"`

---

### Task 2: Empty-key preflight for chat providers

**Files:**
- Modify: `providers/{anthropic,gemini,xai,perplexity,huggingface,zai}/provider.go` (guard `Chat` + `StreamChat`)
- Test: one new `auth_test.go` per provider (or a shared representative) — at minimum anthropic + gemini + one Bearer provider

**Interfaces:**
- Consumes: `normalize.RequireAPIKey` (Task 1).

- [ ] **Step 1: Failing test** — add `providers/anthropic/auth_test.go` (mirror `providers/openai/auth_test.go`):

```go
package anthropic

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestChatEmptyKeyFailsBeforeHTTP(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	p := New("", WithBaseURL(srv.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}
```

Use a real model constant for each provider (grep its `models.go` for a valid `Model*` const or use a plausible id string). Add an equivalent test for gemini and one Bearer provider (xai). For the remaining providers, at minimum ensure `go test ./...` covers compilation; a per-provider auth test is preferred but the shared pattern is proven by these three.

- [ ] **Step 2: Run — expect FAIL** (request currently goes through).
- [ ] **Step 3: Implement** — at the top of each provider's `Chat` and `StreamChat` methods, add:

```go
	if err := normalize.RequireAPIKey("<provider>", p.config.APIKey); err != nil {
		return nil, err
	}
```

Use the correct provider-name string per package (`"anthropic"`, `"gemini"`, `"xai"`, `"perplexity"`, `"huggingface"`, `"zai"`). Read each `provider.go` first to place the guard correctly (before `doChat`/`doStreamChat`). Confirm `normalize` is imported in each (add the import if missing). **Do NOT touch huggingface `discovery.go` methods.**

- [ ] **Step 4: Run** `go test ./providers/... -run 'EmptyKey|Auth' -v` then `go test ./...` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(providers): fail fast on empty API key for chat providers"`

---

### Task 3: Empty-key preflight for voyageai

**Files:**
- Modify: `providers/voyageai/client_embeddings.go`, `client_contextualized.go`, `client_rerank.go`
- Test: `providers/voyageai/auth_test.go`

**Interfaces:** Consumes `normalize.RequireAPIKey`.

- [ ] **Step 1: Failing test** — `providers/voyageai/auth_test.go` asserting `CreateEmbeddings` with an empty key returns `core.ErrUnauthorized` before HTTP (mirror the Task 2 test, using `p.CreateEmbeddings(ctx, &core.EmbeddingRequest{...})`; read `core.EmbeddingRequest` shape and a valid voyage model id from `providers/voyageai/models.go`). Add cases for `Rerank` and `CreateContextualizedEmbeddings` too.
- [ ] **Step 2: Run — expect FAIL.**
- [ ] **Step 3: Implement** — add the `normalize.RequireAPIKey("voyageai", p.config.APIKey)` guard at the top of `CreateEmbeddings`, `CreateContextualizedEmbeddings`, and `Rerank` (return `nil, err`). Read each method for the correct return signature.
- [ ] **Step 4: Run** `go test ./providers/voyageai/ -v` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(voyageai): fail fast on empty API key for embeddings"`

---

### Task 4: Align azurefoundry ErrNoAuth with ErrUnauthorized

**Files:**
- Modify: `providers/azurefoundry/provider.go` (the `ErrNoAuth` return in `buildHeaders`)
- Test: `providers/azurefoundry/errors_test.go` or a new auth test

**Interfaces:** none new.

- [ ] **Step 1: Failing test** — assert that the no-auth error (from a provider built with an empty key and no token credential, triggering `buildHeaders`) satisfies BOTH `errors.Is(err, ErrNoAuth)` AND `errors.Is(err, core.ErrUnauthorized)`. Read `providers/azurefoundry/errors_test.go` for the existing `ErrNoAuth` test pattern and how to trigger `buildHeaders` (e.g. via `Chat` with empty key + no credential against an httptest server).
- [ ] **Step 2: Run — expect FAIL** (currently `ErrNoAuth` is a plain sentinel, so `errors.Is(_, core.ErrUnauthorized)` is false).
- [ ] **Step 3: Implement** — keep `ErrNoAuth` as-is, but where `buildHeaders` returns it, wrap so both hold:

```go
	} else {
		return nil, fmt.Errorf("%w: %w", core.ErrUnauthorized, ErrNoAuth)
	}
```

Confirm `fmt` and `core` are imported. Do NOT change azurefoundry's dual-auth logic (token-credential path stays first). Any existing test asserting `errors.Is(err, ErrNoAuth)` must still pass (multi-`%w` preserves it).

- [ ] **Step 4: Run** `go test ./providers/azurefoundry/ -v` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "fix(azurefoundry): make no-auth error satisfy ErrUnauthorized"`

---

### Task 5: WithAPIKey option for all providers

**Files:**
- Modify: `providers/{anthropic,gemini,xai,perplexity,huggingface,voyageai,zai,azurefoundry}/options.go`
- Test: `providers/anthropic/options_test.go` (+ representative others)

**Interfaces:**
- Produces: `WithAPIKey(key string) Option` in each of the 8 packages.

- [ ] **Step 1: Failing test** — add to `providers/anthropic/options_test.go`:

```go
func TestWithAPIKeyOverridesConstructorArg(t *testing.T) {
	p := New("ctor-key", WithAPIKey("opt-key"))
	if got := p.config.APIKey.Expose(); got != "opt-key" {
		t.Errorf("APIKey = %q, want opt-key (option should win over constructor)", got)
	}
}
```

(Confirm the field access path — `p.config.APIKey.Expose()` — matches the package; same-package white-box test.) Add an equivalent test for at least gemini and azurefoundry.

- [ ] **Step 2: Run — expect FAIL** (`WithAPIKey` undefined).
- [ ] **Step 3: Implement** — add to each provider's `options.go` (mirror `providers/ollama/options.go:45-51`):

```go
// WithAPIKey sets the API key, overriding any value passed to New.
func WithAPIKey(key string) Option {
	return func(c *Config) {
		c.APIKey = core.NewSecret(key)
	}
}
```

Confirm `core` is imported in each `options.go` (add if missing). For azurefoundry, place it near `WithTokenCredential`.

- [ ] **Step 4: Run** `go test ./providers/... -run 'WithAPIKey' -v` then `go build ./...` — expect PASS.
- [ ] **Step 5: Commit** `git commit -m "feat(providers): add WithAPIKey option across providers"`

---

### Task 6: Remove dead api_key_ref config field

**Files:**
- Modify: `cli/config/config.go` (remove `APIKeyRef` from `ProviderConfig`)
- Modify: `cli/config/config_test.go` (remove the 5 references)

**Interfaces:** Removes `ProviderConfig.APIKeyRef`.

- [ ] **Step 1: Confirm dead** — `rg -n "APIKeyRef" .` must show only `cli/config/config.go` (definition) and `cli/config/config_test.go`. If any production reference appears, STOP and report.
- [ ] **Step 2: Implement** — delete the `APIKeyRef string \`yaml:"api_key_ref"\`` line from `ProviderConfig` (leaving `BaseURL`). Remove the field from the test structs/assertions at `cli/config/config_test.go:106,107,175,185,186` (read the file to remove cleanly without breaking surrounding test logic).
- [ ] **Step 3: Run** `go test ./cli/config/ -v` then `go build ./...` — expect PASS.
- [ ] **Step 4: Commit** `git commit -m "refactor(cli): remove unused api_key_ref config field"`

---

### Task 7: Change doc + full verification

**Files:**
- Create: `docs/changes/2026-08-01_v0.0.0-dev_issue-47-key-handling.md` (version: use the next unreleased — `v0.17.0` if a release is planned, else `v0.0.0-dev`; check latest tag with `git tag --sort=-v:refname | head -1` and use the next minor)

- [ ] **Step 1:** `go test -race ./...` — expect PASS.
- [ ] **Step 2:** `go build ./...`; `gofmt -l .` (ignore pre-existing untracked files).
- [ ] **Step 3:** Write the change doc per repo `CLAUDE.md` structure. `product: iris`, `change_type: feature`. `affected_components` lists every file touched across Tasks 1-6. Document: the new `normalize.RequireAPIKey`, the per-provider empty-key preflight (and the deliberate exclusions: Ollama, huggingface discovery, voyageai chat), the azurefoundry `ErrNoAuth` alignment, the 8 new `WithAPIKey` options (last-wins), and the `api_key_ref` removal. Include a "Breaking Changes & Migration" note: providers now hard-error on an empty key before the request. Include a "Deferred" note: files/images entry points not yet guarded.
- [ ] **Step 4: Commit** `git commit -m "docs: change doc for issue 47 key handling"`

---

## Self-Review

**Spec coverage:** Item 13 → Tasks 1-4 (helper + chat providers + voyageai + azurefoundry). Item 14 → Task 6. Item 15 → Task 5. Change doc → Task 7. Exclusions (Ollama, hf discovery, voyageai chat) are encoded in the Global Constraints and Task scoping.

**Placeholder scan:** No TBD. Model-id/field-shape lookups (per-provider model constants, `core.EmbeddingRequest`) are called out with explicit "read/grep" instructions and fallbacks.

**Type consistency:** `RequireAPIKey(provider string, key core.Secret) error`, `WithAPIKey(key string) Option`, `core.ErrUnauthorized`, `core.ProviderError` used consistently. Every `New` sets `APIKey` before the opts loop (verified in investigation), so `WithAPIKey` last-wins holds across all 8 providers.
