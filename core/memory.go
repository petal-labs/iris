package core

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// Memory is the interface for managing conversation history.
// Implementations provide different storage backends (in-memory, Redis,
// PostgreSQL, etc.). Every operation receives the caller's context so remote
// stores can honor cancellation and deadlines and propagate trace metadata.
type Memory interface {
	// AddMessage appends a message to the conversation history.
	AddMessage(ctx context.Context, msg Message)

	// AddMessages appends multiple messages to the conversation history.
	AddMessages(ctx context.Context, msgs []Message)

	// GetHistory returns all messages in the conversation.
	GetHistory(ctx context.Context) []Message

	// GetLastN returns the last N messages in the conversation.
	GetLastN(ctx context.Context, n int) []Message

	// Clear removes all messages from the conversation.
	Clear(ctx context.Context)

	// Len returns the number of messages in the conversation.
	Len(ctx context.Context) int

	// SetMessages replaces the entire conversation history.
	SetMessages(ctx context.Context, msgs []Message)
}

// InMemoryStore is a thread-safe in-memory implementation of the Memory
// interface. Operations complete synchronously, so their contexts are unused.
// It is suitable for single-session conversations that don't require
// persistence.
type InMemoryStore struct {
	mu       sync.RWMutex
	messages []Message
}

// NewInMemoryStore creates a new in-memory conversation store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make([]Message, 0),
	}
}

// AddMessage appends a message to the conversation history.
func (m *InMemoryStore) AddMessage(_ context.Context, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

// AddMessages appends multiple messages to the conversation history.
func (m *InMemoryStore) AddMessages(_ context.Context, msgs []Message) {
	if len(msgs) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs...)
}

// GetHistory returns all messages in the conversation.
// Returns a copy of the messages slice.
func (m *InMemoryStore) GetHistory(_ context.Context) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// GetLastN returns the last N messages in the conversation.
// If N is greater than the number of messages, returns all messages.
func (m *InMemoryStore) GetLastN(_ context.Context, n int) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n <= 0 {
		return nil
	}

	if n >= len(m.messages) {
		result := make([]Message, len(m.messages))
		copy(result, m.messages)
		return result
	}

	start := len(m.messages) - n
	result := make([]Message, n)
	copy(result, m.messages[start:])
	return result
}

// Clear removes all messages from the conversation.
func (m *InMemoryStore) Clear(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]Message, 0)
}

// Len returns the number of messages in the conversation.
func (m *InMemoryStore) Len(_ context.Context) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// SetMessages replaces the entire conversation history.
func (m *InMemoryStore) SetMessages(_ context.Context, msgs []Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]Message, len(msgs))
	copy(m.messages, msgs)
}

// -----------------------------------------------------------------------------
// Conversation Session
// -----------------------------------------------------------------------------

// Conversation provides a high-level API for managing multi-turn conversations
// with automatic history management.
type Conversation struct {
	memory Memory
	client *Client
	model  ModelID
	system string // Optional system message
}

// ConversationOption configures a Conversation.
type ConversationOption func(*Conversation)

// WithSystemMessage sets a system message for the conversation.
func WithSystemMessage(system string) ConversationOption {
	return func(c *Conversation) {
		c.system = system
	}
}

// WithMemoryStore sets a custom memory store for the conversation.
func WithMemoryStore(memory Memory) ConversationOption {
	return func(c *Conversation) {
		c.memory = memory
	}
}

