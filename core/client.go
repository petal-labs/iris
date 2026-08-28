package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

// DefaultTimeout is the execution timeout applied to calls routed through
// Client, including chat, streaming, and embeddings, when the caller's
// context does not already supply a deadline. Disable it with WithTimeout(0).
const DefaultTimeout = 120 * time.Second

// Provider is the interface that LLM providers must implement.
// Providers SHOULD be safe for concurrent calls.
// If a provider cannot be concurrent-safe, it MUST document this.
type Provider interface {
	// ID returns the provider identifier (e.g., "openai", "anthropic").
	ID() string

	// Models returns the list of models available from this provider.
	Models() []ModelInfo

	// Supports reports whether the provider supports the given feature.
	Supports(feature Feature) bool

	// Chat sends a non-streaming chat request.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat sends a streaming chat request.
	StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error)
}

// ImageGenerator is an optional interface for providers that support image generation.
type ImageGenerator interface {
	// GenerateImage generates images from a text prompt.
	GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error)

	// EditImage edits existing images using a prompt and optional mask.
	EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error)

	// StreamImage generates images with streaming partial results.
	// Not all providers support streaming.
	StreamImage(ctx context.Context, req *ImageGenerateRequest) (*ImageStream, error)
}

// AsImageGenerator attempts to cast a Provider to ImageGenerator. It returns
// nil and false when the provider does not implement image operations.
func AsImageGenerator(p Provider) (ImageGenerator, bool) {
	generator, ok := p.(ImageGenerator)
	return generator, ok
}

// Client is the main entry point for interacting with LLM providers.
// Client is safe for concurrent use.
type Client struct {
	provider          Provider
	telemetry         TelemetryHook
	retry             RetryPolicy
	rateLimiter       RateLimiter
	warningHandler    WarningHandler
	timeout           time.Duration
	streamIdleTimeout time.Duration
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WarningHandler receives non-fatal warnings emitted by the SDK.
// Implementations should be safe for concurrent use.
type WarningHandler func(message string)

// NewClient creates a new Client with the given provider and options.
func NewClient(p Provider, opts ...ClientOption) *Client {
	c := &Client{
		provider:       p,
		telemetry:      NoopTelemetryHook{},
		retry:          DefaultRetryPolicy(),
		warningHandler: func(string) {},
		timeout:        DefaultTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithTelemetry sets the telemetry hook for the client.
func WithTelemetry(h TelemetryHook) ClientOption {
	return func(c *Client) {
		if h != nil {
			c.telemetry = h
		}
	}
}

// WithRetryPolicy sets the retry policy for the client.
func WithRetryPolicy(r RetryPolicy) ClientOption {
	return func(c *Client) {
		if r != nil {
			c.retry = r
		}
	}
}

// WithRateLimiter configures a limiter used before every provider attempt,
// including retries. Pass nil to leave rate limiting disabled.
func WithRateLimiter(l RateLimiter) ClientOption {
	return func(c *Client) {
		if l != nil {
			c.rateLimiter = l
		}
	}
}

// WithWarningHandler sets a handler for non-fatal SDK warnings.
// Pass nil to keep the default no-op handler.
func WithWarningHandler(h WarningHandler) ClientOption {
	return func(c *Client) {
		if h != nil {
			c.warningHandler = h
		}
	}
}

// WithTimeout sets the default execution timeout applied to calls routed
// through Client when the caller's context has no deadline of its own. Chat
// builders can override it per call with Timeout(). Pass 0 to disable the
// default and allow unbounded calls.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = d
	}
}

// WithStreamIdleTimeout sets a stall/idle timeout for streaming calls: if a
// stream produces no chunk for at least d, it is terminated and the
// resulting error (delivered on the stream's Err channel, and returned by
// DrainStream) satisfies errors.Is(err, ErrTimeout). This is independent of
// WithTimeout, which bounds the overall call; WithStreamIdleTimeout instead
// bounds the gap between successive chunks, so a provider that goes silent
// mid-stream is caught well before a long overall timeout would elapse.
//
// Pass 0 (the default) to disable idle detection.
func WithStreamIdleTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.streamIdleTimeout = d
	}
}

// Provider returns the underlying provider.
func (c *Client) Provider() Provider {
	return c.provider
}

// Chat returns a ChatBuilder for constructing and executing a chat request.
func (c *Client) Chat(model ModelID) *ChatBuilder {
	return &ChatBuilder{
		client: c,
		req: ChatRequest{
			Model: model,
		},
	}
}

