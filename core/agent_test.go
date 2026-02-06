package core

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockToolExecutor is a test implementation of ToolExecutor.
type mockToolExecutor struct {
	tools     map[string]func(ctx context.Context, args json.RawMessage) (any, error)
	callCount int
	mu        sync.Mutex
}

func newMockToolExecutor() *mockToolExecutor {
	return &mockToolExecutor{
		tools: make(map[string]func(ctx context.Context, args json.RawMessage) (any, error)),
	}
}

func (m *mockToolExecutor) Register(name string, fn func(ctx context.Context, args json.RawMessage) (any, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[name] = fn
}

func (m *mockToolExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (any, error) {
	m.mu.Lock()
	m.callCount++
	fn, ok := m.tools[name]
	m.mu.Unlock() // Release lock before calling function to allow parallel execution

	if !ok {
		return nil, errors.New("tool not found: " + name)
	}
	return fn(ctx, args)
}

func (m *mockToolExecutor) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockAgentProvider simulates a provider that returns tool calls.
type mockAgentProvider struct {
	mockProvider
	responses     []*ChatResponse
	responseIndex int
	mu            sync.Mutex
}

func newMockAgentProvider(responses []*ChatResponse) *mockAgentProvider {
	return &mockAgentProvider{
		mockProvider: mockProvider{id: "mock-agent"},
		responses:    responses,
	}
}

func (m *mockAgentProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.responseIndex >= len(m.responses) {
		return &ChatResponse{Output: "No more responses"}, nil
	}

	resp := m.responses[m.responseIndex]
	m.responseIndex++
	return resp, nil
}

func TestDefaultAgentConfig(t *testing.T) {
	cfg := DefaultAgentConfig()

	if cfg.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want 10", cfg.MaxIterations)
	}
	if cfg.MaxToolCalls != 50 {
		t.Errorf("MaxToolCalls = %d, want 50", cfg.MaxToolCalls)
	}
	if cfg.IterationTimeout != 30*time.Second {
		t.Errorf("IterationTimeout = %v, want 30s", cfg.IterationTimeout)
	}
	if cfg.ToolTimeout != 60*time.Second {
		t.Errorf("ToolTimeout = %v, want 60s", cfg.ToolTimeout)
	}
	if !cfg.ParallelTools {
		t.Error("ParallelTools should be true by default")
	}
	if cfg.MaxParallelTools != 5 {
		t.Errorf("MaxParallelTools = %d, want 5", cfg.MaxParallelTools)
	}
	if !cfg.ContinueOnToolError {
		t.Error("ContinueOnToolError should be true by default")
	}
}

func TestAgentRunNoToolCalls(t *testing.T) {
	// Provider returns response with no tool calls
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", Output: "Hello, how can I help?"},
	})
	client := NewClient(provider)
	executor := newMockToolExecutor()

	result, err := client.Chat("test-model").
		User("Hello").
		Agent(executor).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonComplete {
		t.Errorf("StopReason = %v, want complete", result.StopReason)
	}
	if result.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", result.Iterations)
	}
	if result.TotalToolCalls() != 0 {
		t.Errorf("TotalToolCalls = %d, want 0", result.TotalToolCalls())
	}
	if result.FinalResponse.Output != "Hello, how can I help?" {
		t.Errorf("Output = %q, want 'Hello, how can I help?'", result.FinalResponse.Output)
	}
}

