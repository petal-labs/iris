# Design: SDK Reliability Hardening (Errors, Structured Output, Timeouts)

- **Date:** 2026-08-01
- **Status:** Approved (pending spec review)
- **Branch:** `feat/sdk-reliability-hardening`

## Problem

A downstream app built on Iris hit three classes of failure, confirmed by a four-part source audit:

1. **Errors not descriptive / not propagated.** Provider error bodies are replaced with generic `http.StatusText`; decode errors omit the body; transport errors drop the wrapped `context.DeadlineExceeded`; and the standard streaming drain (`DrainStream`) can swallow a real error and return false success.
2. **Non-strict document shapes from models.** Structured output is wired only on the OpenAI Chat Completions path. The Responses API (all GPT-5.x models, incl. `gpt-5.4-mini`) silently drops it; `Strict` defaults to false; several providers ignore the schema entirely; and there is no capability gating.
3. **Provider keys "didn't work."** The API key is never trimmed (a trailing newline from Azure Key Vault corrupts the `Authorization` header); empty keys are not validated at construction; `Secret.IsEmpty()` misses whitespace.

A cross-cutting root cause underlies much of #1: `core.ProviderError` has no field for the raw body, and the normalization layer discards both the provider's real error text and the original error object.

## Goals

- Errors carry the provider's actual detail and preserve `errors.Is` chains end-to-end (including `context.DeadlineExceeded` → `ErrTimeout`).
- Structured output is enforced on every path that claims to support it, strict by default, and fails loud when unsupported.
- API keys are sanitized and validated so a mis-provisioned key fails with a clear, early error.
- No streaming error is ever silently swallowed.

## Non-Goals / Deferred

- Full timeout coverage for embeddings/batch/files/vector-stores/images via new `core.Client` wrappers. This pass fixes the one concrete hang (batch poll) and **documents** the remaining gap; broad coverage is a follow-up.
- Client-side re-validation of the model's returned JSON against the schema (a separate feature). We enforce the request-side contract only.
- Adding `WithAPIKey` options across all providers (API-symmetry nicety, not a correctness bug).
- PetalFlow changes — separate repo; only the Iris contract it consumes is in scope.

## Decisions (confirmed with maintainer)

- **`Strict` defaults to true** for schema requests. The builder forces strict on; a validator checks the schema can satisfy OpenAI strict mode and returns a clear error if not (rather than letting the API reject it). An explicit non-strict opt-out is provided.
- **Unsupported structured output is a hard error** before the request is sent (behavior change from silent-ignore).

---

## Workstream A — Error handling & key hygiene

### A1. Preserve error chains in normalization

`providers/internal/normalize/errors.go`:

```go
func NetworkError(provider string, err error) error {
    return &core.ProviderError{
        Provider: provider,
        Message:  err.Error(),
        Err:      fmt.Errorf("%w: %w", core.ErrNetwork, err), // was: core.ErrNetwork
    }
}
```

Same treatment for `DecodeError` (`Err: fmt.Errorf("%w: %w", core.ErrDecode, err)`). `ProviderError.Unwrap()` returns `e.Err`, so both `errors.Is(_, core.ErrNetwork)` and `errors.Is(_, context.DeadlineExceeded)` then hold.

Effects (all verified in the audit):
- Streaming timeouts surface as `core.ErrTimeout` again (the remap at `core/client.go:629,659` fires).
- `GetResponse` returns `ErrTimeout` even when retries are exhausted/disabled.
- `isRetryable` (`core/retry.go`) stops misclassifying a timed-out request as retryable — its `context.DeadlineExceeded` check (which runs before the `ErrNetwork` check) now matches. **A test must assert this ordering holds.**

Also apply the same `%w` fix to the hand-rolled equivalents in `providers/ollama/chat.go` and `providers/ollama/embeddings.go`, and confirm `providers/azurefoundry` uses `normalize.NetworkError` (or fix its hand-rolled version).

### A2. Carry the raw body on `ProviderError`

`core/errors.go`: add a field.

```go
type ProviderError struct {
    Provider  string
    Status    int
    RequestID string
    Code      string
    Message   string
    Body      string // raw response body (truncated), preserved when the structured message is empty
    Err       error
}
```

- `normalize.OpenAIStyleProviderError` and the hand-rolled per-provider variants: when the parsed `error.message` is empty, set `Message` from a truncated body snippet (first ~512 bytes) instead of `http.StatusText`, and store the truncated body (≤ ~4 KB) in `Body`. Never silently discard the body.
- `normalize.DecodeError` gains a `body []byte` parameter; store the truncated body and include a snippet in `Message`. Update all `newDecodeError` call sites (mechanical; enumerate with `rg "newDecodeError|DecodeError\("`).
- `ProviderError.Error()`: include the request ID as today; append `Body` snippet only when `Message` is body-derived (avoid double-printing). Omit the `(status=0, code=)` suffix when both are zero-valued (fixes noisy network/decode output).