// ChatBuilder provides a fluent API for building chat requests.
// ChatBuilder is NOT thread-safe and should not be shared across goroutines.
// Most configuration methods mutate the receiver and return the same builder.
// Clone returns an independent builder; ToolResults, ToolResult, and ToolError
// are the only configuration methods that clone automatically.
type ChatBuilder struct {
	client  *Client
	req     ChatRequest
	timeout time.Duration // optional timeout for GetResponse/Stream
}

// System appends a system message.
func (b *ChatBuilder) System(s string) *ChatBuilder {
	b.req.Messages = append(b.req.Messages, Message{Role: RoleSystem, Content: s})
	return b
}

// User appends a user message.
func (b *ChatBuilder) User(s string) *ChatBuilder {
	b.req.Messages = append(b.req.Messages, Message{Role: RoleUser, Content: s})
	return b
}

// Assistant appends an assistant message.
func (b *ChatBuilder) Assistant(s string) *ChatBuilder {
	b.req.Messages = append(b.req.Messages, Message{Role: RoleAssistant, Content: s})
	return b
}

// Temperature sets the temperature parameter.
func (b *ChatBuilder) Temperature(v float32) *ChatBuilder {
	b.req.Temperature = &v
	return b
}

// MaxTokens sets the maximum tokens parameter.
func (b *ChatBuilder) MaxTokens(n int) *ChatBuilder {
	b.req.MaxTokens = &n
	return b
}

// Tools sets the tools available for the request. It copies the caller's slice,
// so later changes to the slice do not affect the request. The Tool values are
// retained as interfaces and must not be mutated while the builder is in use.
func (b *ChatBuilder) Tools(ts ...Tool) *ChatBuilder {
	b.req.Tools = slices.Clone(ts)
	return b
}

// Instructions sets the system instructions (Responses API style).
// For Chat Completions API, this is equivalent to adding a system message.
func (b *ChatBuilder) Instructions(s string) *ChatBuilder {
	b.req.Instructions = s
	return b
}

// ReasoningEffort sets the reasoning effort level for models that support it.
func (b *ChatBuilder) ReasoningEffort(level ReasoningEffort) *ChatBuilder {
	b.req.ReasoningEffort = level
	return b
}

// SearchOptions configures web-search grounding for providers that support
// FeatureWebSearch (currently Perplexity). Passing a nil pointer clears any
// previously set options. Requests carrying search options against a
// provider without the capability fail validation with ErrSearchUnsupported
// before any HTTP call is made.
//
// Note: this is distinct from WebSearch(), which enables OpenAI's built-in
// web_search tool on the Responses API. SearchOptions controls how
// search-native providers ground their results.
//
//	resp, err := client.Chat(perplexity.ModelSonar).
//	    User("Latest Go release notes").
//	    SearchOptions(&core.SearchOptions{
//	        SearchDomainFilter: []string{"go.dev"},
//	        Recency:            core.SearchRecencyMonth,
//	    }).
//	    GetResponse(ctx)
//
//	for _, url := range resp.Citations {
//	    fmt.Println(url)
//	}
func (b *ChatBuilder) SearchOptions(opts *SearchOptions) *ChatBuilder {
	b.req.SearchOptions = opts
	return b
}

// BuiltInTool adds a built-in tool to the request.
func (b *ChatBuilder) BuiltInTool(toolType string) *ChatBuilder {
	b.req.BuiltInTools = append(b.req.BuiltInTools, BuiltInTool{Type: toolType})
	return b
}

// WebSearch adds the web_search built-in tool.
func (b *ChatBuilder) WebSearch() *ChatBuilder {
	return b.BuiltInTool("web_search")
}

// FileSearch adds the file_search built-in tool with optional vector store IDs.
func (b *ChatBuilder) FileSearch(vectorStoreIDs ...string) *ChatBuilder {
	// Add the file_search tool
	b.req.BuiltInTools = append(b.req.BuiltInTools, BuiltInTool{Type: "file_search"})

	// If vector store IDs are provided, set up tool resources
	if len(vectorStoreIDs) > 0 {
		if b.req.ToolResources == nil {
			b.req.ToolResources = &ToolResources{}
		}
		b.req.ToolResources.FileSearch = &FileSearchResources{
			VectorStoreIDs: vectorStoreIDs,
		}
	}

	return b
}

// CodeInterpreter adds the code_interpreter built-in tool.
func (b *ChatBuilder) CodeInterpreter() *ChatBuilder {
	return b.BuiltInTool("code_interpreter")
}

// ContinueFrom chains this request to a previous response.
func (b *ChatBuilder) ContinueFrom(responseID string) *ChatBuilder {
	b.req.PreviousResponseID = responseID
	return b
}

