package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToolExecutor is an interface for executing tools by name.
// This interface is implemented by tools.Registry.
type ToolExecutor interface {
	// Execute finds a tool by name and calls it with the given arguments.
	Execute(ctx context.Context, name string, args json.RawMessage) (any, error)
}

// AgentConfig configures the behavior of an agent loop.
type AgentConfig struct {
	// MaxIterations is the maximum number of LLM calls before stopping.
	// Each iteration may execute multiple tools.
	// Default: 10. Set to 0 for unlimited (use with caution).
	MaxIterations int

	// MaxToolCalls is the maximum total tool calls across all iterations.
	// Default: 50. Set to 0 for unlimited.
	MaxToolCalls int

	// IterationTimeout is the timeout for each individual LLM call.
	// Does not include tool execution time.
	// Default: 30s.
	IterationTimeout time.Duration

	// ToolTimeout is the timeout for each individual tool execution.
	// Default: 60s.
	ToolTimeout time.Duration

	// ParallelTools enables concurrent execution of tools within an iteration.
	// When false, tools execute sequentially in order.
	// Default: true.
	ParallelTools bool

	// MaxParallelTools limits concurrent tool executions.
	// Only applies when ParallelTools is true.
	// Default: 5. Set to 0 for unlimited.
	MaxParallelTools int

	// ContinueOnToolError determines behavior when a tool fails.
	// If true, the error is passed to the model as a tool result.
	// If false, the agent loop returns immediately with the error.
	// Default: true (pass errors to model).
	ContinueOnToolError bool

	// StopSequences are additional strings that stop the agent.
	// If the model's output contains any of these, the loop terminates.
	// Default: empty.
	StopSequences []string

	// ToolFilter optionally filters which tools can be executed.
	// Return false to skip a tool (error sent to model).
	// Default: nil (all tools allowed).
	ToolFilter func(call ToolCall) bool

	// Hooks for observability and control.
	Hooks AgentHooks

	// Memory configures conversation memory management with auto-summarization.
	// If nil, no memory management is performed (may hit context limits).
	Memory *MemoryConfig
}

// DefaultAgentConfig returns a configuration with sensible defaults.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxIterations:       10,
		MaxToolCalls:        50,
		IterationTimeout:    30 * time.Second,
		ToolTimeout:         60 * time.Second,
		ParallelTools:       true,
		MaxParallelTools:    5,
		ContinueOnToolError: true,
		StopSequences:       nil,
		ToolFilter:          nil,
		Hooks:               AgentHooks{},
		Memory:              nil,
	}
}

// AgentHooks provides callbacks for observing and controlling agent execution.
type AgentHooks struct {
	// OnIterationStart is called at the start of each LLM call.
	// Return an error to abort the agent loop.
	OnIterationStart func(ctx context.Context, e IterationStartEvent) error

	// OnIterationEnd is called after each LLM response.
	OnIterationEnd func(ctx context.Context, e IterationEndEvent)

	// OnToolCallStart is called before executing each tool.
	// Return an error to skip this tool (error sent to model).
	OnToolCallStart func(ctx context.Context, e ToolCallStartEvent) error

	// OnToolCallEnd is called after each tool execution.
	OnToolCallEnd func(ctx context.Context, e ToolCallEndEvent)

	// OnAgentComplete is called when the agent loop finishes.
	OnAgentComplete func(ctx context.Context, e AgentCompleteEvent)

	// OnTextDelta is called for each text chunk during streaming.
	// Only used with RunStream.
	OnTextDelta func(ctx context.Context, delta string)
}

// Event types for hooks

// IterationStartEvent is emitted at the start of each LLM call.
type IterationStartEvent struct {
	Iteration    int
	MessageCount int
	ToolCount    int // Tools executed so far
}

// IterationEndEvent is emitted after each LLM response.
type IterationEndEvent struct {
	Iteration  int
	Response   *ChatResponse
	ToolCalls  []ToolCall
	Duration   time.Duration
	TokensUsed TokenUsage
}

