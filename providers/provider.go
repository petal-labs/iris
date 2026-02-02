// Package providers contains LLM provider implementations for Iris.
//
// Each provider is implemented in its own subpackage (e.g., providers/openai,
// providers/anthropic). Providers implement the core.Provider interface.
//
// # Provider Interface
//
// All providers must implement core.Provider:
//
//	type Provider interface {
//	    ID() string
//	    Models() []ModelInfo
//	    Supports(feature Feature) bool
//	    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
//	    StreamChat(ctx context.Context, req *ChatRequest) (*ChatStream, error)
//	}
//
// # Concurrency
//
// Providers SHOULD be safe for concurrent calls. If a provider cannot be
// concurrent-safe, it MUST document this limitation.
//
// # Streaming
//
// StreamChat returns a *ChatStream (not a raw channel) to carry errors and
// final tool calls consistently. Providers MUST:
//   - Close all channels (Ch, Err, Final) when finished
//   - Terminate promptly on context cancellation
//   - Send at most one error on Err
//   - Send exactly one response on Final (or zero on setup failure)
//
// # Feature Detection
//
// Use Supports() to check provider capabilities before making requests:
//
//	if p.Supports(core.FeatureToolCalling) {
//	    // Safe to use tools
//	}
package providers

import "github.com/petal-labs/iris/core"

// Re-export core types for convenience.
// Provider implementations can import just the providers package.
type (
	// Provider is the interface that LLM providers must implement.
	Provider = core.Provider

	// Feature represents a capability that a provider may support.
	Feature = core.Feature

	// ModelInfo describes a model available from a provider.
	ModelInfo = core.ModelInfo

	// ModelID is a string identifier for a model.
	ModelID = core.ModelID

	// ChatRequest represents a request to a chat model.
	ChatRequest = core.ChatRequest

	// ChatResponse represents a response from a chat model.
	ChatResponse = core.ChatResponse

	// ChatStream represents a streaming response from a provider.
	ChatStream = core.ChatStream

	// ChatChunk represents an incremental streaming response.
	ChatChunk = core.ChatChunk

	// Message represents a single message in a conversation.
	Message = core.Message

	// Role represents a message participant role.
	Role = core.Role

	// TokenUsage tracks token consumption for a request.
	TokenUsage = core.TokenUsage

	// ToolCall represents a tool invocation requested by the model.
	ToolCall = core.ToolCall

	// ProviderError represents an error returned by a provider.
	ProviderError = core.ProviderError
)

// Re-export feature constants.
const (
	FeatureChat          = core.FeatureChat
	FeatureChatStreaming = core.FeatureChatStreaming
	FeatureToolCalling   = core.FeatureToolCalling
)

// Re-export role constants.
const (
	RoleSystem    = core.RoleSystem
	RoleUser      = core.RoleUser
	RoleAssistant = core.RoleAssistant
)

// Re-export sentinel errors.
var (
	ErrUnauthorized  = core.ErrUnauthorized
	ErrRateLimited   = core.ErrRateLimited
	ErrBadRequest    = core.ErrBadRequest
	ErrServer        = core.ErrServer
	ErrNetwork       = core.ErrNetwork
	ErrDecode        = core.ErrDecode
	ErrModelRequired = core.ErrModelRequired
	ErrNoMessages    = core.ErrNoMessages
)