// Timeout sets an optional timeout for the request.
// When set, GetResponse and Stream will create a context with this timeout
// if a context.Background() or context without deadline is passed.
// This provides a convenient alternative to manually creating contexts:
//
//	// Instead of:
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	resp, err := client.Chat(model).User("Hello").GetResponse(ctx)
//
//	// You can write:
//	resp, err := client.Chat(model).User("Hello").Timeout(30*time.Second).GetResponse(context.Background())
func (b *ChatBuilder) Timeout(d time.Duration) *ChatBuilder {
	b.timeout = d
	return b
}

// Clone creates a deep copy of the ChatBuilder.
// This is useful for reusing a base configuration across multiple requests:
//
//	base := client.Chat(model).System("You are a helpful assistant").Temperature(0.7)
//	resp1, _ := base.Clone().User("Question 1").GetResponse(ctx)
//	resp2, _ := base.Clone().User("Question 2").GetResponse(ctx)
//
// The original builder remains unchanged after cloning.
func (b *ChatBuilder) Clone() *ChatBuilder {
	clone := &ChatBuilder{
		client:  b.client,
		timeout: b.timeout,
		req: ChatRequest{
			Model:              b.req.Model,
			Instructions:       b.req.Instructions,
			ReasoningEffort:    b.req.ReasoningEffort,
			PreviousResponseID: b.req.PreviousResponseID,
			Truncation:         b.req.Truncation,
			ResponseFormat:     b.req.ResponseFormat,
		},
	}

	// Deep copy pointer values
	if b.req.Temperature != nil {
		t := *b.req.Temperature
		clone.req.Temperature = &t
	}
	if b.req.MaxTokens != nil {
		m := *b.req.MaxTokens
		clone.req.MaxTokens = &m
	}
	if b.req.JSONSchema != nil {
		clone.req.JSONSchema = cloneJSONSchemaDefinition(b.req.JSONSchema)
	}

	// Deep copy slices
	if len(b.req.Messages) > 0 {
		clone.req.Messages = make([]Message, len(b.req.Messages))
		for i, msg := range b.req.Messages {
			clone.req.Messages[i] = Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
			if len(msg.Parts) > 0 {
				clone.req.Messages[i].Parts = make([]ContentPart, len(msg.Parts))
				copy(clone.req.Messages[i].Parts, msg.Parts)
			}
			if len(msg.ToolCalls) > 0 {
				clone.req.Messages[i].ToolCalls = make([]ToolCall, len(msg.ToolCalls))
				copy(clone.req.Messages[i].ToolCalls, msg.ToolCalls)
			}
			if len(msg.ToolResults) > 0 {
				clone.req.Messages[i].ToolResults = make([]ToolResult, len(msg.ToolResults))
				copy(clone.req.Messages[i].ToolResults, msg.ToolResults)
			}
		}
	}

	if len(b.req.Tools) > 0 {
		clone.req.Tools = make([]Tool, len(b.req.Tools))
		copy(clone.req.Tools, b.req.Tools)
	}

	if len(b.req.BuiltInTools) > 0 {
		clone.req.BuiltInTools = make([]BuiltInTool, len(b.req.BuiltInTools))
		copy(clone.req.BuiltInTools, b.req.BuiltInTools)
	}

	// Deep copy ToolResources
	if b.req.ToolResources != nil {
		clone.req.ToolResources = &ToolResources{}
		if b.req.ToolResources.FileSearch != nil {
			clone.req.ToolResources.FileSearch = &FileSearchResources{
				VectorStoreIDs: make([]string, len(b.req.ToolResources.FileSearch.VectorStoreIDs)),
			}
			copy(clone.req.ToolResources.FileSearch.VectorStoreIDs, b.req.ToolResources.FileSearch.VectorStoreIDs)
		}
	}

	// Deep copy SearchOptions
	if b.req.SearchOptions != nil {
		searchCopy := *b.req.SearchOptions
		if len(b.req.SearchOptions.SearchDomainFilter) > 0 {
			searchCopy.SearchDomainFilter = make([]string, len(b.req.SearchOptions.SearchDomainFilter))
			copy(searchCopy.SearchDomainFilter, b.req.SearchOptions.SearchDomainFilter)
		}
		clone.req.SearchOptions = &searchCopy
	}

	return clone
}

// Truncation sets the truncation mode for the request.
func (b *ChatBuilder) Truncation(mode string) *ChatBuilder {
	b.req.Truncation = mode
	return b
}

// ResponseJSON constrains the model output to valid JSON.
// This enables JSON mode where the model outputs syntactically valid JSON.
// Note: The model may still produce invalid JSON in edge cases; always validate.
func (b *ChatBuilder) ResponseJSON() *ChatBuilder {
	b.req.ResponseFormat = ResponseFormatJSON
	return b
}

