# SDK Reliability Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix descriptive/propagated errors, enforce strict structured output (incl. the Responses API), and close two streaming/timeout robustness bugs — the concrete failures a downstream app hit.

**Architecture:** Preserve error chains and raw bodies in the shared normalization layer; wire and gate structured output in core + the OpenAI Responses path; harden `DrainStream` and `BatchWaiter`. No new public API beyond the symbols listed in Global Constraints.

**Tech Stack:** Go 1.24, standard library. Tests use `testing`, `net/http/httptest`, `-race`.

## Global Constraints

- Module `github.com/petal-labs/iris` (go 1.24). Base branch: `feat/sdk-reliability-hardening`.
- New exported symbols (only these): `core.ProviderError.Body`, `core.ErrInvalidSchema`, `core.ErrStructuredOutputUnsupported`, `(*core.ChatBuilder).ResponseJSONSchemaNonStrict`.
- Behavior changes (must be documented in the change doc): `ResponseJSONSchema` defaults to strict; structured output on an unsupported provider/model is a hard error.
- `Provider.Supports(feature core.Feature) bool` is on the `core.Provider` interface (`core/client.go:15`). Use it for capability gating.
- Conventional commits, subject ≤ 72 chars, no trailing period, no backticks/emoji.
- Windows-safe paths (`filepath.Join`) in any test that touches the filesystem (none expected).
- Every task ends green: `go test ./...` (and `-race` where noted), `go build ./...`, `gofmt -l` clean.

---

### Task 1: Preserve error chains in normalization (Workstream A1)

**Files:**
- Modify: `providers/internal/normalize/errors.go` (NetworkError ~40-46, DecodeError ~49-55)
- Modify: `providers/ollama/chat.go`, `providers/ollama/embeddings.go` (hand-rolled network errors)
- Test: `providers/internal/normalize/errors_test.go`; `core/timeout_test.go` (append)

**Interfaces:**
- Produces: `NetworkError`/`DecodeError` return a `*core.ProviderError` whose `Err` wraps the original error, so `errors.Is` reaches both the sentinel and the underlying (e.g. `context.DeadlineExceeded`).

- [ ] **Step 1: Failing test (chain preservation)**

In `providers/internal/normalize/errors_test.go`:

```go
func TestNetworkErrorPreservesChain(t *testing.T) {
	underlying := fmt.Errorf("dial tcp: %w", context.DeadlineExceeded)
	err := NetworkError("openai", underlying)
	if !errors.Is(err, core.ErrNetwork) {
		t.Error("want errors.Is(err, core.ErrNetwork)")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("want errors.Is(err, context.DeadlineExceeded)")
	}
}

func TestDecodeErrorPreservesChain(t *testing.T) {
	underlying := errors.New("invalid character '<'")
	err := DecodeError("openai", underlying)
	if !errors.Is(err, core.ErrDecode) {
		t.Error("want errors.Is(err, core.ErrDecode)")
	}
	if !errors.Is(err, underlying) {
		t.Error("want errors.Is(err, underlying)")
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`go test ./providers/internal/normalize/ -run PreservesChain -v`) — currently `Err` is the bare sentinel.

- [ ] **Step 3: Implement**

```go
func NetworkError(provider string, err error) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Err:      fmt.Errorf("%w: %w", core.ErrNetwork, err),
	}
}

func DecodeError(provider string, err error) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Err:      fmt.Errorf("%w: %w", core.ErrDecode, err),
	}
}
```

Add `"fmt"` to imports. Then update the ollama hand-rolled equivalents (grep `rg -n "Err:.*ErrNetwork" providers/ollama/`) to the same `fmt.Errorf("%w: %w", core.ErrNetwork, err)` shape. Confirm `providers/azurefoundry` routes through `normalize.NetworkError`; if it hand-rolls, fix it the same way.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Failing test (streaming timeout now surfaces as ErrTimeout + retry classification)**

Append to `core/timeout_test.go` a provider whose `StreamChat` blocks then returns a `NetworkError`-wrapped context error, mirroring real providers:

```go
// wrappingTimeoutProvider simulates a real provider that wraps a timed-out
// transport error the way normalize.NetworkError does.
type wrappingTimeoutProvider struct{}

