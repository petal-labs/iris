// Package core provides the Iris SDK client and types for interacting with AI providers.
//
// Iris is a Go-native framework for building chat and multimodal workflows.
// The core package defines the fundamental abstractions
// that all providers implement.
//
// # Client and Provider
//
// The primary entry point is [Client], which wraps a [Provider] and adds telemetry,
// retry logic, and a fluent builder API:
//
//	provider := openai.New(os.Getenv("OPENAI_API_KEY"))
//	client := core.NewClient(provider,
//	    core.WithTelemetry(myTelemetryHook),
//	    core.WithRetryPolicy(core.DefaultRetryPolicy()),
//	)
//
// # ChatBuilder
//
// The [ChatBuilder] provides a fluent API for constructing chat requests:
//
//	resp, err := client.Chat("gpt-4o").
//	    System("You are a helpful assistant.").
//	    User("Hello!").
//	    Temperature(0.7).
//	    GetResponse(ctx)
//
// ChatBuilder is NOT thread-safe. Each goroutine should create its own builder
// instance. Use [ChatBuilder.Clone] to create independent copies from a base
// configuration:
//
//	base := client.Chat(model).System("You are helpful.").Temperature(0.7)
//	go func() { resp1, _ := base.Clone().User("Q1").GetResponse(ctx) }()
//	go func() { resp2, _ := base.Clone().User("Q2").GetResponse(ctx) }()
//
// # Streaming
//
// Iris treats streaming as a first-class primitive. Use [ChatBuilder.Stream] for
// streaming responses:
//
//	stream, err := client.Chat(model).User("Tell me a story.").Stream(ctx)
//	if err != nil {
//	    return err
//	}
//	for chunk := range stream.Ch {
//	    fmt.Print(chunk.Delta)
//	}
//
// The [ChatStream] type provides three channels:
//   - Ch: Emits text deltas in order
//   - Err: Emits at most one error
//   - Final: Emits the complete response with usage and tool calls
//
// Use [DrainStream] as a convenience to accumulate all chunks into a final response.
//
// # Provider Interface
//
// All providers implement the [Provider] interface:
//
//	type Provider interface {
//	    ID() string
//	    Models() []ModelInfo
//	    Supports(feature Feature) bool
//	    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
//	    StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error)
//	}
//
// Providers SHOULD be safe for concurrent use. Use [Provider.Supports] to check
// capabilities before making requests:
//
//	if provider.Supports(core.FeatureToolCalling) {
//	    // Safe to use tools
//	}
//
// # Features
//
// Providers declare their capabilities through [Feature] constants:
//   - [FeatureChat]: Basic chat completion
//   - [FeatureChatStreaming]: Streaming chat completion
//   - [FeatureToolCalling]: Function/tool calling support
//   - [FeatureReasoning]: Extended reasoning capabilities
//   - [FeatureBuiltInTools]: Web search, file search, code interpreter
//   - [FeatureResponseChain]: Multi-turn response chaining
//   - [FeatureEmbeddings]: Text embedding generation
//   - [FeatureContextualizedEmbeddings]: Document-aware embeddings
//   - [FeatureReranking]: Search result reranking
//
// # Error Handling
//
// The package defines sentinel errors for common failure modes:
//   - [ErrUnauthorized]: Invalid or missing API key
//   - [ErrRateLimited]: Provider rate limit exceeded
//   - [ErrBadRequest]: Invalid request parameters
//   - [ErrServer]: Provider server error (5xx)
//   - [ErrNetwork]: Network connectivity issues
//   - [ErrDecode]: Response parsing failed
//   - [ErrModelRequired]: Model ID not specified
//   - [ErrNoMessages]: No messages in request
//
// Use errors.Is to check error types:
//
//	if errors.Is(err, core.ErrRateLimited) {
//	    // Handle rate limiting
//	}
//
// # Telemetry
//
// Implement [TelemetryHook] to observe request lifecycle:
//
//	type MyTelemetry struct{}
//
//	func (t MyTelemetry) OnRequestStart(e RequestStartEvent) {
//	    log.Printf("Starting %s request to %s", e.Model, e.Provider)
//	}
//
//	func (t MyTelemetry) OnRequestEnd(e RequestEndEvent) {
//	    log.Printf("Completed in %v, tokens: %d", e.End.Sub(e.Start), e.Usage.TotalTokens)
//	}
//
// # Retry Policy
//
// Configure retry behavior with [RetryPolicy]:
//
//	policy := &core.ExponentialBackoff{
//	    MaxRetries:  3,
//	    BaseDelay:   time.Second,
//	    MaxDelay:    30 * time.Second,
//	}
//	client := core.NewClient(provider, core.WithRetryPolicy(policy))
//
// The default policy retries transient errors (rate limits, server errors) with
// exponential backoff.
//
// # Multimodal Messages
//
// For vision and document analysis, use [MessageBuilder]:
//
//	resp, err := client.Chat(model).
//	    UserMultimodal().
//	        Text("What's in this image?").
//	        ImageURL("https://example.com/image.jpg").
//	        Done().
//	    GetResponse(ctx)
//
// Convenience methods are also available:
//
//	resp, err := client.Chat(model).
//	    UserWithImageURL("Describe this:", "https://example.com/image.jpg").
//	    GetResponse(ctx)
//
// # Thread Safety
//
// [Client] is safe for concurrent use across goroutines.
// [ChatBuilder] and [MessageBuilder] are NOT thread-safe.
// [ChatStream] channels may be read by one goroutine at a time.
// Providers SHOULD be safe for concurrent calls (check provider documentation).
package core