// ResponseJSONSchema constrains the model output to match a specific JSON Schema.
// This enables structured output mode where the model produces JSON conforming to the schema.
// The schema parameter defines the structure the output must conform to.
//
// This copies schema, including its raw JSON bytes, and forces Strict = true on
// the copy without modifying the caller's value. Strict mode requires the
// schema to set "additionalProperties": false and list every property in
// "required" at every object node; validate() rejects non-compliant schemas
// with ErrInvalidSchema before the request is sent. Use
// ResponseJSONSchemaNonStrict to opt out of strict mode for schemas that cannot
// meet those constraints.
//
// Example:
//
//	schema := &core.JSONSchemaDefinition{
//	    Name:   "person",
//	    Schema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}`),
//	}
//	resp, err := client.Chat(model).
//	    User("Extract: John is 30 years old").
//	    ResponseJSONSchema(schema).
//	    GetResponse(ctx)
func (b *ChatBuilder) ResponseJSONSchema(schema *JSONSchemaDefinition) *ChatBuilder {
	schemaCopy := cloneJSONSchemaDefinition(schema)
	if schemaCopy != nil {
		schemaCopy.Strict = true
	}
	b.req.ResponseFormat = ResponseFormatJSONSchema
	b.req.JSONSchema = schemaCopy
	return b
}

// ResponseJSONSchemaNonStrict constrains the model output to match a specific
// JSON Schema without enforcing strict mode. This copies schema, including its
// raw JSON bytes, and forces Strict = false on the copy without modifying the
// caller's value. It skips the strict-schema validation that ResponseJSONSchema
// applies. Use this when a schema cannot satisfy strict mode's requirements
// (every object node needs "additionalProperties": false and a "required"
// array covering all declared properties) and the provider accepts loose
// schemas.
func (b *ChatBuilder) ResponseJSONSchemaNonStrict(schema *JSONSchemaDefinition) *ChatBuilder {
	schemaCopy := cloneJSONSchemaDefinition(schema)
	if schemaCopy != nil {
		schemaCopy.Strict = false
	}
	b.req.ResponseFormat = ResponseFormatJSONSchema
	b.req.JSONSchema = schemaCopy
	return b
}

func cloneJSONSchemaDefinition(schema *JSONSchemaDefinition) *JSONSchemaDefinition {
	if schema == nil {
		return nil
	}

	clone := *schema
	clone.Schema = slices.Clone(schema.Schema)
	return &clone
}

// ResponseText explicitly sets the response format to text (the default).
// This is mainly useful for clearing a previously set JSON format.
func (b *ChatBuilder) ResponseText() *ChatBuilder {
	b.req.ResponseFormat = ResponseFormatText
	b.req.JSONSchema = nil
	return b
}

// ToolResults returns a new ChatBuilder with tool execution results appended.
// This automatically formats results according to the provider's expected format.
// The assistant message containing the original tool calls is automatically included.
//
// IMPORTANT: This method returns a NEW builder (immutable). The original builder is unchanged.
//
// If fewer results are provided than tool calls, a warning is emitted via the client warning handler.
// If result IDs don't match any tool calls, a warning is emitted via the client warning handler.
func (b *ChatBuilder) ToolResults(assistantResp *ChatResponse, results []ToolResult) *ChatBuilder {
	// Always clone to maintain immutability
	newBuilder := b.Clone()

	if assistantResp == nil || !assistantResp.HasToolCalls() {
		return newBuilder
	}

	// Build lookup maps for validation
	callIDs := make(map[string]string) // callID -> toolName
	for _, tc := range assistantResp.ToolCalls {
		callIDs[tc.ID] = tc.Name
	}

	providedIDs := make(map[string]bool)
	for _, r := range results {
		providedIDs[r.CallID] = true
		// Warn about results that don't match any tool call
		if _, ok := callIDs[r.CallID]; !ok {
			b.client.warnf("tool result ID %q does not match any tool call", r.CallID)
		}
	}

	// Warn about tool calls without results
	for id, name := range callIDs {
		if !providedIDs[id] {
			b.client.warnf("no result provided for tool call %q (tool: %s)", id, name)
		}
	}

	// Add assistant message with tool calls (required by all providers)
	newBuilder.req.Messages = append(newBuilder.req.Messages, Message{
		Role:      RoleAssistant,
		ToolCalls: assistantResp.ToolCalls,
	})

	// Add tool results - actual formatting happens in provider mapping
	newBuilder.req.Messages = append(newBuilder.req.Messages, Message{
		Role:        RoleTool,
		ToolResults: results,
	})

	return newBuilder
}

