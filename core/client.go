package core

import (
	"context"
	"fmt"
	"time"
)

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

// Client is the main entry point for interacting with LLM providers.
// Client is safe for concurrent use.
type Client struct {
	provider       Provider
	telemetry      TelemetryHook
	retry          RetryPolicy
	warningHandler WarningHandler
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

// WithWarningHandler sets a handler for non-fatal SDK warnings.
// Pass nil to keep the default no-op handler.
func WithWarningHandler(h WarningHandler) ClientOption {
	return func(c *Client) {
		if h != nil {
			c.warningHandler = h
		}
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

// Tools sets the tools available for the request.
func (b *ChatBuilder) Tools(ts ...Tool) *ChatBuilder {
	b.req.Tools = ts
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

	return clone
}

// Truncation sets the truncation mode for the request.
func (b *ChatBuilder) Truncation(mode string) *ChatBuilder {
	b.req.Truncation = mode
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

	return nil
}

// GetResponse executes the chat request and returns the response.
// It applies validation, telemetry, and retry logic.
// If Timeout was set and ctx has no deadline, a timeout context is created internally.
func (b *ChatBuilder) GetResponse(ctx context.Context) (*ChatResponse, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	// Apply timeout if set and context has no deadline
	if b.timeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, b.timeout)
			defer cancel()
		}
	}

	start := time.Now()
	providerID := b.client.provider.ID()

	// Emit telemetry start
	b.client.telemetry.OnRequestStart(RequestStartEvent{
		Provider: providerID,
		Model:    b.req.Model,
		Start:    start,
	})

	var resp *ChatResponse
	var err error

	// Execute with retry logic
retryLoop:
	for attempt := 0; ; attempt++ {
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

	// Emit telemetry end
	end := time.Now()
	usage := TokenUsage{}
	if resp != nil {
		usage = resp.Usage
	}
	b.client.telemetry.OnRequestEnd(RequestEndEvent{
		Provider: providerID,
		Model:    b.req.Model,
		Start:    start,
		End:      end,
		Usage:    usage,
		Err:      err,
	})

	return resp, err
}

// Stream executes the chat request and returns a streaming response.
// It applies validation and telemetry.
//
// Note: The Timeout() setting is NOT applied to streaming requests because
// the context must outlive this method call. For streaming with timeouts,
// create the context externally:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	defer cancel()
//	stream, err := client.Chat(model).User("...").Stream(ctx)
func (b *ChatBuilder) Stream(ctx context.Context) (*ChatStream, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	start := time.Now()
	providerID := b.client.provider.ID()

	// Emit telemetry start
	b.client.telemetry.OnRequestStart(RequestStartEvent{
		Provider: providerID,
		Model:    b.req.Model,
		Start:    start,
	})

	stream, err := b.client.provider.StreamChat(ctx, &b.req)
	if err != nil {
		// Emit telemetry end on immediate error
		b.client.telemetry.OnRequestEnd(RequestEndEvent{
			Provider: providerID,
			Model:    b.req.Model,
			Start:    start,
			End:      time.Now(),
			Err:      err,
		})
		return nil, err
	}

	// Wrap the stream to emit telemetry when it completes
	return wrapStreamWithTelemetry(stream, b.client.telemetry, providerID, b.req.Model, start), nil
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
func wrapStreamWithTelemetry(
	stream *ChatStream,
	hook TelemetryHook,
	provider string,
	model ModelID,
	start time.Time,
) *ChatStream {
	finalCh := make(chan *ChatResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(finalCh)
		defer close(errCh)

		var finalResp *ChatResponse
		var finalErr error

		// Wait for either final response or error
		select {
		case resp, ok := <-stream.Final:
			if ok {
				finalResp = resp
				finalCh <- resp
			}
		case err, ok := <-stream.Err:
			if ok {
				finalErr = err
				errCh <- err
			}
		}

		// Emit telemetry end
		usage := TokenUsage{}
		if finalResp != nil {
			usage = finalResp.Usage
		}
		hook.OnRequestEnd(RequestEndEvent{
			Provider: provider,
			Model:    model,
			Start:    start,
			End:      time.Now(),
			Usage:    usage,
			Err:      finalErr,
		})
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
