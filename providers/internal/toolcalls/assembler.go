// Package toolcalls provides shared streaming tool-call assembly utilities.
package toolcalls

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/petal-labs/iris/core"
)

// ErrInvalidJSON is returned when assembled tool arguments are not valid JSON.
var ErrInvalidJSON = errors.New("tool args invalid json")

// Config controls assembler behavior.
type Config struct {
	// EmptyArgumentsJSON, when set, is used as arguments when a tool call has no
	// accumulated argument fragments.
	EmptyArgumentsJSON string
}

// Fragment represents one streaming tool-call delta fragment.
type Fragment struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

type assemblingCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

// Assembler accumulates fragmented tool calls and emits canonical tool calls.
type Assembler struct {
	calls map[int]*assemblingCall
	cfg   Config
}

// NewAssembler creates a tool-call assembler.
func NewAssembler(cfg Config) *Assembler {
	return &Assembler{
		calls: make(map[int]*assemblingCall),
		cfg:   cfg,
	}
}

// AddFragment applies a streaming fragment, creating a call entry if needed.
func (a *Assembler) AddFragment(f Fragment) {
	call, exists := a.calls[f.Index]
	if !exists {
		call = &assemblingCall{}
		a.calls[f.Index] = call
	}

	if f.ID != "" {
		call.ID = f.ID
	}
	if f.Name != "" {
		call.Name = f.Name
	}
	if f.Arguments != "" {
		call.Arguments.WriteString(f.Arguments)
	}
}

// StartCall initializes a tool call by index.
func (a *Assembler) StartCall(index int, id, name string) {
	a.calls[index] = &assemblingCall{
		ID:   id,
		Name: name,
	}
}

// AddArguments appends argument fragments for an existing call.
// If the call index has not been initialized, this is a no-op.
func (a *Assembler) AddArguments(index int, chunk string) {
	call, exists := a.calls[index]
	if !exists || chunk == "" {
		return
	}
	call.Arguments.WriteString(chunk)
}

// Finalize validates and returns assembled tool calls in index order.
func (a *Assembler) Finalize() ([]core.ToolCall, error) {
	if len(a.calls) == 0 {
		return nil, nil
	}

	maxIndex := 0
	for idx := range a.calls {
		if idx > maxIndex {
			maxIndex = idx
		}
	}

	result := make([]core.ToolCall, 0, len(a.calls))
	for i := 0; i <= maxIndex; i++ {
		call, exists := a.calls[i]
		if !exists {
			continue
		}

		args := call.Arguments.String()
		if args == "" && a.cfg.EmptyArgumentsJSON != "" {
			args = a.cfg.EmptyArgumentsJSON
		}
		if !json.Valid([]byte(args)) {
			return nil, ErrInvalidJSON
		}

		result = append(result, core.ToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: json.RawMessage(args),
		})
	}

	return result, nil
}