// ToolResult is a convenience method for adding a single successful tool result.
// Returns a new builder (immutable).
func (b *ChatBuilder) ToolResult(assistantResp *ChatResponse, callID string, content any) *ChatBuilder {
	return b.ToolResults(assistantResp, []ToolResult{{
		CallID:  callID,
		Content: content,
		IsError: false,
	}})
}

// ToolError is a convenience method for adding a single tool error result.
// Returns a new builder (immutable).
func (b *ChatBuilder) ToolError(assistantResp *ChatResponse, callID string, err error) *ChatBuilder {
	return b.ToolResults(assistantResp, []ToolResult{{
		CallID:  callID,
		Content: err.Error(),
		IsError: true,
	}})
}

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

// validate checks that the request is valid.
func (b *ChatBuilder) validate() error {
	if b.req.Model == "" {
		return ErrModelRequired
	}
	if len(b.req.Messages) == 0 {
		return ErrNoMessages
	}

	// Validate each message has content (Content, Parts, ToolCalls, or ToolResults)
	for _, msg := range b.req.Messages {
		hasContent := msg.Content != "" || len(msg.Parts) > 0 || len(msg.ToolCalls) > 0 || len(msg.ToolResults) > 0
		if !hasContent {
			return ErrNoMessages
		}
	}

	// Capability gate: reject schema-based structured output requests
	// (ResponseFormatJSONSchema) against a provider/model that does not
	// support core.FeatureStructuredOutput. This must run before the
	// strict-schema check below so an unsupported provider fails with
	// ErrStructuredOutputUnsupported rather than a schema validation error.
	//
	// Plain JSON mode (ResponseFormatJSON / json_object) is intentionally
	// NOT hard-gated here: it has no schema/shape contract to silently
	// violate, and many models support json_object without being tagged
	// with FeatureStructuredOutput (e.g. OpenAI's gpt-3.5-turbo, gpt-4, and
	// gpt-4-turbo). Gating it would reject requests that previously worked.
	// If a provider genuinely can't do json_object, its descriptive API
	// error surfaces instead.
	if b.req.ResponseFormat == ResponseFormatJSONSchema {
		if !b.client.provider.Supports(FeatureStructuredOutput) {
			return fmt.Errorf("%w: provider %s model %s",
				ErrStructuredOutputUnsupported, b.client.provider.ID(), b.req.Model)
		}

		// Model-level gate: the provider supports structured output overall,
		// but the specific requested model may not (e.g. OpenAI supports it,
		// but gpt-3.5-turbo / gpt-4 do not). If the model is present in the
		// provider's catalog and lacks the capability, reject it. If the
		// model is NOT present in the catalog (unknown, brand-new, or a
		// custom deployment not yet reflected in the static catalog), fall
		// back to allowing it since the provider-level check already passed.
		for _, m := range b.client.provider.Models() {
			if m.ID == b.req.Model {
				if !m.HasCapability(FeatureStructuredOutput) {
					return fmt.Errorf("%w: provider %s model %s",
						ErrStructuredOutputUnsupported, b.client.provider.ID(), b.req.Model)
				}
				break
			}
		}
	}

	// Capability gate: reject requests carrying web-search options against
	// a provider that does not support core.FeatureWebSearch. This fails
	// fast instead of silently sending a request whose search directives
	// the provider would ignore.
	if b.req.SearchOptions != nil {
		if !b.client.provider.Supports(FeatureWebSearch) {
			return fmt.Errorf("%w: provider %s model %s",
				ErrSearchUnsupported, b.client.provider.ID(), b.req.Model)
		}
	}

	// Strict structured output requires a schema shape the provider can
	// enforce exactly.
	if b.req.ResponseFormat == ResponseFormatJSONSchema && b.req.JSONSchema != nil && b.req.JSONSchema.Strict {
		if err := validateStrictSchema(b.req.JSONSchema.Schema); err != nil {
			return err
		}
	}

	return nil
}

// GetResponse executes the chat request and returns the response.
// It applies validation, telemetry, and retry logic.
// If Timeout was set and ctx has no deadline, a timeout context is created internally.
func (b *ChatBuilder) GetResponse(ctx context.Context) (*ChatResponse, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}
	b.warnUnsupportedContentParts()

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

	start := time.Now()
	providerID := b.client.provider.ID()
	startEvent := RequestStartEvent{
		Provider: providerID,
		Model:    b.req.Model,
		Start:    start,
	}

	// Emit telemetry start (with context if supported)
	if ctxHook, ok := b.client.telemetry.(ContextualTelemetryHook); ok {
		ctx = ctxHook.OnRequestStartWithContext(ctx, startEvent)
	} else {
		b.client.telemetry.OnRequestStart(startEvent)
	}

	var resp *ChatResponse
	var err error

	// Execute with retry logic