func (wrappingTimeoutProvider) ID() string                 { return "wrapping" }
func (wrappingTimeoutProvider) Supports(Feature) bool      { return false }
func (wrappingTimeoutProvider) Chat(ctx context.Context, _ *ChatRequest) (*ChatResponse, error) {
	<-ctx.Done()
	// Emulate NetworkError chain: sentinel + underlying ctx error.
	return nil, &ProviderError{Provider: "wrapping", Message: ctx.Err().Error(),
		Err: fmt.Errorf("%w: %w", ErrNetwork, ctx.Err())}
}
func (wrappingTimeoutProvider) StreamChat(ctx context.Context, _ *ChatRequest) (*ChatStream, error) {
	<-ctx.Done()
	return nil, &ProviderError{Provider: "wrapping", Message: ctx.Err().Error(),
		Err: fmt.Errorf("%w: %w", ErrNetwork, ctx.Err())}
}

func TestGetResponseTimeoutSurfacesThroughWrappedNetworkError(t *testing.T) {
	c := NewClient(wrappingTimeoutProvider{}, WithTimeout(50*time.Millisecond),
		WithRetryPolicy(noRetryPolicy{}))
	_, err := c.Chat("m").User("hi").GetResponse(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout even when provider wraps via NetworkError", err)
	}
}
```

(`noRetryPolicy` already exists in `core/timeout_test.go` from prior work; reuse it. `ErrNetwork` is in package `core`, same package as the test.)

Also add a retry-classification assertion in `core/retry_test.go`:

```go
func TestWrappedTimeoutIsNotRetryable(t *testing.T) {
	err := &ProviderError{Provider: "x", Err: fmt.Errorf("%w: %w", ErrNetwork, context.DeadlineExceeded)}
	if isRetryable(err) {
		t.Error("a timed-out request must not be retryable")
	}
}
```

Confirm the real `isRetryable` symbol/name via `rg -n "func isRetryable|isRetryable\(" core/retry.go` and adapt the call if the API differs (e.g. it may be a method on the policy).

- [ ] **Step 6: Run both — expect PASS** (`go test ./core/ -run 'Timeout|Retryable' -v`). This proves the A1 fix restores the timeout feature for real providers.

- [ ] **Step 7: Commit**

```bash
git add providers/internal/normalize/ providers/ollama/ core/timeout_test.go core/retry_test.go
git commit -m "fix(providers): preserve wrapped error chain in normalization"
```

---

### Task 2: Carry the raw body on ProviderError (Workstream A2)

**Files:**
- Modify: `core/errors.go` (ProviderError struct + Error())
- Modify: `providers/internal/normalize/errors.go` (OpenAIStyleProviderError, add DecodeErrorWithBody)
- Modify hand-rolled normalizers: `providers/anthropic/errors.go`, `providers/gemini/errors.go`, `providers/zai/errors.go`, `providers/voyageai/errors.go`
- Modify OpenAI decode call sites: `providers/openai/client.go`, `providers/openai/client_responses.go`, `providers/openai/streaming.go`, `providers/openai/errors.go` (newDecodeError)
- Test: `core/errors_test.go`, `providers/internal/normalize/errors_test.go`

**Interfaces:**
- Consumes: Task 1's chain-preserving normalizers.
- Produces: `core.ProviderError.Body string`; `normalize.DecodeErrorWithBody(provider string, err error, body []byte) error`; the truncation helper `normalize.truncateBody([]byte) string`.

- [ ] **Step 1: Failing test**

In `core/errors_test.go`:

```go
func TestProviderErrorCarriesBody(t *testing.T) {
	e := &ProviderError{Provider: "openai", Status: 400, Message: "bad", Body: `{"detail":"nope"}`}
	if e.Body == "" {
		t.Fatal("Body should be populated")
	}
	if !strings.Contains(e.Error(), "openai") {
		t.Errorf("Error() = %q", e.Error())
	}
}
```

In `providers/internal/normalize/errors_test.go`:

```go
func TestOpenAIStyleErrorFallsBackToBody(t *testing.T) {
	// Body is valid text but NOT the {"error":{"message"}} envelope.
	body := []byte(`{"detail":"model gpt-x does not exist"}`)
	err := OpenAIStyleProviderError("openai", 404, body, "req-1")
	var pe *core.ProviderError
	if !errors.As(err, &pe) {
		t.Fatal("want *core.ProviderError")
	}
	if !strings.Contains(pe.Message+pe.Body, "does not exist") {
		t.Errorf("real body text lost: msg=%q body=%q", pe.Message, pe.Body)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (Body field/behavior absent).

- [ ] **Step 3: Implement**

`core/errors.go` — add field and adjust Error():

```go
type ProviderError struct {
	Provider  string
	Status    int
	RequestID string
	Code      string
	Message   string
	Body      string // raw response body (truncated); preserved when Message lacks detail
	Err       error
}

func (e *ProviderError) Error() string {
	// Omit the (status,code) suffix for pure network/decode errors.
	if e.Status == 0 && e.Code == "" {
		return fmt.Sprintf("%s: %s", e.Provider, e.Message)
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (status=%d, code=%s, request_id=%s)",
			e.Provider, e.Message, e.Status, e.Code, e.RequestID)
	}
	return fmt.Sprintf("%s: %s (status=%d, code=%s)", e.Provider, e.Message, e.Status, e.Code)
}
```

`providers/internal/normalize/errors.go`:

```go
const maxBodyLen = 4096

func truncateBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxBodyLen {
		return s[:maxBodyLen] + "…(truncated)"
	}
	return s
}

func OpenAIStyleProviderError(provider string, status int, body []byte, requestID string) error {
	var errResp openAIStyleErrorResponse
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Error.Message
	bodyStr := truncateBody(body)
	if message == "" {
		if bodyStr != "" {
			message = bodyStr // surface the real body instead of http.StatusText
		} else {
			message = http.StatusText(status)
		}
	}
	code := errResp.Error.Code
	if code == "" {
		code = errResp.Error.Type
	}
	pe := ProviderError(provider, status, requestID, code, message, SentinelForStatus(status))
	pe.(*core.ProviderError).Body = bodyStr
	return pe
}

func DecodeErrorWithBody(provider string, err error, body []byte) error {
	return &core.ProviderError{
		Provider: provider,
		Message:  err.Error(),
		Body:     truncateBody(body),
		Err:      fmt.Errorf("%w: %w", core.ErrDecode, err),
	}
}
```

Add `"strings"` to imports. Apply the same body-preservation pattern to the four hand-rolled normalizers (anthropic/gemini/zai/voyageai `errors.go`): where they currently fall back to `http.StatusText`, prefer the truncated body, and set `.Body`.

`providers/openai/errors.go`: change `newDecodeError` to accept the body and delegate to `DecodeErrorWithBody`:

```go
func newDecodeError(err error, body []byte) error {
	return normalize.DecodeErrorWithBody("openai", err, body)
}
```

Update OpenAI decode call sites to pass the body they already have (`respBody`): `providers/openai/client.go:63-66`, `client_responses.go` (decode), `streaming.go` decode site. Grep: `rg -n "newDecodeError" providers/openai/`.

- [ ] **Step 4: Run — expect PASS** (`go test ./core/ ./providers/internal/normalize/ ./providers/openai/ -run 'Body|Decode|Error' -v`).

- [ ] **Step 5: Full build (other providers' newDecodeError unchanged, still compile)**

Run: `go build ./...` — expect success. (Only OpenAI's `newDecodeError` signature changed; other providers keep theirs — deferred per spec.)

- [ ] **Step 6: Commit**

```bash
git add core/errors.go providers/internal/normalize/ providers/anthropic/errors.go providers/gemini/errors.go providers/zai/errors.go providers/voyageai/errors.go providers/openai/
git commit -m "fix(errors): preserve raw response body on ProviderError"
```

---

### Task 3: Key sanitization & empty-key validation (Workstream A3)

**Files:**
- Modify: `core/secret.go` (NewSecret, IsEmpty)
- Modify: `providers/openai/provider.go` and/or `client.go`/`client_responses.go`/`streaming.go`/`client_embeddings.go` (empty-key preflight)
- Test: `core/secret_test.go`, `providers/openai/provider_test.go` (or a new `providers/openai/auth_test.go`)

**Interfaces:**
- Produces: trimmed secrets globally; OpenAI Chat/StreamChat/CreateEmbeddings return a descriptive `ErrUnauthorized` error when the key is empty, before any HTTP call.

- [ ] **Step 1: Failing test**

`core/secret_test.go`:

```go
func TestNewSecretTrims(t *testing.T) {
	s := NewSecret("  sk-abc\n")
	if s.Expose() != "sk-abc" {
		t.Errorf("Expose() = %q, want %q", s.Expose(), "sk-abc")
	}
}

func TestIsEmptyTreatsWhitespaceAsEmpty(t *testing.T) {
	if !NewSecret("   ").IsEmpty() {
		t.Error("whitespace-only secret should be empty")
	}
}
```

`providers/openai/auth_test.go` (new):

```go
package openai

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
		Model:    ModelGPT52,
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if hit {
		t.Error("no HTTP request should be made with an empty key")
	}
}
```

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement**

`core/secret.go`: add `"strings"`; `NewSecret` → `return Secret{value: strings.TrimSpace(value)}`; `IsEmpty` → `return strings.TrimSpace(s.value) == ""`.

`providers/openai/`: add a preflight guard used by `Chat`, `StreamChat`, and `CreateEmbeddings`. Add to `provider.go`:

```go
func (p *OpenAI) requireAPIKey() error {
	if p.config.APIKey.IsEmpty() {
		return &core.ProviderError{
			Provider: "openai",
			Message:  "API key is empty; pass it to openai.New(key) or configure your secret source",
			Err:      core.ErrUnauthorized,
		}
	}
	return nil
}
```

Call it at the top of `Chat` (`provider.go:114`), `StreamChat`, and `CreateEmbeddings` (`client_embeddings.go`), returning the error before building/sending the request.

- [ ] **Step 4: Run — expect PASS** (`go test ./core/ -run Secret -v` and `go test ./providers/openai/ -run EmptyKey -v`).

- [ ] **Step 5: Full suite (trim doesn't break existing secret/provider tests)**

Run: `go test ./...` — expect PASS. If any test asserted an untrimmed secret value, it was asserting a bug; fix the assertion to the trimmed value.

- [ ] **Step 6: Commit**

```bash
git add core/secret.go core/secret_test.go providers/openai/
git commit -m "fix(auth): trim secrets and fail fast on empty OpenAI key"
```

---

### Task 4: Wire structured output into the Responses API (Workstream B1)

**Files:**
- Modify: `providers/openai/types_responses.go`, `providers/openai/mapping_responses.go`
- Test: `providers/openai/mapping_responses_test.go` or `client_responses_test.go`

**Interfaces:**
- Produces: `responsesRequest.Text *responsesText` carrying `format.{type,name,schema,strict}`, populated from `req.ResponseFormat`/`req.JSONSchema`.

- [ ] **Step 1: VERIFY the wire shape first**

Use the context7 MCP (`resolve-library-id` → OpenAI, then `query-docs` "Responses API structured outputs text format json_schema strict") to confirm the exact JSON. Expected: `"text": {"format": {"type":"json_schema","name":"...","schema":{...},"strict":true}}`. Record the confirmed shape in the task-4 report. If it differs, adjust the struct in Step 3 accordingly.

- [ ] **Step 2: Failing test**

Append to `providers/openai/client_responses_test.go`:

```go
func TestResponsesAPISendsStrictSchema(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(responsesResponse{ID: "r", Status: "completed", OutputText: "{}"})
	}))
	defer srv.Close()

	p := New("k", WithBaseURL(srv.URL))
	_, err := p.Chat(context.Background(), &core.ChatRequest{
		Model:          ModelGPT52,
		Messages:       []core.Message{{Role: core.RoleUser, Content: "hi"}},
		ResponseFormat: core.ResponseFormatJSONSchema,
		JSONSchema: &core.JSONSchemaDefinition{
			Name:   "person",
			Strict: true,
			Schema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"n":{"type":"string"}},"required":["n"]}`),
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	text, _ := body["text"].(map[string]any)
	format, _ := text["format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Fatalf("text.format.type = %v, want json_schema; body=%v", format["type"], body)
	}
	if format["strict"] != true {
		t.Errorf("text.format.strict = %v, want true", format["strict"])
	}
	if format["name"] != "person" {
		t.Errorf("text.format.name = %v, want person", format["name"])
	}
}
```

- [ ] **Step 3: Run — expect FAIL**, then implement.

`providers/openai/types_responses.go` — add to `responsesRequest`:

```go
	Text *responsesText `json:"text,omitempty"`
```

and new types (adjust to the Step 1 verified shape):

```go
type responsesText struct {
	Format *responsesTextFormat `json:"format,omitempty"`
}

type responsesTextFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
	Strict *bool           `json:"strict,omitempty"`
}
```

`providers/openai/mapping_responses.go` — in `buildResponsesRequest`, before `return respReq`:

```go
	if tf := mapResponsesTextFormat(req); tf != nil {
		respReq.Text = &responsesText{Format: tf}
	}
```

and add:

```go
func mapResponsesTextFormat(req *core.ChatRequest) *responsesTextFormat {
	switch req.ResponseFormat {
	case core.ResponseFormatJSON:
		return &responsesTextFormat{Type: "json_object"}
	case core.ResponseFormatJSONSchema:
		if req.JSONSchema == nil {
			return nil
		}
		strict := req.JSONSchema.Strict
		return &responsesTextFormat{
			Type:   "json_schema",
			Name:   req.JSONSchema.Name,
			Schema: req.JSONSchema.Schema,
			Strict: &strict,
		}
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run — expect PASS** (`go test ./providers/openai/ -run ResponsesAPISendsStrictSchema -v`).

- [ ] **Step 5: Commit**

```bash
git add providers/openai/types_responses.go providers/openai/mapping_responses.go providers/openai/client_responses_test.go
git commit -m "feat(openai): send structured output on the Responses API"
```

---

### Task 5: Strict-by-default + schema validation (Workstream B2)

**Files:**
- Modify: `core/client.go` (ResponseJSONSchema, add ResponseJSONSchemaNonStrict), `core/types.go` (Strict tag), `core/errors.go` (ErrInvalidSchema)
- Create: `core/schema.go` (validator)
- Test: `core/schema_test.go`, `core/client_test.go` (append)

**Interfaces:**
- Consumes: `JSONSchemaDefinition`.
- Produces: `ResponseJSONSchema` forces `Strict=true`; `ResponseJSONSchemaNonStrict(*JSONSchemaDefinition) *ChatBuilder`; `core.ErrInvalidSchema`; `func validateStrictSchema(json.RawMessage) error`.

- [ ] **Step 1: Failing tests**

`core/schema_test.go`:

```go
func TestValidateStrictSchemaRejectsMissingAdditionalProps(t *testing.T) {
	s := json.RawMessage(`{"type":"object","properties":{"n":{"type":"string"}},"required":["n"]}`)
	if err := validateStrictSchema(s); !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}

func TestValidateStrictSchemaAcceptsWellFormed(t *testing.T) {
	s := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"n":{"type":"string"}},"required":["n"]}`)
	if err := validateStrictSchema(s); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
```

`core/client_test.go`:

```go
func TestResponseJSONSchemaDefaultsStrict(t *testing.T) {
	b := NewClient(&mockProvider{}).Chat("m").
		ResponseJSONSchema(&JSONSchemaDefinition{Name: "x", Schema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{},"required":[]}`)})
	if !b.req.JSONSchema.Strict {
		t.Error("ResponseJSONSchema should default Strict=true")
	}
}

func TestResponseJSONSchemaNonStrictOptOut(t *testing.T) {
	b := NewClient(&mockProvider{}).Chat("m").
		ResponseJSONSchemaNonStrict(&JSONSchemaDefinition{Name: "x", Schema: json.RawMessage(`{}`)})
	if b.req.JSONSchema.Strict {
		t.Error("ResponseJSONSchemaNonStrict should set Strict=false")
	}
}
```

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement**

`core/errors.go`: add `ErrInvalidSchema = errors.New("invalid json schema for strict structured output")`.

`core/types.go`: change the tag to `Strict bool \`json:"strict"\`` (drop `omitempty`) so an explicit false is representable.

`core/schema.go` (new): implement `validateStrictSchema(raw json.RawMessage) error` — unmarshal into `map[string]any`; recursively for every node with `"type":"object"`, require `additionalProperties == false` and that the set of `required` entries covers all `properties` keys; on violation return `fmt.Errorf("%w: %s", ErrInvalidSchema, detail)` naming the JSON path (track a path string like `.address.city`). Walk `properties`, `items`, `$defs`.

`core/client.go`:

```go
func (b *ChatBuilder) ResponseJSONSchema(schema *JSONSchemaDefinition) *ChatBuilder {
	if schema != nil {
		schema.Strict = true
	}
	b.req.ResponseFormat = ResponseFormatJSONSchema
	b.req.JSONSchema = schema
	return b
}

func (b *ChatBuilder) ResponseJSONSchemaNonStrict(schema *JSONSchemaDefinition) *ChatBuilder {
	if schema != nil {
		schema.Strict = false
	}
	b.req.ResponseFormat = ResponseFormatJSONSchema
	b.req.JSONSchema = schema
	return b
}
```

In `validate()` (`core/client.go:433`), after the existing checks, add:

```go
	if b.req.ResponseFormat == ResponseFormatJSONSchema && b.req.JSONSchema != nil && b.req.JSONSchema.Strict {
		if err := validateStrictSchema(b.req.JSONSchema.Schema); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Run — expect PASS** (`go test ./core/ -run 'Strict|Schema|NonStrict' -v`).

- [ ] **Step 5: Commit**

```bash
git add core/errors.go core/types.go core/schema.go core/client.go core/schema_test.go core/client_test.go
git commit -m "feat(core): default structured output to strict with schema validation"
```

---

### Task 6: Capability gating — hard error when unsupported (Workstream B3)

**Files:**
- Modify: `core/client.go` (validate/preflight), `core/errors.go` (ErrStructuredOutputUnsupported)
- Modify: `providers/openai/models.go` (ensure structured-output-capable models declare `FeatureStructuredOutput`)
- Audit-only: other providers' `Supports`/model declarations
- Test: `core/client_test.go` (append)

**Interfaces:**
- Consumes: `Provider.Supports`, `core.FeatureStructuredOutput`.
- Produces: `core.ErrStructuredOutputUnsupported`; validate() returns it when a JSON/JSONSchema request targets an unsupporting provider.

- [ ] **Step 1: Failing test**

`core/client_test.go` (mockProvider's `Supports` returns based on the feature — check its current impl at `core/client_test.go:32` and make it return false for `FeatureStructuredOutput`, or add a dedicated provider):

```go
func TestStructuredOutputUnsupportedIsHardError(t *testing.T) {
	// nonStructuredProvider.Supports(FeatureStructuredOutput) == false
	c := NewClient(nonStructuredProvider{})
	_, err := c.Chat("m").User("hi").ResponseJSON().GetResponse(context.Background())
	if !errors.Is(err, ErrStructuredOutputUnsupported) {
		t.Fatalf("err = %v, want ErrStructuredOutputUnsupported", err)
	}
}
```

Add a minimal `nonStructuredProvider` in the test file whose `Supports` returns false and whose `Chat` would fail the test if reached (`t.Fatal`-style sentinel, or return a marker error) so the test proves the gate fires before the provider call.

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement**

`core/errors.go`: add `ErrStructuredOutputUnsupported = errors.New("structured output not supported by this provider or model")`.

`core/client.go` `validate()` — before the strict-schema check from Task 5:

```go
	if b.req.ResponseFormat == ResponseFormatJSON || b.req.ResponseFormat == ResponseFormatJSONSchema {
		if !b.client.provider.Supports(FeatureStructuredOutput) {
			return fmt.Errorf("%w: provider %s model %s",
				ErrStructuredOutputUnsupported, b.client.provider.ID(), b.req.Model)
		}
	}
```

Confirm `"fmt"` is imported in `core/client.go` (it is).

`providers/openai/models.go`: verify the models that support structured output (GPT-4o family, GPT-5.x, o-series) include `core.FeatureStructuredOutput` in their `Capabilities`, and that `OpenAI.Supports(core.FeatureStructuredOutput)` returns true (`provider.go:73`). Add the feature where missing.

Audit (report only, no change unless a provider declares-but-ignores): for anthropic, gemini, ollama, perplexity, zai — confirm each provider's `Supports(FeatureStructuredOutput)` returns **false** OR it actually wires the schema. If any returns true while ignoring the schema (per the earlier audit: anthropic/ollama/perplexity/zai ignore it), change that provider's `Supports` to return false for `FeatureStructuredOutput` so the gate protects callers. Record findings in the task-6 report.

- [ ] **Step 4: Run — expect PASS**, then full suite `go test ./...` (the gate must not break existing structured-output tests on OpenAI, which supports it).

- [ ] **Step 5: Commit**

```bash
git add core/errors.go core/client.go providers/openai/models.go providers/*/provider.go core/client_test.go
git commit -m "feat(core): hard-error structured output on unsupported providers"
```

---

### Task 7: Fix DrainStream error-swallowing race (Workstream C1)

**Files:**
- Modify: `core/streaming.go` (DrainStream)
- Test: `core/streaming_test.go` (append)

**Interfaces:**
- Produces: `DrainStream` never returns `(partial, nil)` when an error is delivered on `Err`, even if `Ch` closes first.

- [ ] **Step 1: Failing test**

Append to `core/streaming_test.go` a stream whose `Ch` closes before the error is delivered on `Err`:

```go
func TestDrainStreamDoesNotSwallowLateError(t *testing.T) {
	ch := make(chan ChatChunk, 1)
	errc := make(chan error, 1)
	final := make(chan *ChatResponse, 1)
	ch <- ChatChunk{Delta: "partial"}
	close(ch) // Ch closes FIRST
	go func() {
		// error delivered slightly later, after Ch is already closed
		errc <- &ProviderError{Provider: "x", Message: "mid-stream boom", Err: ErrServer}
		close(errc)
		close(final)
	}()
	s := &ChatStream{Ch: ch, Err: errc, Final: final}
	_, err := DrainStream(context.Background(), s)
	if err == nil {
		t.Fatal("DrainStream returned nil error; the late stream error was swallowed")
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (currently returns nil error with partial text), run under `-race` too.

- [ ] **Step 3: Implement** — rewrite `DrainStream` to mirror the non-lossy discipline in `wrapStreamWithTelemetry` (`core/client.go:800-868`): keep reading after `Ch` closes until both `Err` and `Final` resolve; on `Final` closing without a value, check `Err`; on `ctx.Done()`, do a final non-blocking drain of `Err`/`Final` before returning `ctx.Err()`. Preserve the existing accumulation of `Ch` deltas.

- [ ] **Step 4: Run — expect PASS**, incl. `go test -race ./core/ -run DrainStream -count=20`.

- [ ] **Step 5: Commit**

```bash
git add core/streaming.go core/streaming_test.go
git commit -m "fix(core): stop DrainStream swallowing late stream errors"
```

---

### Task 8: Fix batch-poll infinite hang (Workstream C2)

**Files:**
- Modify: `core/batch.go` (BatchWaiter.Wait)
- Test: `core/batch_test.go` (append)

**Interfaces:**
- Produces: `Wait` bounds each poll by the remaining `maxWait` budget so a hung poll cannot exceed it.

- [ ] **Step 1: Failing test**

```go
func TestBatchWaiterDoesNotHangOnStuckPoll(t *testing.T) {
	p := &stuckBatchProvider{} // GetBatchStatus blocks on <-ctx.Done()
	w := NewBatchWaiter(p, /* interval */ 10*time.Millisecond, /* maxWait */ 100*time.Millisecond)
	start := time.Now()
	_, err := w.Wait(context.Background(), "batch-1")
	if err == nil {
		t.Fatal("want timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Wait hung for %v; maxWait budget not enforced per-poll", time.Since(start))
	}
}
```

Check the real `NewBatchWaiter`/`BatchWaiter.Wait` signatures (`rg -n "func NewBatchWaiter|func .*BatchWaiter. Wait" core/batch.go`) and the existing batch test mocks (`core/batch_test.go`) — reuse/extend a mock rather than inventing one; add a `stuckBatchProvider` whose `GetBatchStatus` blocks on `<-ctx.Done()`.

- [ ] **Step 2: Run — expect FAIL/hang** (bounded by `go test -timeout 30s`).

- [ ] **Step 3: Implement** — in `Wait`, compute an overall deadline from `maxWait` and derive a per-poll context: `pollCtx, cancel := context.WithDeadline(ctx, deadline)` (or `WithTimeout` for the remaining budget) for each `GetBatchStatus` call, and return a deadline error once `now > deadline`. Ensure `cancel()` runs each iteration (no leak).

- [ ] **Step 4: Run — expect PASS** (`go test ./core/ -run BatchWaiter -v -timeout 30s`), and `-race`.

- [ ] **Step 5: Commit**

```bash
git add core/batch.go core/batch_test.go
git commit -m "fix(core): bound batch poll by maxWait to prevent hang"
```

---

### Task 9: Change doc + timeout-gap docs + full verification

**Files:**
- Create: `docs/changes/2026-08-01_v0.16.0_sdk-reliability-hardening.md`
- Modify: doc comments on the uncovered non-chat methods (embeddings/batch/files/images) noting they require a caller-supplied context deadline

**Interfaces:** none.

- [ ] **Step 1: Full suite with race** — `go test -race ./...` — expect PASS.
- [ ] **Step 2: Build + fmt** — `go build ./...`; `gofmt -l .` (expect empty besides pre-existing untracked files).
- [ ] **Step 3: Change doc** per repo `CLAUDE.md` structure: `product: iris`, `change_type: feature`. `affected_components` must list every file touched across Tasks 1-8. Document exactly: `ErrTimeout` restoration for real providers, `ProviderError.Body`, key trimming + empty-key error, Responses-API structured output, strict-by-default (+ `ErrInvalidSchema`, `ResponseJSONSchemaNonStrict`), capability gating (`ErrStructuredOutputUnsupported`), `DrainStream` and batch-poll fixes, and the two behavior changes. Include a "Deferred" section: other providers' empty-key preflight, native structured output on anthropic/ollama/gemini, broad non-chat timeout coverage, client-side response validation.
- [ ] **Step 4: Timeout-gap doc comments** — add a one-line note to the Go doc comment of each non-chat entry point (`CreateEmbeddings`, batch methods, files, images) that `core.WithTimeout` does not apply and a context deadline must be supplied by the caller.
- [ ] **Step 5: Commit**

```bash
git add docs/changes/ providers/ core/
git commit -m "docs: change doc and timeout-gap notes for reliability hardening"
```

---

## Self-Review

**Spec coverage:** A1→T1, A2→T2, A3→T3, B1→T4, B2→T5, B3→T6, C1→T7, C2→T8, C3(docs)→T9. Both behavior changes (strict default, unsupported hard-error) are implemented (T5/T6) and documented (T9). Deferrals are recorded in T9's change doc. All spec sections covered.

**Placeholder scan:** No TBD/TODO. Every external-contract unknown (Responses API shape in T4, `isRetryable` name in T1, `NewBatchWaiter` signature in T8, mockProvider `Supports` in T6) is called out with an explicit `rg`/context7 verification step and a fallback, not left vague.

**Type consistency:** `ProviderError.Body string`, `DecodeErrorWithBody(provider, err, body)`, `validateStrictSchema(json.RawMessage) error`, `ResponseJSONSchemaNonStrict(*JSONSchemaDefinition) *ChatBuilder`, `ErrInvalidSchema`, `ErrStructuredOutputUnsupported`, `responsesTextFormat` used consistently across tasks. Task 2 changes only OpenAI's `newDecodeError` signature; other providers' wrappers are explicitly left intact so the build stays green.