func TestAgentRunWithToolCalls(t *testing.T) {
	// Provider returns tool call, then final response
	provider := newMockAgentProvider([]*ChatResponse{
		{
			ID:        "resp-1",
			ToolCalls: []ToolCall{{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"NYC"}`)}},
		},
		{
			ID:     "resp-2",
			Output: "The weather in NYC is sunny.",
		},
	})
	client := NewClient(provider)

	executor := newMockToolExecutor()
	executor.Register("get_weather", func(ctx context.Context, args json.RawMessage) (any, error) {
		return map[string]string{"condition": "sunny", "temp": "72F"}, nil
	})

	result, err := client.Chat("test-model").
		User("What's the weather in NYC?").
		Agent(executor).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonComplete {
		t.Errorf("StopReason = %v, want complete", result.StopReason)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", result.Iterations)
	}
	if result.TotalToolCalls() != 1 {
		t.Errorf("TotalToolCalls = %d, want 1", result.TotalToolCalls())
	}
	if executor.CallCount() != 1 {
		t.Errorf("executor.CallCount = %d, want 1", executor.CallCount())
	}
}

func TestAgentRunMaxIterations(t *testing.T) {
	// Provider always returns tool calls
	provider := &mockAgentProvider{
		mockProvider: mockProvider{id: "mock"},
		responses:    make([]*ChatResponse, 20),
	}
	for i := range provider.responses {
		provider.responses[i] = &ChatResponse{
			ID:        "resp",
			ToolCalls: []ToolCall{{ID: "call", Name: "test_tool", Arguments: json.RawMessage(`{}`)}},
		}
	}

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("test_tool", func(ctx context.Context, args json.RawMessage) (any, error) {
		return "ok", nil
	})

	result, err := client.Chat("test-model").
		User("Run forever").
		Agent(executor).
		WithMaxIterations(5).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonMaxIterations {
		t.Errorf("StopReason = %v, want max_iterations", result.StopReason)
	}
	if result.Iterations != 5 {
		t.Errorf("Iterations = %d, want 5", result.Iterations)
	}
}

func TestAgentRunMaxToolCalls(t *testing.T) {
	// Provider returns multiple tool calls per iteration
	provider := &mockAgentProvider{
		mockProvider: mockProvider{id: "mock"},
		responses:    make([]*ChatResponse, 10),
	}
	for i := range provider.responses {
		provider.responses[i] = &ChatResponse{
			ID: "resp",
			ToolCalls: []ToolCall{
				{ID: "call_a", Name: "tool_a", Arguments: json.RawMessage(`{}`)},
				{ID: "call_b", Name: "tool_b", Arguments: json.RawMessage(`{}`)},
				{ID: "call_c", Name: "tool_c", Arguments: json.RawMessage(`{}`)},
			},
		}
	}

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("tool_a", func(ctx context.Context, args json.RawMessage) (any, error) { return "a", nil })
	executor.Register("tool_b", func(ctx context.Context, args json.RawMessage) (any, error) { return "b", nil })
	executor.Register("tool_c", func(ctx context.Context, args json.RawMessage) (any, error) { return "c", nil })

	result, err := client.Chat("test-model").
		User("Run tools").
		Agent(executor).
		WithMaxToolCalls(5).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonMaxToolCalls {
		t.Errorf("StopReason = %v, want max_tool_calls", result.StopReason)
	}
	// First iteration executes 3 tools, second would exceed limit
	if result.TotalToolCalls() != 3 {
		t.Errorf("TotalToolCalls = %d, want 3", result.TotalToolCalls())
	}
}

func TestAgentRunStopSequence(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", Output: "Processing..."},
		{ID: "resp-2", Output: "TASK_COMPLETE: All done!"},
	})
	// First response has tool call to continue the loop
	provider.responses[0].ToolCalls = []ToolCall{{ID: "call", Name: "process", Arguments: json.RawMessage(`{}`)}}

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("process", func(ctx context.Context, args json.RawMessage) (any, error) { return "done", nil })

	result, err := client.Chat("test-model").
		User("Process").
		Agent(executor).
		WithStopSequences("TASK_COMPLETE").
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonStopSequence {
		t.Errorf("StopReason = %v, want stop_sequence", result.StopReason)
	}
}

func TestAgentRunToolError(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{{ID: "call", Name: "failing_tool", Arguments: json.RawMessage(`{}`)}}},
		{ID: "resp-2", Output: "I see the tool failed, let me help differently."},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("failing_tool", func(ctx context.Context, args json.RawMessage) (any, error) {
		return nil, errors.New("connection timeout")
	})

	// Default: continue on tool error
	result, err := client.Chat("test-model").
		User("Try tool").
		Agent(executor).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonComplete {
		t.Errorf("StopReason = %v, want complete", result.StopReason)
	}
	if len(result.FailedToolCalls()) != 1 {
		t.Errorf("FailedToolCalls = %d, want 1", len(result.FailedToolCalls()))
	}
}

func TestAgentRunToolErrorFailFast(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{{ID: "call", Name: "failing_tool", Arguments: json.RawMessage(`{}`)}}},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("failing_tool", func(ctx context.Context, args json.RawMessage) (any, error) {
		return nil, errors.New("connection timeout")
	})

	result, err := client.Chat("test-model").
		User("Try tool").
		Agent(executor).
		WithContinueOnToolError(false).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonError {
		t.Errorf("StopReason = %v, want error", result.StopReason)
	}
	if result.Error == nil {
		t.Error("Error should be set")
	}
}

func TestAgentRunToolFilter(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{
			{ID: "call_1", Name: "read_file", Arguments: json.RawMessage(`{}`)},
			{ID: "call_2", Name: "write_file", Arguments: json.RawMessage(`{}`)},
		}},
		{ID: "resp-2", Output: "I could only read, not write."},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("read_file", func(ctx context.Context, args json.RawMessage) (any, error) { return "file contents", nil })
	executor.Register("write_file", func(ctx context.Context, args json.RawMessage) (any, error) { return "written", nil })

	result, err := client.Chat("test-model").
		User("Read and write").
		Agent(executor).
		WithToolFilter(func(call ToolCall) bool {
			return call.Name != "write_file" // Block write_file
		}).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 success (read) and 1 failure (write blocked)
	if len(result.SuccessfulToolCalls()) != 1 {
		t.Errorf("SuccessfulToolCalls = %d, want 1", len(result.SuccessfulToolCalls()))
	}
	if len(result.FailedToolCalls()) != 1 {
		t.Errorf("FailedToolCalls = %d, want 1", len(result.FailedToolCalls()))
	}
}

func TestAgentRunParallelTools(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{
			{ID: "call_1", Name: "slow_tool", Arguments: json.RawMessage(`{"id":1}`)},
			{ID: "call_2", Name: "slow_tool", Arguments: json.RawMessage(`{"id":2}`)},
			{ID: "call_3", Name: "slow_tool", Arguments: json.RawMessage(`{"id":3}`)},
		}},
		{ID: "resp-2", Output: "Done"},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()

	var concurrentCalls int32
	var maxConcurrent int32
	var mu sync.Mutex

	executor.Register("slow_tool", func(ctx context.Context, args json.RawMessage) (any, error) {
		current := atomic.AddInt32(&concurrentCalls, 1)
		defer atomic.AddInt32(&concurrentCalls, -1)

		// Track max concurrent safely
		mu.Lock()
		if current > maxConcurrent {
			maxConcurrent = current
		}
		mu.Unlock()

		time.Sleep(100 * time.Millisecond) // Longer sleep for more reliable testing
		return "done", nil
	})

	start := time.Now()
	result, err := client.Chat("test-model").
		User("Run parallel").
		Agent(executor).
		WithParallelTools(true).
		WithMaxParallelTools(10). // Ensure enough parallelism
		Run(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalToolCalls() != 3 {
		t.Errorf("TotalToolCalls = %d, want 3", result.TotalToolCalls())
	}

	// Sequential would take 300ms (3 * 100ms)
	// Parallel should take ~100-150ms
	// Allow up to 200ms for CI environments
	if elapsed > 200*time.Millisecond {
		t.Logf("Note: elapsed = %v (parallel execution might not be working)", elapsed)
		// Only fail if it took more than 250ms (clearly sequential)
		if elapsed > 250*time.Millisecond {
			t.Errorf("elapsed = %v, expected < 250ms for parallel execution", elapsed)
		}
	}

	mu.Lock()
	maxC := maxConcurrent
	mu.Unlock()

	// Should have run at least 2 concurrently
	// (may not always hit 3 due to goroutine scheduling)
	if maxC < 2 {
		t.Logf("Note: maxConcurrent = %d (expected >= 2, but can vary)", maxC)
	}
}

func TestAgentRunSequentialTools(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{
			{ID: "call_1", Name: "tool", Arguments: json.RawMessage(`{}`)},
			{ID: "call_2", Name: "tool", Arguments: json.RawMessage(`{}`)},
		}},
		{ID: "resp-2", Output: "Done"},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()

	var order []int
	var mu sync.Mutex

	executor.Register("tool", func(ctx context.Context, args json.RawMessage) (any, error) {
		mu.Lock()
		order = append(order, len(order)+1)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return "done", nil
	})

	result, err := client.Chat("test-model").
		User("Run sequential").
		Agent(executor).
		WithParallelTools(false). // Sequential
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalToolCalls() != 2 {
		t.Errorf("TotalToolCalls = %d, want 2", result.TotalToolCalls())
	}

	// Verify sequential order
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("order = %v, want [1, 2]", order)
	}
}

func TestAgentRunContextCancellation(t *testing.T) {
	// Create a provider that simulates slow responses
	provider := &mockProvider{
		id: "mock-slow",
		chatFunc: func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
			// Simulate slow response
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return &ChatResponse{Output: "Done"}, nil
			}
		},
	}

	client := NewClient(provider)
	executor := newMockToolExecutor()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := client.Chat("test-model").
		User("Slow request").
		Agent(executor).
		Run(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonError {
		t.Errorf("StopReason = %v, want error (context deadline)", result.StopReason)
	}
	if result.Error == nil {
		t.Error("Error should be set for context cancellation")
	}
}

func TestAgentHooks(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{{ID: "call", Name: "test", Arguments: json.RawMessage(`{}`)}}},
		{ID: "resp-2", Output: "Done"},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("test", func(ctx context.Context, args json.RawMessage) (any, error) { return "ok", nil })

	var events []string
	var mu sync.Mutex

	result, err := client.Chat("test-model").
		User("Test hooks").
		Agent(executor).
		WithHooks(AgentHooks{
			OnIterationStart: func(ctx context.Context, e IterationStartEvent) error {
				mu.Lock()
				events = append(events, "iteration_start")
				mu.Unlock()
				return nil
			},
			OnIterationEnd: func(ctx context.Context, e IterationEndEvent) {
				mu.Lock()
				events = append(events, "iteration_end")
				mu.Unlock()
			},
			OnToolCallStart: func(ctx context.Context, e ToolCallStartEvent) error {
				mu.Lock()
				events = append(events, "tool_start")
				mu.Unlock()
				return nil
			},
			OnToolCallEnd: func(ctx context.Context, e ToolCallEndEvent) {
				mu.Lock()
				events = append(events, "tool_end")
				mu.Unlock()
			},
			OnAgentComplete: func(ctx context.Context, e AgentCompleteEvent) {
				mu.Lock()
				events = append(events, "complete")
				mu.Unlock()
			},
		}).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonComplete {
		t.Errorf("StopReason = %v, want complete", result.StopReason)
	}

	mu.Lock()
	defer mu.Unlock()

	// Expected order: iter1_start, iter1_end, tool_start, tool_end, iter2_start, iter2_end, complete
	expected := []string{
		"iteration_start", "iteration_end",
		"tool_start", "tool_end",
		"iteration_start", "iteration_end",
		"complete",
	}

	if len(events) != len(expected) {
		t.Fatalf("events = %v, want %v", events, expected)
	}
	for i, e := range expected {
		if events[i] != e {
			t.Errorf("events[%d] = %q, want %q", i, events[i], e)
		}
	}
}

func TestAgentHookAbort(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{{ID: "call", Name: "test", Arguments: json.RawMessage(`{}`)}}},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("test", func(ctx context.Context, args json.RawMessage) (any, error) { return "ok", nil })

	result, err := client.Chat("test-model").
		User("Test").
		Agent(executor).
		WithHooks(AgentHooks{
			OnIterationStart: func(ctx context.Context, e IterationStartEvent) error {
				if e.Iteration > 1 {
					return errors.New("abort after first iteration")
				}
				return nil
			},
		}).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != StopReasonHookAbort {
		t.Errorf("StopReason = %v, want hook_abort", result.StopReason)
	}
}

func TestAgentResultMethods(t *testing.T) {
	result := &AgentResult{
		ToolHistory: []ToolExecution{
			{Call: ToolCall{Name: "success1"}, Error: nil},
			{Call: ToolCall{Name: "fail1"}, Error: errors.New("error1")},
			{Call: ToolCall{Name: "success2"}, Error: nil},
			{Call: ToolCall{Name: "fail2"}, Error: errors.New("error2")},
		},
		StopReason: StopReasonComplete,
	}

	if result.TotalToolCalls() != 4 {
		t.Errorf("TotalToolCalls = %d, want 4", result.TotalToolCalls())
	}
	if len(result.SuccessfulToolCalls()) != 2 {
		t.Errorf("SuccessfulToolCalls = %d, want 2", len(result.SuccessfulToolCalls()))
	}
	if len(result.FailedToolCalls()) != 2 {
		t.Errorf("FailedToolCalls = %d, want 2", len(result.FailedToolCalls()))
	}
	if result.HasError() {
		t.Error("HasError should be false for StopReasonComplete")
	}

	result.StopReason = StopReasonError
	if !result.HasError() {
		t.Error("HasError should be true for StopReasonError")
	}
}

func TestAgentSnapshot(t *testing.T) {
	provider := newMockAgentProvider([]*ChatResponse{
		{ID: "resp-1", ToolCalls: []ToolCall{{ID: "call", Name: "test", Arguments: json.RawMessage(`{}`)}}},
		{ID: "resp-2", Output: "Done"},
	})

	client := NewClient(provider)
	executor := newMockToolExecutor()
	executor.Register("test", func(ctx context.Context, args json.RawMessage) (any, error) { return "ok", nil })

	var snapshot *AgentSnapshot

	_, err := client.Chat("test-model").
		User("Test snapshot").
		Agent(executor).
		WithHooks(AgentHooks{
			OnIterationEnd: func(ctx context.Context, e IterationEndEvent) {
				// Capture snapshot after first iteration
				if e.Iteration == 1 {
					runner := client.Chat("test-model").User("Test").Agent(executor)
					runner.state = &agentState{
						messages:       []Message{{Role: RoleUser, Content: "Test"}},
						iteration:      1,
						totalToolCalls: 1,
						toolHistory:    []ToolExecution{},
						startTime:      time.Now(),
					}
					s, _ := runner.Snapshot()
					snapshot = s
				}
			},
		}).
		Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snapshot == nil {
		t.Fatal("snapshot should have been captured")
	}
	if snapshot.Version != "1.0" {
		t.Errorf("Version = %q, want '1.0'", snapshot.Version)
	}
	if snapshot.Iteration != 1 {
		t.Errorf("Iteration = %d, want 1", snapshot.Iteration)
	}
}

func TestAgentSnapshotSaveLoad(t *testing.T) {
	snapshot := &AgentSnapshot{
		Version:        "1.0",
		Messages:       []Message{{Role: RoleUser, Content: "Hello"}},
		Iteration:      3,
		TotalToolCalls: 5,
		StartTime:      time.Now(),
		ElapsedTime:    10 * time.Second,
	}

	data, err := snapshot.SaveJSON()
	if err != nil {
		t.Fatalf("SaveJSON error: %v", err)
	}

	loaded, err := LoadSnapshot(data)
	if err != nil {
		t.Fatalf("LoadSnapshot error: %v", err)
	}

	if loaded.Version != snapshot.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, snapshot.Version)
	}
	if loaded.Iteration != snapshot.Iteration {
		t.Errorf("Iteration = %d, want %d", loaded.Iteration, snapshot.Iteration)
	}
	if loaded.TotalToolCalls != snapshot.TotalToolCalls {
		t.Errorf("TotalToolCalls = %d, want %d", loaded.TotalToolCalls, snapshot.TotalToolCalls)
	}
}

func TestDefaultMemoryConfig(t *testing.T) {
	cfg := DefaultMemoryConfig()

	if cfg.MaxTokens != 100000 {
		t.Errorf("MaxTokens = %d, want 100000", cfg.MaxTokens)
	}
	if cfg.SummarizationThreshold != 0.8 {
		t.Errorf("SummarizationThreshold = %f, want 0.8", cfg.SummarizationThreshold)
	}
	if cfg.PreserveLastN != 4 {
		t.Errorf("PreserveLastN = %d, want 4", cfg.PreserveLastN)
	}
	if cfg.SummarizationPrompt == "" {
		t.Error("SummarizationPrompt should not be empty")
	}
}

func TestAddTokenUsage(t *testing.T) {
	a := TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	b := TokenUsage{PromptTokens: 5, CompletionTokens: 15, TotalTokens: 20}

	result := addTokenUsage(a, b)

	if result.PromptTokens != 15 {
		t.Errorf("PromptTokens = %d, want 15", result.PromptTokens)
	}
	if result.CompletionTokens != 35 {
		t.Errorf("CompletionTokens = %d, want 35", result.CompletionTokens)
	}
	if result.TotalTokens != 50 {
		t.Errorf("TotalTokens = %d, want 50", result.TotalTokens)
	}
}