retryLoop:
	for attempt := 0; ; attempt++ {
		if err = b.client.waitForRateLimit(ctx); err != nil {
			break
		}
		resp, err = b.client.provider.Chat(ctx, &b.req)
		if err == nil {
			break
		}

		// Check if we should retry
		delay, shouldRetry := b.client.retry.NextDelay(attempt, err)
		if !shouldRetry {
			break
		}

		// Wait before retry, respecting context cancellation
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break retryLoop
		case <-time.After(delay):
			continue
		}
	}

	// Map an Iris-applied deadline to a legible typed error.
	if timedOut > 0 && err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = newTimeoutError(timedOut, providerID, b.req.Model)
	}

	// Emit telemetry end
	end := time.Now()
	usage := TokenUsage{}
	if resp != nil {
		usage = resp.Usage
	}
	endEvent := RequestEndEvent{
		Provider: providerID,
		Model:    b.req.Model,
		Start:    start,
		End:      end,
		Usage:    usage,
		Err:      err,
	}

	if ctxHook, ok := b.client.telemetry.(ContextualTelemetryHook); ok {
		ctxHook.OnRequestEndWithContext(ctx, endEvent)
	} else {
		b.client.telemetry.OnRequestEnd(endEvent)
	}

	return resp, err
}

// Stream executes the chat request and returns a streaming response.
// It applies validation and telemetry.
//
// The effective execution timeout (see effectiveTimeout for precedence) is
// applied to the context used to establish the stream. The derived timeout
// context is NOT cancelled when Stream returns; it stays alive until the
// stream completes (successfully or with an error) or the deadline fires,
// so the timeout does not truncate an in-flight stream prematurely.
//
// A caller-supplied ctx deadline is never remapped to ErrTimeout — only a
// deadline applied internally by Iris is. To apply a timeout externally
// instead:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	defer cancel()
//	stream, err := client.Chat(model).User("...").Stream(ctx)
func (b *ChatBuilder) Stream(ctx context.Context) (*ChatStream, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}
	b.warnUnsupportedContentParts()

	// Apply the effective execution timeout (see effectiveTimeout for precedence).
	// cancel is intentionally NOT deferred here: on success it is handed to
	// wrapStreamWithTelemetry via onDone, which fires it exactly once when the
	// stream completes or the deadline fires. On a setup error we cancel
	// immediately below.
	var (
		cancel  context.CancelFunc
		timeout time.Duration
	)
	if d := b.effectiveTimeout(ctx); d > 0 {
		ctx, cancel = context.WithTimeout(ctx, d)
		timeout = d
	}

	start := time.Now()
	providerID := b.client.provider.ID()
	startEvent := RequestStartEvent{
		Provider: providerID,
		Model:    b.req.Model,
		Start:    start,
	}

	// Emit telemetry start (with context if supported)
	if ctxHook, ok := b.client.telemetry.(ContextualTelemetryHook); ok {
		ctx = ctxHook.OnRequestStartWithContext(ctx, startEvent)
	} else {
		b.client.telemetry.OnRequestStart(startEvent)
	}

	// A configured idle timeout needs a context the watchdog can cancel
	// on its own, independent of (and nested inside) any execution-timeout
	// deadline established above, so a stall can be terminated immediately
	// without waiting for the full execution timeout to elapse.
	var idleCancel context.CancelFunc
	if b.client.streamIdleTimeout > 0 {
		ctx, idleCancel = context.WithCancel(ctx)
	}

	var stream *ChatStream
	var err error