// ToolCallStartEvent is emitted before executing each tool.
type ToolCallStartEvent struct {
	Iteration int
	ToolCall  ToolCall
	Index     int // Index within this iteration's tool calls
	Total     int // Total tool calls in this iteration
}

// ToolCallEndEvent is emitted after each tool execution.
type ToolCallEndEvent struct {
	Iteration int
	ToolCall  ToolCall
	Result    any
	Error     error
	Duration  time.Duration
}

// AgentCompleteEvent is emitted when the agent loop finishes.
type AgentCompleteEvent struct {
	Iterations     int
	TotalToolCalls int
	TotalDuration  time.Duration
	TotalTokens    TokenUsage
	StopReason     AgentStopReason
	FinalResponse  *ChatResponse
}

// AgentStopReason indicates why the agent loop terminated.
type AgentStopReason string

const (
	StopReasonComplete      AgentStopReason = "complete"       // Model finished (no tool calls)
	StopReasonMaxIterations AgentStopReason = "max_iterations" // Hit MaxIterations limit
	StopReasonMaxToolCalls  AgentStopReason = "max_tool_calls" // Hit MaxToolCalls limit
	StopReasonStopSequence  AgentStopReason = "stop_sequence"  // Output contained stop sequence
	StopReasonHookAbort     AgentStopReason = "hook_abort"     // Hook returned error
	StopReasonError         AgentStopReason = "error"          // Unrecoverable error
	StopReasonCanceled      AgentStopReason = "canceled"       // Context canceled
)

// AgentResult contains the complete result of an agent execution.
type AgentResult struct {
	// FinalResponse is the last response from the model.
	FinalResponse *ChatResponse

	// Iterations is the number of LLM calls made.
	Iterations int

	// ToolHistory contains all tool executions in order.
	ToolHistory []ToolExecution

	// TotalTokens is the sum of tokens across all iterations.
	TotalTokens TokenUsage

	// Duration is the total time from start to finish.
	Duration time.Duration

	// StopReason indicates why the agent stopped.
	StopReason AgentStopReason

	// Error is set if the agent stopped due to an error.
	// May be nil even if StopReason is StopReasonError.
	Error error
}

// ToolExecution records a single tool call and its result.
type ToolExecution struct {
	Iteration int
	Call      ToolCall
	Result    any
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

// HasError returns true if the agent encountered an error.
func (r *AgentResult) HasError() bool {
	return r.Error != nil || r.StopReason == StopReasonError
}

// TotalToolCalls returns the number of tool executions.
func (r *AgentResult) TotalToolCalls() int {
	return len(r.ToolHistory)
}

// SuccessfulToolCalls returns tool executions that succeeded.
func (r *AgentResult) SuccessfulToolCalls() []ToolExecution {
	var successful []ToolExecution
	for _, te := range r.ToolHistory {
		if te.Error == nil {
			successful = append(successful, te)
		}
	}
	return successful
}

// FailedToolCalls returns tool executions that failed.
func (r *AgentResult) FailedToolCalls() []ToolExecution {
	var failed []ToolExecution
	for _, te := range r.ToolHistory {
		if te.Error != nil {
			failed = append(failed, te)
		}
	}
	return failed
}

// MemoryConfig configures conversation memory management.
type MemoryConfig struct {
	// MaxTokens is the target maximum tokens for the conversation.
	// When exceeded, auto-summarization is triggered.
	// Default: 0 (no limit - use provider's context window)
	MaxTokens int

	// SummarizationThreshold triggers summarization when token count
	// exceeds MaxTokens * SummarizationThreshold.
	// Default: 0.8 (summarize when 80% full)
	SummarizationThreshold float64

	// SummarizationPrompt is the system prompt used for summarization.
	// Default: built-in prompt optimized for agent context preservation
	SummarizationPrompt string

	// PreserveLastN keeps the N most recent messages unsummarized.
	// Default: 4 (keep last 2 turns)
	PreserveLastN int

	// OnSummarize is called when summarization occurs.
	OnSummarize func(ctx context.Context, e SummarizationEvent)
}

// SummarizationEvent is emitted when auto-summarization occurs.
type SummarizationEvent struct {
	OriginalTokens   int
	SummarizedTokens int
	MessagesRemoved  int
	Summary          string
}

// DefaultMemoryConfig returns sensible defaults for memory management.
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MaxTokens:              100000,
		SummarizationThreshold: 0.8,
		SummarizationPrompt:    defaultSummarizationPrompt,
		PreserveLastN:          4,
	}
}

