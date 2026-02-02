package tools

import (
	"errors"
	"sync"
)

// ErrDuplicateTool is returned when attempting to register a tool with a name
// that is already registered.
var ErrDuplicateTool = errors.New("tool already registered")

// Registry manages a collection of tools indexed by name.
// Registry is safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
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

	r.tools[name] = t
	return nil
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