retryLoop:
	for attempt := 0; ; attempt++ {
		if err = b.client.waitForRateLimit(ctx); err != nil {
			break
		}
		stream, err = b.client.provider.StreamChat(ctx, &b.req)
		if err == nil {
			break
		}

		delay, shouldRetry := b.client.retry.NextDelay(attempt, err)
		if !shouldRetry {
			break
		}
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break retryLoop
		case <-time.After(delay):
		}
	}
	if err != nil {
		if idleCancel != nil {
			idleCancel()
		}
		if cancel != nil {
			cancel()
		}
		// Map an Iris-applied deadline to a legible typed error.
		if timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			err = newTimeoutError(timeout, providerID, b.req.Model)
		}
		// Emit telemetry end on immediate error
		endEvent := RequestEndEvent{
			Provider: providerID,
			Model:    b.req.Model,
			Start:    start,
			End:      time.Now(),
			Err:      err,
		}
		if ctxHook, ok := b.client.telemetry.(ContextualTelemetryHook); ok {
			ctxHook.OnRequestEndWithContext(ctx, endEvent)
		} else {
			b.client.telemetry.OnRequestEnd(endEvent)
		}
		return nil, err
	}

	// Wrap with the idle watchdog BEFORE wrapStreamWithTelemetry so telemetry
	// observes the idle-wrapped Ch/Err/Final (and thus still fires its
	// end-of-stream event correctly whether the stream finishes normally or
	// is cut short by an idle timeout).
	if idleCancel != nil {
		stream = wrapStreamWithIdleTimeout(stream, b.client.streamIdleTimeout, idleCancel)
	}

	// onDone releases the timeout and idle contexts exactly once, when the
	// stream completes (success or error) or a deadline/idle fires — never
	// when Stream itself returns. context.CancelFunc is idempotent, so
	// calling idleCancel here is safe even if the idle watchdog already
	// called it itself on an idle fire.
	onDone := func() {
		if idleCancel != nil {
			idleCancel()
		}
		if cancel != nil {
			cancel()
		}
	}
	// mapErr remaps a mid-stream Iris-applied deadline to ErrTimeout while
	// leaving caller-supplied deadlines and all other errors untouched. An
	// error that already satisfies errors.Is(_, ErrTimeout) — such as the
	// stream idle error, which also wraps context.DeadlineExceeded to signal
	// a real deadline was involved — is returned unchanged so it is not
	// overwritten by a misleading "execution timeout" message: the overall
	// execution timeout (if any) has not necessarily elapsed at all when the
	// idle watchdog fires.
	mapErr := func(e error) error {
		if errors.Is(e, ErrTimeout) {
			return e
		}
		if timeout > 0 && errors.Is(e, context.DeadlineExceeded) {
			return newTimeoutError(timeout, providerID, b.req.Model)
		}
		return e
	}

	// Wrap the stream to emit telemetry when it completes
	return wrapStreamWithTelemetry(ctx, stream, b.client.telemetry, providerID, b.req.Model, start, onDone, mapErr), nil
}

func (c *Client) waitForRateLimit(ctx context.Context) error {
	if c.rateLimiter == nil {
		return contextError(ctx)
	}
	return c.rateLimiter.Wait(ctx)
}

// MessageBuilder provides a fluent API for building multimodal messages.
type MessageBuilder struct {
	parent *ChatBuilder
	role   Role
	parts  []ContentPart
}

// UserMultimodal starts building a multimodal user message.
func (b *ChatBuilder) UserMultimodal() *MessageBuilder {
	return &MessageBuilder{
		parent: b,
		role:   RoleUser,
		parts:  make([]ContentPart, 0),
	}
}

// Text adds a text content part to the message.
func (m *MessageBuilder) Text(s string) *MessageBuilder {
	m.parts = append(m.parts, &InputText{Text: s})
	return m
}

// ImageURL adds an image by URL (HTTPS or data URL).
func (m *MessageBuilder) ImageURL(url string) *MessageBuilder {
	m.parts = append(m.parts, &InputImage{ImageURL: url})
	return m
}

// ImageURLWithDetail adds an image by URL with a specific detail level.
func (m *MessageBuilder) ImageURLWithDetail(url string, detail ImageDetail) *MessageBuilder {
	m.parts = append(m.parts, &InputImage{ImageURL: url, Detail: detail})
	return m
}

// ImageFileID adds an image by file ID from the Files API.
func (m *MessageBuilder) ImageFileID(fileID string) *MessageBuilder {
	m.parts = append(m.parts, &InputImage{FileID: fileID})
	return m
}

// ImageFileIDWithDetail adds an image by file ID with a specific detail level.
func (m *MessageBuilder) ImageFileIDWithDetail(fileID string, detail ImageDetail) *MessageBuilder {
	m.parts = append(m.parts, &InputImage{FileID: fileID, Detail: detail})
	return m
}

// FileURL adds a file by URL.
func (m *MessageBuilder) FileURL(url string) *MessageBuilder {
	m.parts = append(m.parts, &InputFile{FileURL: url})
	return m
}

// FileID adds a file by file ID from the Files API.
func (m *MessageBuilder) FileID(fileID string) *MessageBuilder {
	m.parts = append(m.parts, &InputFile{FileID: fileID})
	return m
}

// FileBase64 adds a file with base64-encoded content.
func (m *MessageBuilder) FileBase64(filename, base64Data string) *MessageBuilder {
	m.parts = append(m.parts, &InputFile{
		Filename: filename,
		FileData: base64Data,
	})
	return m
}