// NewConversation creates a new conversation session with the given client and model.
// ctx is passed to the configured Memory when initializing a system message.
func NewConversation(ctx context.Context, client *Client, model ModelID, opts ...ConversationOption) *Conversation {
	c := &Conversation{
		memory: NewInMemoryStore(),
		client: client,
		model:  model,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Add system message if provided
	if c.system != "" {
		c.memory.AddMessage(ctx, Message{
			Role:    RoleSystem,
			Content: c.system,
		})
	}

	return c
}

// Send sends a user message with ctx and returns the assistant's response.
// It automatically manages conversation history, preserving multimodal and
// tool-call turns when replaying earlier messages.
func (c *Conversation) Send(ctx context.Context, userMessage string) (*ChatResponse, error) {
	builder := c.builderWithUserMessage(ctx, userMessage)
	resp, err := builder.GetResponse(ctx)
	if err != nil {
		return nil, err
	}

	c.memory.AddMessage(ctx, Message{
		Role:      RoleAssistant,
		Content:   resp.Output,
		ToolCalls: slices.Clone(resp.ToolCalls),
	})

	return resp, nil
}

// SendWithContext delegates to Send.
//
// Deprecated: use Send.
func (c *Conversation) SendWithContext(ctx context.Context, userMessage string) (*ChatResponse, error) {
	return c.Send(ctx, userMessage)
}

func (c *Conversation) builderWithUserMessage(ctx context.Context, userMessage string) *ChatBuilder {
	c.memory.AddMessage(ctx, Message{Role: RoleUser, Content: userMessage})

	builder := c.client.Chat(c.model)
	builder.req.Messages = slices.Clone(c.memory.GetHistory(ctx))
	return builder
}

// AddToolResults appends tool execution results to the conversation. Call it
// after Send or Stream returns assistant tool calls and before sending the next
// user message. The results slice is copied before it is stored.
func (c *Conversation) AddToolResults(ctx context.Context, results []ToolResult) {
	if len(results) == 0 {
		return
	}

	c.memory.AddMessage(ctx, Message{
		Role:        RoleTool,
		ToolResults: slices.Clone(results),
	})
}

// GetHistory returns the full conversation history.
func (c *Conversation) GetHistory(ctx context.Context) []Message {
	return c.memory.GetHistory(ctx)
}

// Clear resets the conversation history.
// If a system message was provided, it will be re-added.
func (c *Conversation) Clear(ctx context.Context) {
	c.memory.Clear(ctx)
	if c.system != "" {
		c.memory.AddMessage(ctx, Message{
			Role:    RoleSystem,
			Content: c.system,
		})
	}
}

// MessageCount returns the number of messages in the conversation.
func (c *Conversation) MessageCount(ctx context.Context) int {
	return c.memory.Len(ctx)
}

// Stream sends a user message with ctx and returns a streaming response.
// The stream is wrapped to add the assistant's complete text and tool calls to
// conversation history when it finishes successfully.
func (c *Conversation) Stream(ctx context.Context, userMessage string) (*ChatStream, error) {
	builder := c.builderWithUserMessage(ctx, userMessage)
	stream, err := builder.Stream(ctx)
	if err != nil {
		return nil, err
	}

	return c.wrapStreamForHistory(ctx, stream), nil
}

// StreamWithContext delegates to Stream.
//
// Deprecated: use Stream.
func (c *Conversation) StreamWithContext(ctx context.Context, userMessage string) (*ChatStream, error) {
	return c.Stream(ctx, userMessage)
}

// wrapStreamForHistory wraps a ChatStream to capture the final response
// and add it to conversation history.
func (c *Conversation) wrapStreamForHistory(ctx context.Context, stream *ChatStream) *ChatStream {
	wrappedCh := make(chan ChatChunk, 1)
	wrappedFinal := make(chan *ChatResponse, 1)

	go c.forwardStreamToHistory(ctx, stream, wrappedCh, wrappedFinal)

	return &ChatStream{
		Ch:    wrappedCh,
		Err:   stream.Err,
		Final: wrappedFinal,
	}
}

func (c *Conversation) forwardStreamToHistory(
	ctx context.Context,
	stream *ChatStream,
	wrappedCh chan<- ChatChunk,
	wrappedFinal chan<- *ChatResponse,
) {
	var accumulated strings.Builder
	for chunk := range stream.Ch {
		accumulated.WriteString(chunk.Delta)
		wrappedCh <- chunk
	}
	close(wrappedCh)

	finalResp, ok := <-stream.Final
	if !ok || finalResp == nil {
		close(wrappedFinal)
		return
	}

	output := finalResp.Output
	if output == "" {
		output = accumulated.String()
	}
	if output != "" || len(finalResp.ToolCalls) > 0 {
		c.memory.AddMessage(ctx, Message{
			Role:      RoleAssistant,
			Content:   output,
			ToolCalls: slices.Clone(finalResp.ToolCalls),
		})
	}

	wrappedFinal <- finalResp
	close(wrappedFinal)
}