const defaultSummarizationPrompt = `Summarize the conversation so far, preserving:
1. The original user request and goals
2. Key decisions and reasoning
3. Tool calls made and their results
4. Current progress toward the goal
5. Any errors encountered and how they were handled

Be concise but complete. This summary will replace the conversation history.`

// AgentSnapshot captures the complete state of an agent execution for resumability.
type AgentSnapshot struct {
	// Version for forward compatibility
	Version string `json:"version"`

	// Conversation state
	Messages []Message `json:"messages"`

	// Execution progress
	Iteration      int             `json:"iteration"`
	TotalToolCalls int             `json:"total_tool_calls"`
	ToolHistory    []ToolExecution `json:"tool_history"`

	// Timing (for metrics continuity)
	StartTime   time.Time     `json:"start_time"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	TotalTokens TokenUsage    `json:"total_tokens"`

	// Configuration hash for validation on restore
	ConfigHash string `json:"config_hash"`

	// Last response (if paused mid-iteration)
	PendingToolCalls []ToolCall `json:"pending_tool_calls,omitempty"`
}

// SaveJSON serializes the snapshot to JSON for storage.
func (s *AgentSnapshot) SaveJSON() ([]byte, error) {
	return json.Marshal(s)
}

// LoadSnapshot deserializes a snapshot from JSON.
func LoadSnapshot(data []byte) (*AgentSnapshot, error) {
	var s AgentSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// AgentRunner executes an agentic loop with the configured tools.
type AgentRunner struct {
	builder  *ChatBuilder
	executor ToolExecutor
	config   AgentConfig

	// Internal state (protected by mu for snapshot/resume)
	mu    sync.RWMutex
	state *agentState
}

// agentState holds the mutable state of an agent execution.
type agentState struct {
	messages         []Message
	iteration        int
	totalToolCalls   int
	toolHistory      []ToolExecution
	startTime        time.Time
	totalTokens      TokenUsage
	pendingToolCalls []ToolCall
}

// Agent creates an agent runner from a ChatBuilder.
// The builder should already have messages, tools, and other options set.
// The executor parameter is typically a *tools.Registry.
func (b *ChatBuilder) Agent(executor ToolExecutor) *AgentRunner {
	return &AgentRunner{
		builder:  b,
		executor: executor,
		config:   DefaultAgentConfig(),
	}
}

// WithConfig sets the agent configuration.
func (r *AgentRunner) WithConfig(cfg AgentConfig) *AgentRunner {
	r.config = cfg
	return r
}

// WithMaxIterations sets the maximum iterations.
func (r *AgentRunner) WithMaxIterations(n int) *AgentRunner {
	r.config.MaxIterations = n
	return r
}

// WithMaxToolCalls sets the maximum tool calls.
func (r *AgentRunner) WithMaxToolCalls(n int) *AgentRunner {
	r.config.MaxToolCalls = n
	return r
}

// WithParallelTools enables or disables parallel tool execution.
func (r *AgentRunner) WithParallelTools(enabled bool) *AgentRunner {
	r.config.ParallelTools = enabled
	return r
}

// WithMaxParallelTools sets the maximum number of concurrent tool executions.
func (r *AgentRunner) WithMaxParallelTools(n int) *AgentRunner {
	r.config.MaxParallelTools = n
	return r
}

// WithToolTimeout sets the timeout for individual tool executions.
func (r *AgentRunner) WithToolTimeout(d time.Duration) *AgentRunner {
	r.config.ToolTimeout = d
	return r
}

// WithIterationTimeout sets the timeout for each LLM call.
func (r *AgentRunner) WithIterationTimeout(d time.Duration) *AgentRunner {
	r.config.IterationTimeout = d
	return r
}

// WithContinueOnToolError sets whether to continue when tools fail.
func (r *AgentRunner) WithContinueOnToolError(cont bool) *AgentRunner {
	r.config.ContinueOnToolError = cont
	return r
}

// WithStopSequences sets strings that terminate the agent loop.
func (r *AgentRunner) WithStopSequences(seqs ...string) *AgentRunner {
	r.config.StopSequences = seqs
	return r
}

// WithToolFilter sets a filter function for tool execution.
func (r *AgentRunner) WithToolFilter(f func(ToolCall) bool) *AgentRunner {
	r.config.ToolFilter = f
	return r
}

// WithHooks sets the agent hooks for observability.
func (r *AgentRunner) WithHooks(hooks AgentHooks) *AgentRunner {
	r.config.Hooks = hooks
	return r
}

// WithMemory enables memory management with auto-summarization.
func (r *AgentRunner) WithMemory(cfg *MemoryConfig) *AgentRunner {
	r.config.Memory = cfg
	return r
}

// Run executes the agent loop synchronously.
// Returns when the model completes or a termination condition is met.
func (r *AgentRunner) Run(ctx context.Context) (*AgentResult, error) {
	return r.execute(ctx, false)
}

// RunStream executes the agent loop with streaming.
// Text deltas are sent to hooks.OnTextDelta.
func (r *AgentRunner) RunStream(ctx context.Context) (*AgentResult, error) {
	return r.execute(ctx, true)
}

// Snapshot captures the current agent state for later resumption.
func (r *AgentRunner) Snapshot() (*AgentSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.state == nil {
		return nil, fmt.Errorf("no agent state to snapshot")
	}

	return &AgentSnapshot{
		Version:          "1.0",
		Messages:         r.state.messages,
		Iteration:        r.state.iteration,
		TotalToolCalls:   r.state.totalToolCalls,
		ToolHistory:      r.state.toolHistory,
		StartTime:        r.state.startTime,
		ElapsedTime:      time.Since(r.state.startTime),
		TotalTokens:      r.state.totalTokens,
		ConfigHash:       r.configHash(),
		PendingToolCalls: r.state.pendingToolCalls,
	}, nil
}

// Resume restores agent state from a snapshot and continues execution.
func (r *AgentRunner) Resume(ctx context.Context, snapshot *AgentSnapshot) (*AgentResult, error) {
	if snapshot.Version != "1.0" {
		return nil, fmt.Errorf("unsupported snapshot version: %s", snapshot.Version)
	}

	r.mu.Lock()
	r.state = &agentState{
		messages:         snapshot.Messages,
		iteration:        snapshot.Iteration,
		totalToolCalls:   snapshot.TotalToolCalls,
		toolHistory:      snapshot.ToolHistory,
		startTime:        time.Now().Add(-snapshot.ElapsedTime),
		totalTokens:      snapshot.TotalTokens,
		pendingToolCalls: snapshot.PendingToolCalls,
	}
	r.mu.Unlock()

	return r.execute(ctx, false)
}

// configHash returns a simple hash of the configuration for change detection.
func (r *AgentRunner) configHash() string {
	return fmt.Sprintf("%d-%d-%v-%d",
		r.config.MaxIterations,
		r.config.MaxToolCalls,
		r.config.ParallelTools,
		r.config.MaxParallelTools,
	)
}

func (r *AgentRunner) execute(ctx context.Context, streaming bool) (*AgentResult, error) {
	startTime := time.Now()
	result := &AgentResult{
		ToolHistory: make([]ToolExecution, 0),
	}

	// Initialize or restore state
	r.mu.Lock()
	if r.state == nil {
		r.state = &agentState{
			messages:    r.builder.req.Messages,
			iteration:   0,
			toolHistory: make([]ToolExecution, 0),
			startTime:   startTime,
		}
	}
	r.mu.Unlock()

	// Create a working builder from the original
	builder := r.builder.Clone()

	for {
		r.mu.Lock()
		totalToolCalls := r.state.totalToolCalls
		r.mu.Unlock()

		// Check iteration limit before incrementing
		r.mu.RLock()
		currentIteration := r.state.iteration
		r.mu.RUnlock()

		if r.config.MaxIterations > 0 && currentIteration >= r.config.MaxIterations {
			result.StopReason = StopReasonMaxIterations
			break
		}

		// Increment iteration
		r.mu.Lock()
		r.state.iteration++
		iteration := r.state.iteration
		r.mu.Unlock()

		// Check context cancellation
		if ctx.Err() != nil {
			result.StopReason = StopReasonCanceled
			result.Error = ctx.Err()
			break
		}

		// Call OnIterationStart hook
		if r.config.Hooks.OnIterationStart != nil {
			if err := r.config.Hooks.OnIterationStart(ctx, IterationStartEvent{
				Iteration:    iteration,
				MessageCount: len(builder.req.Messages),
				ToolCount:    totalToolCalls,
			}); err != nil {
				result.StopReason = StopReasonHookAbort
				result.Error = err
				break
			}
		}

		// Make LLM call with timeout
		iterStart := time.Now()
		var resp *ChatResponse
		var err error

		iterCtx := ctx
		var cancel context.CancelFunc
		if r.config.IterationTimeout > 0 {
			iterCtx, cancel = context.WithTimeout(ctx, r.config.IterationTimeout)
		}

		if streaming {
			resp, err = r.executeStreaming(iterCtx, builder)
		} else {
			resp, err = builder.GetResponse(iterCtx)
		}

		if cancel != nil {
			cancel()
		}

		if err != nil {
			result.StopReason = StopReasonError
			result.Error = err
			break
		}

		result.FinalResponse = resp
		r.mu.Lock()
		r.state.totalTokens = addTokenUsage(r.state.totalTokens, resp.Usage)
		r.mu.Unlock()
		result.TotalTokens = addTokenUsage(result.TotalTokens, resp.Usage)

		// Call OnIterationEnd hook
		if r.config.Hooks.OnIterationEnd != nil {
			r.config.Hooks.OnIterationEnd(ctx, IterationEndEvent{
				Iteration:  iteration,
				Response:   resp,
				ToolCalls:  resp.ToolCalls,
				Duration:   time.Since(iterStart),
				TokensUsed: resp.Usage,
			})
		}

		// Check for stop sequences
		if r.containsStopSequence(resp.Output) {
			result.StopReason = StopReasonStopSequence
			break
		}

		// Check if model is done (no tool calls)
		if !resp.HasToolCalls() {
			result.StopReason = StopReasonComplete
			break
		}

		// Check tool call limit
		if r.config.MaxToolCalls > 0 && totalToolCalls+len(resp.ToolCalls) > r.config.MaxToolCalls {
			result.StopReason = StopReasonMaxToolCalls
			break
		}

		// Execute tools
		toolResults, executions, err := r.executeTools(ctx, resp.ToolCalls, iteration)

		r.mu.Lock()
		r.state.toolHistory = append(r.state.toolHistory, executions...)
		r.state.totalToolCalls += len(resp.ToolCalls)
		r.mu.Unlock()

		result.ToolHistory = append(result.ToolHistory, executions...)

		if err != nil && !r.config.ContinueOnToolError {
			result.StopReason = StopReasonError
			result.Error = err
			break
		}

		// Inject tool results for next iteration
		builder = builder.ToolResults(resp, toolResults)

		// Update internal state messages
		r.mu.Lock()
		r.state.messages = builder.req.Messages
		r.mu.Unlock()
	}

	r.mu.RLock()
	result.Iterations = r.state.iteration
	r.mu.RUnlock()
	result.Duration = time.Since(startTime)

	// Call OnAgentComplete hook
	if r.config.Hooks.OnAgentComplete != nil {
		r.config.Hooks.OnAgentComplete(ctx, AgentCompleteEvent{
			Iterations:     result.Iterations,
			TotalToolCalls: result.TotalToolCalls(),
			TotalDuration:  result.Duration,
			TotalTokens:    result.TotalTokens,
			StopReason:     result.StopReason,
			FinalResponse:  result.FinalResponse,
		})
	}

	return result, nil
}

func (r *AgentRunner) executeTools(
	ctx context.Context,
	calls []ToolCall,
	iteration int,
) ([]ToolResult, []ToolExecution, error) {
	if r.config.ParallelTools && len(calls) > 1 {
		return r.executeToolsParallel(ctx, calls, iteration)
	}
	return r.executeToolsSequential(ctx, calls, iteration)
}

func (r *AgentRunner) executeToolsSequential(
	ctx context.Context,
	calls []ToolCall,
	iteration int,
) ([]ToolResult, []ToolExecution, error) {
	results := make([]ToolResult, 0, len(calls))
	executions := make([]ToolExecution, 0, len(calls))
	var firstErr error

	for i, call := range calls {
		// Apply tool filter
		if r.config.ToolFilter != nil && !r.config.ToolFilter(call) {
			err := fmt.Errorf("tool %q not allowed", call.Name)
			results = append(results, ToolResult{
				CallID:  call.ID,
				Content: err.Error(),
				IsError: true,
			})
			executions = append(executions, ToolExecution{
				Iteration: iteration,
				Call:      call,
				Error:     err,
				Timestamp: time.Now(),
			})
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// Call OnToolCallStart hook
		if r.config.Hooks.OnToolCallStart != nil {
			if err := r.config.Hooks.OnToolCallStart(ctx, ToolCallStartEvent{
				Iteration: iteration,
				ToolCall:  call,
				Index:     i,
				Total:     len(calls),
			}); err != nil {
				results = append(results, ToolResult{
					CallID:  call.ID,
					Content: err.Error(),
					IsError: true,
				})
				executions = append(executions, ToolExecution{
					Iteration: iteration,
					Call:      call,
					Error:     err,
					Timestamp: time.Now(),
				})
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
		}

		// Execute tool with timeout
		toolCtx := ctx
		var cancel context.CancelFunc
		if r.config.ToolTimeout > 0 {
			toolCtx, cancel = context.WithTimeout(ctx, r.config.ToolTimeout)
		}

		execStart := time.Now()
		result, err := r.executor.Execute(toolCtx, call.Name, call.Arguments)
		execDuration := time.Since(execStart)

		if cancel != nil {
			cancel()
		}

		// Call OnToolCallEnd hook
		if r.config.Hooks.OnToolCallEnd != nil {
			r.config.Hooks.OnToolCallEnd(ctx, ToolCallEndEvent{
				Iteration: iteration,
				ToolCall:  call,
				Result:    result,
				Error:     err,
				Duration:  execDuration,
			})
		}

		execution := ToolExecution{
			Iteration: iteration,
			Call:      call,
			Result:    result,
			Error:     err,
			Duration:  execDuration,
			Timestamp: execStart,
		}
		executions = append(executions, execution)

		if err != nil {
			results = append(results, ToolResult{
				CallID:  call.ID,
				Content: err.Error(),
				IsError: true,
			})
			if firstErr == nil {
				firstErr = err
			}
		} else {
			results = append(results, ToolResult{
				CallID:  call.ID,
				Content: result,
				IsError: false,
			})
		}
	}

	return results, executions, firstErr
}

func (r *AgentRunner) executeToolsParallel(
	ctx context.Context,
	calls []ToolCall,
	iteration int,
) ([]ToolResult, []ToolExecution, error) {
	type toolOutput struct {
		index     int
		result    ToolResult
		execution ToolExecution
	}

	// Create semaphore for max parallel tools
	semSize := len(calls)
	if r.config.MaxParallelTools > 0 && r.config.MaxParallelTools < semSize {
		semSize = r.config.MaxParallelTools
	}
	sem := make(chan struct{}, semSize)

	outputs := make(chan toolOutput, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c ToolCall) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Apply tool filter
			if r.config.ToolFilter != nil && !r.config.ToolFilter(c) {
				err := fmt.Errorf("tool %q not allowed", c.Name)
				outputs <- toolOutput{
					index: idx,
					result: ToolResult{
						CallID:  c.ID,
						Content: err.Error(),
						IsError: true,
					},
					execution: ToolExecution{
						Iteration: iteration,
						Call:      c,
						Error:     err,
						Timestamp: time.Now(),
					},
				}
				return
			}

			// Call OnToolCallStart hook
			if r.config.Hooks.OnToolCallStart != nil {
				if err := r.config.Hooks.OnToolCallStart(ctx, ToolCallStartEvent{
					Iteration: iteration,
					ToolCall:  c,
					Index:     idx,
					Total:     len(calls),
				}); err != nil {
					outputs <- toolOutput{
						index: idx,
						result: ToolResult{
							CallID:  c.ID,
							Content: err.Error(),
							IsError: true,
						},
						execution: ToolExecution{
							Iteration: iteration,
							Call:      c,
							Error:     err,
							Timestamp: time.Now(),
						},
					}
					return
				}
			}

			// Execute tool with timeout
			toolCtx := ctx
			var cancel context.CancelFunc
			if r.config.ToolTimeout > 0 {
				toolCtx, cancel = context.WithTimeout(ctx, r.config.ToolTimeout)
			}

			execStart := time.Now()
			result, err := r.executor.Execute(toolCtx, c.Name, c.Arguments)
			execDuration := time.Since(execStart)

			if cancel != nil {
				cancel()
			}

			// Call OnToolCallEnd hook
			if r.config.Hooks.OnToolCallEnd != nil {
				r.config.Hooks.OnToolCallEnd(ctx, ToolCallEndEvent{
					Iteration: iteration,
					ToolCall:  c,
					Result:    result,
					Error:     err,
					Duration:  execDuration,
				})
			}

			var toolResult ToolResult
			if err != nil {
				toolResult = ToolResult{
					CallID:  c.ID,
					Content: err.Error(),
					IsError: true,
				}
			} else {
				toolResult = ToolResult{
					CallID:  c.ID,
					Content: result,
					IsError: false,
				}
			}

			outputs <- toolOutput{
				index:  idx,
				result: toolResult,
				execution: ToolExecution{
					Iteration: iteration,
					Call:      c,
					Result:    result,
					Error:     err,
					Duration:  execDuration,
					Timestamp: execStart,
				},
			}
		}(i, call)
	}

	// Close outputs when all done
	go func() {
		wg.Wait()
		close(outputs)
	}()

	// Collect results in order
	results := make([]ToolResult, len(calls))
	executions := make([]ToolExecution, len(calls))
	var firstErr error

	for out := range outputs {
		results[out.index] = out.result
		executions[out.index] = out.execution
		if out.execution.Error != nil && firstErr == nil {
			firstErr = out.execution.Error
		}
	}

	return results, executions, firstErr
}

func (r *AgentRunner) executeStreaming(ctx context.Context, builder *ChatBuilder) (*ChatResponse, error) {
	stream, err := builder.Stream(ctx)
	if err != nil {
		return nil, err
	}

	var accumulated strings.Builder

	// Stream text deltas to hook
	for chunk := range stream.Ch {
		if r.config.Hooks.OnTextDelta != nil && chunk.Delta != "" {
			r.config.Hooks.OnTextDelta(ctx, chunk.Delta)
		}
		accumulated.WriteString(chunk.Delta)
	}

	// Check for streaming errors
	select {
	case err := <-stream.Err:
		if err != nil {
			return nil, err
		}
	default:
	}

	// Get final response
	select {
	case resp := <-stream.Final:
		if resp != nil {
			if resp.Output == "" {
				resp.Output = accumulated.String()
			}
			return resp, nil
		}
		// No final response, create one from accumulated
		return &ChatResponse{Output: accumulated.String()}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *AgentRunner) containsStopSequence(output string) bool {
	for _, seq := range r.config.StopSequences {
		if strings.Contains(output, seq) {
			return true
		}
	}
	return false
}

// addTokenUsage adds two TokenUsage structs together.
func addTokenUsage(a, b TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		TotalTokens:      a.TotalTokens + b.TotalTokens,
	}
}
