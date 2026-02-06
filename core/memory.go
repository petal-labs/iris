package core

import (
	"context"
	"sync"
)

// Memory is the interface for managing conversation history.
// Implementations provide different storage backends (in-memory, Redis, PostgreSQL, etc.).
type Memory interface {
	// AddMessage appends a message to the conversation history.
	AddMessage(msg Message)

	// AddMessages appends multiple messages to the conversation history.
	AddMessages(msgs []Message)

	// GetHistory returns all messages in the conversation.
	GetHistory() []Message

	// GetLastN returns the last N messages in the conversation.
	GetLastN(n int) []Message

	// Clear removes all messages from the conversation.
	Clear()

	// Len returns the number of messages in the conversation.
	Len() int

	// SetMessages replaces the entire conversation history.
	SetMessages(msgs []Message)
}

// InMemoryStore is a thread-safe in-memory implementation of the Memory interface.
// Suitable for single-session conversations that don't require persistence.
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
func (m *InMemoryStore) AddMessage(msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

// AddMessages appends multiple messages to the conversation history.
func (m *InMemoryStore) AddMessages(msgs []Message) {
	if len(msgs) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs...)
}

// GetHistory returns all messages in the conversation.
// Returns a copy of the messages slice.
func (m *InMemoryStore) GetHistory() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// GetLastN returns the last N messages in the conversation.
// If N is greater than the number of messages, returns all messages.
func (m *InMemoryStore) GetLastN(n int) []Message {
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
func (m *InMemoryStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]Message, 0)
}

// Len returns the number of messages in the conversation.
func (m *InMemoryStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// SetMessages replaces the entire conversation history.
func (m *InMemoryStore) SetMessages(msgs []Message) {
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
func NewConversation(client *Client, model ModelID, opts ...ConversationOption) *Conversation {
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
		c.memory.AddMessage(Message{
			Role:    RoleSystem,
			Content: c.system,
		})
	}

	return c
}

// Send sends a user message and returns the assistant's response.
// Automatically manages conversation history.
// Uses context.Background() internally.
func (c *Conversation) Send(userMessage string) (*ChatResponse, error) {
	return c.SendWithContext(context.Background(), userMessage)
}

// SendWithContext sends a user message with context and returns the assistant's response.
func (c *Conversation) SendWithContext(ctx context.Context, userMessage string) (*ChatResponse, error) {
	// Add user message to history
	c.memory.AddMessage(Message{
		Role:    RoleUser,
		Content: userMessage,
	})

	// Build request with full history
	builder := c.client.Chat(c.model)
	for _, msg := range c.memory.GetHistory() {
		switch msg.Role {
		case RoleSystem:
			builder = builder.System(msg.Content)
		case RoleUser:
			builder = builder.User(msg.Content)
		case RoleAssistant:
			builder = builder.Assistant(msg.Content)
		}
	}

	// Get response
	resp, err := builder.GetResponse(ctx)
	if err != nil {
		return nil, err
	}

	// Add assistant response to history
	c.memory.AddMessage(Message{
		Role:    RoleAssistant,
		Content: resp.Output,
	})

	return resp, nil
}

// GetHistory returns the full conversation history.
func (c *Conversation) GetHistory() []Message {
	return c.memory.GetHistory()
}

// Clear resets the conversation history.
// If a system message was provided, it will be re-added.
func (c *Conversation) Clear() {
	c.memory.Clear()
	if c.system != "" {
		c.memory.AddMessage(Message{
			Role:    RoleSystem,
			Content: c.system,
		})
	}
}

// MessageCount returns the number of messages in the conversation.
func (c *Conversation) MessageCount() int {
	return c.memory.Len()
}