### A3. Key sanitization & validation

- `core/secret.go`: `NewSecret` trims surrounding whitespace: `Secret{value: strings.TrimSpace(value)}`. This is the primary fix for the Key-Vault trailing-newline symptom and applies to every provider at once.
- `Secret.IsEmpty()`: `return strings.TrimSpace(s.value) == ""` (defensive; azurefoundry's only auth guard).
- Empty-key preflight error: add a descriptive, typed error when the key is empty at request time. Scope this pass to the **OpenAI** provider (the app's primary): at the top of `Chat`/`StreamChat` (and `CreateEmbeddings`), if `p.config.APIKey.IsEmpty()`, return a `*core.ProviderError{Provider:"openai", Message:"API key is empty; pass it to openai.New(key) or configure your secret source", Err: core.ErrUnauthorized}` before any HTTP call. Other providers: tracked as a follow-up (documented in Deferred), since the global `NewSecret` trim already covers the common corruption case for all of them.

### A4. `DrainStream` must not swallow errors (Workstream C item, grouped here for the streaming-error theme — implemented in C1)

---

## Workstream B — Structured output

### B1. Wire structured output into the Responses API

`providers/openai/types_responses.go`: add a text/format field to `responsesRequest`.

```go
type responsesRequest struct {
    // ...existing...
    Text *responsesText `json:"text,omitempty"`
}

type responsesText struct {
    Format *responsesTextFormat `json:"format,omitempty"`
}

type responsesTextFormat struct {
    Type   string          `json:"type"`             // "json_schema" | "json_object"
    Name   string          `json:"name,omitempty"`   // required for json_schema
    Schema json.RawMessage `json:"schema,omitempty"`
    Strict *bool           `json:"strict,omitempty"`
}
```

`providers/openai/mapping_responses.go` `buildResponsesRequest`: map `req.ResponseFormat`/`req.JSONSchema` into `Text.Format`, mirroring `mapResponseFormat` for Chat Completions.

> **Implementer must verify the exact Responses API wire shape** against current OpenAI docs (use the context7 MCP: resolve OpenAI docs, query "Responses API structured outputs text.format json_schema strict"). The shape above is the expected form (`text.format.{type,name,schema,strict}`); confirm field names/nesting before finalizing, and add an httptest that asserts the emitted JSON matches.

### B2. Strict-by-default + schema validation

`core/client.go`:
- `ResponseJSONSchema(schema)` forces strict on: if `schema.Strict` is false, set it true. Add `ResponseJSONSchemaNonStrict(schema *JSONSchemaDefinition)` as the explicit opt-out (sets `Strict=false`). Document that `ResponseJSONSchema` upgrades to strict (behavior change).
- Add a strict-schema validator invoked in `validate()` (or `GetResponse`/`Stream`) when `ResponseFormat == ResponseFormatJSONSchema && JSONSchema.Strict`:
  - Walk the JSON Schema; for every `type:"object"`, require `additionalProperties:false` and that `required` lists all `properties` keys.
  - On violation, return a new sentinel `core.ErrInvalidSchema` with a message naming the offending path (e.g. `strict schema requires additionalProperties:false at .address`). This surfaces the problem before the provider rejects it.
- `core/types.go`: `JSONSchemaDefinition.Strict` tag becomes `json:"strict"` (drop `omitempty`) so an explicit `false` is transmittable where a provider distinguishes it; the Responses `strict` field is `*bool` to encode true/false explicitly.

### B3. Capability gating (hard error when unsupported)

`core/client.go` `validate()` (or a shared preflight used by both `GetResponse` and `Stream`):
- When `req.ResponseFormat` is `ResponseFormatJSON` or `ResponseFormatJSONSchema`, verify support:
  - Provider-level: if the `Provider` interface exposes `Supports(Feature) bool`, require `Supports(core.FeatureStructuredOutput)`.
  - Model-level: if the provider exposes model info (e.g. openai `GetModelInfo`), require `HasCapability(core.FeatureStructuredOutput)`.
- On unsupported, return new sentinel `core.ErrStructuredOutputUnsupported` with a message naming provider + model.

> **Implementer must verify** whether `Supports` is on the `core.Provider` interface or only concrete providers. If not on the interface, gate on what is reliably available (model registry via a small capability hook) and document the mechanism. Ensure the OpenAI models that use structured output actually declare `FeatureStructuredOutput` in `providers/openai/models.go` (add where missing for models that support it, e.g. the GPT-5.x entries).

### B4. Wire or explicitly reject on other providers

For anthropic, ollama, perplexity, zai, gemini: rather than silently ignoring the schema, the B3 capability gate makes an unsupported request a hard error. **Minimum bar for this pass:** ensure each provider either (a) correctly declares `FeatureStructuredOutput` AND wires the schema, or (b) does *not* declare it, so B3 rejects the request. No provider may declare support while ignoring the schema. Actually wiring native structured output for anthropic/ollama/gemini is a follow-up (documented in Deferred); this pass guarantees no silent non-conformance.

---

## Workstream C — Streaming & timeout robustness

### C1. Fix the `DrainStream` error-swallowing race

`core/streaming.go`: rewrite `DrainStream` to use the same non-lossy discipline as `wrapStreamWithTelemetry` (`core/client.go:800-868`):
- Do not exit the read loop the instant `Ch` closes; continue until both `Err` and `Final` are resolved.
- Treat `Final` closing without a value as "check `Err`".
- On `ctx.Done()`, do a final non-blocking drain of `Err`/`Final` before returning `ctx.Err()`, so a concrete provider error wins over a generic context error.
- Test: a provider that emits chunks, then delivers an error via the supervisor path *after* `Ch` closes → assert `DrainStream` returns that error (not false success). Run under `-race`.

### C2. Fix the batch-poll infinite hang

`core/batch.go` `BatchWaiter.Wait`: the `maxWait` budget is checked only after each poll returns, so one hung `GetBatchStatus` blocks forever. Fix by bounding each poll with the remaining budget:
- Derive an overall deadline from `maxWait`; for each poll, call with a context carrying the remaining time (`context.WithTimeout(ctx, remaining)`), and stop when the deadline passes.
- Test: a stub `BatchProvider` whose `GetBatchStatus` blocks; assert `Wait` returns within ~`maxWait` with a deadline-related error rather than hanging.

### C3. Document the remaining timeout gap

Add to the change doc (and a short note in the relevant Go doc comments): embeddings, batch, files, vector-store, and image calls do **not** go through `core.Client` and thus are not covered by `core.WithTimeout`; callers must pass a context with a deadline. List the affected methods.

---

## Testing

- **A1:** table test — a `context.DeadlineExceeded` wrapped by `NetworkError` satisfies `errors.Is` for both `ErrNetwork` and `context.DeadlineExceeded`; a streaming timeout against a mock provider that wraps via `NetworkError` now surfaces as `ErrTimeout`; `isRetryable` returns false for that error.
- **A2:** given a non-JSON / wrong-shape error body, `ProviderError.Message`/`Body` contain the real body text; decode failure includes the body snippet.
- **A3:** `NewSecret(" sk-x\n")` exposes `"sk-x"`; `IsEmpty` true for `" "`; OpenAI `Chat` with empty key returns the descriptive `ErrUnauthorized` before any HTTP call (httptest server asserts it is never hit).
- **B1:** httptest asserts the Responses API request carries `text.format` with `type:"json_schema"`, the schema, and `strict:true`.
- **B2:** `ResponseJSONSchema` sets strict true; a schema lacking `additionalProperties:false` yields `ErrInvalidSchema` naming the path; `ResponseJSONSchemaNonStrict` sets false.
- **B3:** requesting structured output against a provider/model without `FeatureStructuredOutput` returns `ErrStructuredOutputUnsupported` before any call.
- **C1/C2:** as described above, both under `-race`.

## Breaking Changes & Migration

- `ResponseJSONSchema` now defaults to strict; a previously-passing loose schema may now return `ErrInvalidSchema`. Migration: make the schema strict-compatible (`additionalProperties:false`, all fields required) or use `ResponseJSONSchemaNonStrict`.
- Requesting structured output from an unsupported provider/model now returns `ErrStructuredOutputUnsupported` instead of silently returning unconstrained output.
- `normalize.DecodeError` gains a `body` parameter (internal package; not part of the public SDK surface).
- New exported symbols: `ProviderError.Body`, `core.ErrInvalidSchema`, `core.ErrStructuredOutputUnsupported`, `ChatBuilder.ResponseJSONSchemaNonStrict`.

## Rollout

Semantically a minor release (0.x) with two documented behavior changes; ship as `v0.16.0`. Each workstream is independently testable; land A first (highest impact, lowest risk), then B, then C.
