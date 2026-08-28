package core

import "context"

// TokenCountResponse contains the number of input tokens a provider would
// process for a chat request.
type TokenCountResponse struct {
	InputTokens int `json:"input_tokens"`
}

// TokenCounter is an optional interface for providers with a native token
// counting endpoint. The request should be the same ChatRequest that will be
// sent for generation so system messages, multimodal parts, and tools can be
// included in the provider's count.
type TokenCounter interface {
	CountTokens(ctx context.Context, req *ChatRequest) (*TokenCountResponse, error)
}

// AsTokenCounter attempts to cast a Provider to TokenCounter. It returns nil
// and false when the provider does not implement native token counting.
func AsTokenCounter(p Provider) (TokenCounter, bool) {
	counter, ok := p.(TokenCounter)
	return counter, ok
}
