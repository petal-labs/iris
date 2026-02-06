package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ErrDuplicateTool is returned when attempting to register a tool with a name
// that is already registered.
var ErrDuplicateTool = errors.New("tool already registered")

// RegistryOption configures a Registry.
type RegistryOption func(*Registry)

// WithRegistryMiddleware applies middleware to all tools registered in the registry.
// Middleware are applied in the order provided when tools are registered.
func WithRegistryMiddleware(middlewares ...Middleware) RegistryOption {
	return func(r *Registry) {
		r.middlewares = append(r.middlewares, middlewares...)
	}
}

// Registry manages a collection of tools indexed by name.
// Registry is safe for concurrent use.
type Registry struct {
	mu          sync.RWMutex
	tools       map[string]Tool
	middlewares []Middleware
}

// NewRegistry creates a new tool registry with optional configuration.
func NewRegistry(opts ...RegistryOption) *Registry {
	r := &Registry{
		tools:       make(map[string]Tool),
		middlewares: nil,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Register adds a tool to the registry.
// If registry middleware is configured, it's automatically applied.
// Returns ErrDuplicateTool if a tool with the same name is already registered.
func (r *Registry) Register(t Tool) error {
	if t == nil {
		return errors.New("tool cannot be nil")
	}

	name := t.Name()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return ErrDuplicateTool
	}

	// Apply registry-level middleware
	if len(r.middlewares) > 0 {
		t = ApplyMiddleware(t, r.middlewares...)
	}

	r.tools[name] = t
	return nil
}

// RegisterWithMiddleware adds a tool with additional per-tool middleware.
// Per-tool middleware executes inside registry middleware.
func (r *Registry) RegisterWithMiddleware(t Tool, middlewares ...Middleware) error {
	// Apply per-tool middleware first, then registry middleware wraps it
	if len(middlewares) > 0 {
		t = ApplyMiddleware(t, middlewares...)
	}
	return r.Register(t)
}

// Get retrieves a tool by name.
// Returns the tool and true if found, or nil and false if not found.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
// The returned slice is a copy and safe to modify.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Execute finds a tool by name and calls it with the given arguments.
// Returns an error if the tool is not found or if execution fails.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (any, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return tool.Call(ctx, args)
}