// Done completes the message and returns to the ChatBuilder.
func (m *MessageBuilder) Done() *ChatBuilder {
	m.parent.req.Messages = append(m.parent.req.Messages, Message{
		Role:  m.role,
		Parts: m.parts,
	})
	return m.parent
}

// UserWithImageURL adds a user message with text and an image URL.
// This is a convenience method for common vision use cases.
func (b *ChatBuilder) UserWithImageURL(text, imageURL string) *ChatBuilder {
	return b.UserMultimodal().
		Text(text).
		ImageURL(imageURL).
		Done()
}

// UserWithImageFileID adds a user message with text and an image file ID.
// This is a convenience method for vision use cases with uploaded files.
func (b *ChatBuilder) UserWithImageFileID(text, fileID string) *ChatBuilder {
	return b.UserMultimodal().
		Text(text).
		ImageFileID(fileID).
		Done()
}

// UserWithFileURL adds a user message with text and a file URL.
// This is a convenience method for document analysis use cases.
func (b *ChatBuilder) UserWithFileURL(text, fileURL string) *ChatBuilder {
	return b.UserMultimodal().
		Text(text).
		FileURL(fileURL).
		Done()
}

// UserWithFileID adds a user message with text and a file ID.
// This is a convenience method for document analysis with uploaded files.
func (b *ChatBuilder) UserWithFileID(text, fileID string) *ChatBuilder {
	return b.UserMultimodal().
		Text(text).
		FileID(fileID).
		Done()
}

// wrapStreamWithTelemetry wraps a ChatStream to emit telemetry on completion.
// onDone, if non-nil, is invoked exactly once when the stream completes
// (successfully or with an error) — used by Stream to release an
// Iris-applied timeout context without truncating an in-flight stream.
// mapErr, if non-nil, remaps errors forwarded on the returned Err channel
// (e.g. a mid-stream Iris-applied deadline into ErrTimeout).
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
	finalCh := make(chan *ChatResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(finalCh)
		defer close(errCh)
		if onDone != nil {
			defer onDone()
		}

		var finalResp *ChatResponse
		var finalErr error

		// Wait for final response, checking for errors along the way
		// Use a loop to avoid the race where select randomly picks Err
		// over Final when both channels are ready
		for finalResp == nil && finalErr == nil {
			select {
			case resp, ok := <-stream.Final:
				if ok {
					finalResp = resp
					finalCh <- resp
				} else {
					// Final channel closed without value, check for error
					select {
					case err, ok := <-stream.Err:
						if ok {
							out := err
							if mapErr != nil {
								out = mapErr(err)
							}
							finalErr = out
							errCh <- out
						}
					default:
					}
					goto done
				}
			case err, ok := <-stream.Err:
				if ok {
					out := err
					if mapErr != nil {
						out = mapErr(err)
					}
					finalErr = out
					errCh <- out
				}
				// If Err closed without value, continue to wait for Final
			}
		}
	done:

		// Emit telemetry end
		usage := TokenUsage{}
		if finalResp != nil {
			usage = finalResp.Usage
		}
		endEvent := RequestEndEvent{
			Provider: provider,
			Model:    model,
			Start:    start,
			End:      time.Now(),
			Usage:    usage,
			Err:      finalErr,
		}

		if ctxHook, ok := hook.(ContextualTelemetryHook); ok {
			ctxHook.OnRequestEndWithContext(ctx, endEvent)
		} else {
			hook.OnRequestEnd(endEvent)
		}
	}()

	return &ChatStream{
		Ch:    stream.Ch,
		Err:   errCh,
		Final: finalCh,
	}
}

func (c *Client) warnf(format string, args ...any) {
	c.warningHandler(fmt.Sprintf(format, args...))
}

func (b *ChatBuilder) warnUnsupportedContentParts() {
	supporter, declaresSupport := AsContentPartSupporter(b.client.provider)
	for messageIndex, message := range b.req.Messages {
		for partIndex, part := range message.Parts {
			if declaresSupport && supporter.SupportsContentPart(b.req.Model, message.Role, part) {
				continue
			}
			b.client.warnf(
				"provider %s model %s does not declare support for content part %s at message %d part %d; part will be omitted",
				b.client.provider.ID(), b.req.Model, contentPartType(part), messageIndex, partIndex,
			)
		}
	}
}

func contentPartType(part ContentPart) string {
	switch part.(type) {
	case InputText, *InputText:
		return "input_text"
	case InputImage, *InputImage:
		return "input_image"
	case InputFile, *InputFile:
		return "input_file"
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%T", part)
	}
}
