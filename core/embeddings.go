package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// EncodingFormat specifies the embedding output format.
type EncodingFormat string

const (
	// EncodingFormatFloat returns embeddings as float arrays.
	EncodingFormatFloat EncodingFormat = "float"
	// EncodingFormatBase64 returns embeddings as base64-encoded strings.
	EncodingFormatBase64 EncodingFormat = "base64"
)

// InputType specifies the type of input for retrieval optimization.
type InputType string

const (
	// InputTypeNone uses default embedding without retrieval optimization.
	InputTypeNone InputType = ""
	// InputTypeQuery optimizes embeddings for search queries.
	InputTypeQuery InputType = "query"
	// InputTypeDocument optimizes embeddings for documents being searched.
	InputTypeDocument InputType = "document"
)

// OutputDType specifies the data type for embedding vectors.
type OutputDType string

const (
	// OutputDTypeFloat returns 32-bit floating point numbers (default).
	OutputDTypeFloat OutputDType = "float"
	// OutputDTypeInt8 returns 8-bit signed integers (-128 to 127).
	OutputDTypeInt8 OutputDType = "int8"
	// OutputDTypeUint8 returns 8-bit unsigned integers (0 to 255).
	OutputDTypeUint8 OutputDType = "uint8"
	// OutputDTypeBinary returns bit-packed signed integers.
	OutputDTypeBinary OutputDType = "binary"
	// OutputDTypeUbinary returns bit-packed unsigned integers.
	OutputDTypeUbinary OutputDType = "ubinary"
)

// EmbeddingInput represents a single text to embed with optional metadata.
type EmbeddingInput struct {
	Text     string            `json:"text"`
	ID       string            `json:"id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// EmbeddingRequest represents a request to generate embeddings.
type EmbeddingRequest struct {
	Model          ModelID          `json:"model"`
	Input          []EmbeddingInput `json:"input"`
	EncodingFormat EncodingFormat   `json:"encoding_format,omitempty"`
	Dimensions     *int             `json:"dimensions,omitempty"`
	User           string           `json:"user,omitempty"`
	InputType      InputType        `json:"input_type,omitempty"`
	OutputDType    OutputDType      `json:"output_dtype,omitempty"`
	Truncation     *bool            `json:"truncation,omitempty"`
}

// EmbeddingVector represents a single embedding result.
type EmbeddingVector struct {
	Index     int               `json:"index"`
	ID        string            `json:"id,omitempty"`
	Vector    []float32         `json:"vector,omitempty"`
	VectorB64 string            `json:"vector_b64,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// EmbeddingUsage tracks token consumption for embeddings.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// EmbeddingResponse contains the generated embeddings.
type EmbeddingResponse struct {
	Vectors []EmbeddingVector `json:"vectors"`
	Model   ModelID           `json:"model"`
	Usage   EmbeddingUsage    `json:"usage"`
}

// EmbeddingProvider is an optional interface for providers that support embeddings.
type EmbeddingProvider interface {
	// CreateEmbeddings generates embeddings for the given input texts.
	CreateEmbeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
}

// AsEmbeddingProvider attempts to cast a Provider to EmbeddingProvider.
// It returns nil and false when the provider does not implement embeddings.
func AsEmbeddingProvider(p Provider) (EmbeddingProvider, bool) {
	embedder, ok := p.(EmbeddingProvider)
	return embedder, ok
}

// Embed generates embeddings through the client's provider. It applies the
// client timeout, telemetry, and retry policy to the optional embedding
// capability just as GetResponse does for chat requests.
func (c *Client) Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: embedding context cannot be nil")
	}
	if req == nil {
		return nil, fmt.Errorf("iris: embedding request cannot be nil")
	}
	embedder, ok := AsEmbeddingProvider(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support embeddings: %w", c.provider.ID(), ErrNotSupported)
	}

	var timeout time.Duration
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
		timeout = c.timeout
	}

	start := time.Now()
	providerID := c.provider.ID()
	ctx = c.startEmbeddingTelemetry(ctx, providerID, req.Model, start)
	resp, err := c.createEmbeddingsWithRetry(ctx, embedder, req)
	if timeout > 0 && err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = newTimeoutError(timeout, providerID, req.Model)
	}
	c.endEmbeddingTelemetry(ctx, providerID, req.Model, start, resp, err)
	return resp, err
}

func (c *Client) createEmbeddingsWithRetry(ctx context.Context, provider EmbeddingProvider, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	for attempt := 0; ; attempt++ {
		resp, err := provider.CreateEmbeddings(ctx, req)
		if err == nil {
			return resp, nil
		}
		delay, retry := c.retry.NextDelay(attempt, err)
		if !retry {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (c *Client) startEmbeddingTelemetry(ctx context.Context, provider string, model ModelID, start time.Time) context.Context {
	event := RequestStartEvent{Provider: provider, Model: model, Start: start}
	if hook, ok := c.telemetry.(ContextualTelemetryHook); ok {
		return hook.OnRequestStartWithContext(ctx, event)
	}
	c.telemetry.OnRequestStart(event)
	return ctx
}

func (c *Client) endEmbeddingTelemetry(ctx context.Context, provider string, model ModelID, start time.Time, resp *EmbeddingResponse, err error) {
	usage := TokenUsage{}
	if resp != nil {
		usage.PromptTokens = resp.Usage.PromptTokens
		usage.TotalTokens = resp.Usage.TotalTokens
	}
	event := RequestEndEvent{
		Provider: provider,
		Model:    model,
		Start:    start,
		End:      time.Now(),
		Usage:    usage,
		Err:      err,
	}
	if hook, ok := c.telemetry.(ContextualTelemetryHook); ok {
		hook.OnRequestEndWithContext(ctx, event)
		return
	}
	c.telemetry.OnRequestEnd(event)
}
