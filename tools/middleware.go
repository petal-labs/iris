package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToolCallFunc is the function signature for tool execution.
// Middleware wraps this function to add behavior.
type ToolCallFunc func(ctx context.Context, args json.RawMessage) (any, error)

// Middleware wraps a ToolCallFunc to add behavior before and/or after execution.
// Middleware functions receive the next handler in the chain and return a new handler.
type Middleware func(next ToolCallFunc) ToolCallFunc

// ToolContext provides metadata about the current tool call to middleware.
// It's stored in the context and accessible via ToolContextFromContext.
type ToolContext struct {
	// ToolName is the name of the tool being called.
	ToolName string

	// CallID is a unique identifier for this invocation (if available).
	CallID string

	// Iteration is the current workflow loop iteration (if provided by caller).
	Iteration int

	// Metadata allows middleware to share data with each other.
	Metadata map[string]any
}

// toolContextKey is the context key for ToolContext.
type toolContextKey struct{}

// ContextWithToolContext adds ToolContext to a context.
func ContextWithToolContext(ctx context.Context, tc *ToolContext) context.Context {
	return context.WithValue(ctx, toolContextKey{}, tc)
}

// ToolContextFromContext retrieves ToolContext from a context.
// Returns nil if not present.
func ToolContextFromContext(ctx context.Context) *ToolContext {
	tc, _ := ctx.Value(toolContextKey{}).(*ToolContext)
	return tc
}

// Chain combines multiple middleware into a single middleware.
// Middleware are executed in the order provided (first middleware is outermost).
func Chain(middlewares ...Middleware) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		// Apply in reverse order so first middleware is outermost
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// ApplyMiddleware wraps a tool with middleware.
// Returns a new tool that executes middleware around the original.
func ApplyMiddleware(tool Tool, middlewares ...Middleware) Tool {
	if len(middlewares) == 0 {
		return tool
	}

	chain := Chain(middlewares...)
	wrapped := chain(tool.Call)

	return &wrappedTool{
		tool:    tool,
		wrapped: wrapped,
	}
}

// wrappedTool is a tool with middleware applied.
type wrappedTool struct {
	tool    Tool
	wrapped ToolCallFunc
}

func (w *wrappedTool) Name() string        { return w.tool.Name() }
func (w *wrappedTool) Description() string { return w.tool.Description() }
func (w *wrappedTool) Schema() ToolSchema  { return w.tool.Schema() }

func (w *wrappedTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Ensure ToolContext exists
	tc := ToolContextFromContext(ctx)
	if tc == nil {
		tc = &ToolContext{
			ToolName: w.tool.Name(),
			Metadata: make(map[string]any),
		}
		ctx = ContextWithToolContext(ctx, tc)
	} else if tc.ToolName == "" {
		tc.ToolName = w.tool.Name()
	}

	return w.wrapped(ctx, args)
}

// -----------------------------------------------------------------------------
// Logging Middleware
// -----------------------------------------------------------------------------

// Logger is the interface for logging middleware.
type Logger interface {
	Printf(format string, v ...any)
}

// WithLogging creates middleware that logs tool calls.
func WithLogging(logger Logger) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := "unknown"
			if tc != nil {
				toolName = tc.ToolName
			}

			logger.Printf("tool call start: %s", toolName)
			start := time.Now()

			result, err := next(ctx, args)

			duration := time.Since(start)
			if err != nil {
				logger.Printf("tool call error: %s, duration=%v, error=%v", toolName, duration, err)
			} else {
				logger.Printf("tool call success: %s, duration=%v", toolName, duration)
			}

			return result, err
		}
	}
}

// WithDetailedLogging creates middleware that logs tool calls with arguments.
// WARNING: May log sensitive data. Use only in development.
func WithDetailedLogging(logger Logger) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := "unknown"
			if tc != nil {
				toolName = tc.ToolName
			}

			logger.Printf("tool call: %s, args=%s", toolName, string(args))
			start := time.Now()

			result, err := next(ctx, args)

			duration := time.Since(start)
			if err != nil {
				logger.Printf("tool error: %s, duration=%v, error=%v", toolName, duration, err)
			} else {
				resultJSON, _ := json.Marshal(result)
				logger.Printf("tool result: %s, duration=%v, result=%s", toolName, duration, string(resultJSON))
			}

			return result, err
		}
	}
}

// -----------------------------------------------------------------------------
// Timeout Middleware
// -----------------------------------------------------------------------------

// WithTimeout creates middleware that enforces a timeout on tool execution.
func WithTimeout(d time.Duration) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()

			// Execute in goroutine to respect timeout
			type result struct {
				value any
				err   error
			}
			ch := make(chan result, 1)

			go func() {
				v, err := next(ctx, args)
				ch <- result{v, err}
			}()

			select {
			case r := <-ch:
				return r.value, r.err
			case <-ctx.Done():
				return nil, fmt.Errorf("tool execution timeout after %v", d)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Rate Limiting Middleware
// -----------------------------------------------------------------------------

// RateLimiter is the interface for rate limiting.
type RateLimiter interface {
	// Allow returns true if the request should proceed.
	Allow() bool
	// Wait blocks until the request can proceed or context is canceled.
	Wait(ctx context.Context) error
}

// WithRateLimit creates middleware that rate limits tool calls.
// Uses a token bucket algorithm with the specified rate (calls per second).
func WithRateLimit(ratePerSecond float64) Middleware {
	limiter := newTokenBucket(ratePerSecond)
	return WithRateLimiter(limiter)
}

// WithRateLimiter creates middleware using a custom rate limiter.
func WithRateLimiter(limiter RateLimiter) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			if err := limiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("rate limit exceeded: %w", err)
			}
			return next(ctx, args)
		}
	}
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func newTokenBucket(ratePerSecond float64) *tokenBucket {
	return &tokenBucket{
		tokens:     ratePerSecond,
		maxTokens:  ratePerSecond * 2, // Allow burst of 2x rate
		refillRate: ratePerSecond,
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

func (tb *tokenBucket) Wait(ctx context.Context) error {
	for {
		if tb.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Retry
		}
	}
}

func (tb *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now
}

// -----------------------------------------------------------------------------
// Caching Middleware
// -----------------------------------------------------------------------------

// Cache is the interface for caching tool results.
type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any, ttl time.Duration)
}

// CacheKeyFunc generates a cache key from tool name and arguments.
type CacheKeyFunc func(toolName string, args json.RawMessage) string

// DefaultCacheKey generates a cache key by hashing tool name and arguments.
func DefaultCacheKey(toolName string, args json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write(args)
	return hex.EncodeToString(h.Sum(nil))
}

// WithCache creates middleware that caches tool results.
func WithCache(cache Cache, ttl time.Duration) Middleware {
	return WithCacheCustomKey(cache, ttl, DefaultCacheKey)
}

// WithCacheCustomKey creates caching middleware with a custom key function.
func WithCacheCustomKey(cache Cache, ttl time.Duration, keyFunc CacheKeyFunc) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := ""
			if tc != nil {
				toolName = tc.ToolName
			}

			key := keyFunc(toolName, args)

			// Check cache
			if cached, ok := cache.Get(key); ok {
				return cached, nil
			}

			// Execute tool
			result, err := next(ctx, args)
			if err != nil {
				return nil, err
			}

			// Cache successful result
			cache.Set(key, result, ttl)
			return result, nil
		}
	}
}

// memoryCache is a simple in-memory cache implementation.
type memoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value   any
	expires time.Time
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache() Cache {
	return &memoryCache{
		items: make(map[string]cacheItem),
	}
}

func (c *memoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expires) {
		return nil, false
	}
	return item.value, true
}

func (c *memoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:   value,
		expires: time.Now().Add(ttl),
	}
}

// -----------------------------------------------------------------------------
// Validation Middleware
// -----------------------------------------------------------------------------

// SchemaValidator validates arguments against a JSON schema.
type SchemaValidator interface {
	Validate(schema json.RawMessage, data json.RawMessage) error
}

// WithValidation creates middleware that validates arguments against the tool's schema.
func WithValidation(validator SchemaValidator) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)

			// We need access to the tool's schema
			// This requires the tool to be stored in context or passed differently
			// For now, skip validation if we can't access schema
			if tc == nil || tc.Metadata == nil {
				return next(ctx, args)
			}

			schema, ok := tc.Metadata["schema"].(json.RawMessage)
			if !ok {
				return next(ctx, args)
			}

			if err := validator.Validate(schema, args); err != nil {
				return nil, fmt.Errorf("argument validation failed: %w", err)
			}

			return next(ctx, args)
		}
	}
}

// WithBasicValidation creates middleware that performs basic JSON validation.
func WithBasicValidation() Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			// Verify arguments are valid JSON
			if !json.Valid(args) {
				return nil, errors.New("invalid JSON arguments")
			}
			return next(ctx, args)
		}
	}
}

// -----------------------------------------------------------------------------
// Metrics Middleware
// -----------------------------------------------------------------------------

// MetricsCollector receives tool execution metrics.
type MetricsCollector interface {
	// RecordCall records a tool call with its outcome.
	RecordCall(toolName string, duration time.Duration, err error)
}

// WithMetrics creates middleware that records tool execution metrics.
func WithMetrics(collector MetricsCollector) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			toolName := "unknown"
			if tc != nil {
				toolName = tc.ToolName
			}

			start := time.Now()
			result, err := next(ctx, args)
			duration := time.Since(start)

			collector.RecordCall(toolName, duration, err)

			return result, err
		}
	}
}

// -----------------------------------------------------------------------------
// Retry Middleware
// -----------------------------------------------------------------------------

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
	Retryable   func(error) bool // Returns true if error is retryable
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
		Retryable: func(err error) bool {
			// Retry on common transient errors
			return errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(err.Error(), "temporary") ||
				strings.Contains(err.Error(), "timeout")
		},
	}
}

// WithRetry creates middleware that retries failed tool calls.
func WithRetry(config RetryConfig) Middleware {
	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			var lastErr error
			wait := config.InitialWait

			for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
				result, err := next(ctx, args)
				if err == nil {
					return result, nil
				}

				lastErr = err

				// Check if error is retryable
				if config.Retryable != nil && !config.Retryable(err) {
					return nil, err
				}

				// Don't wait after last attempt
				if attempt == config.MaxAttempts {
					break
				}

				// Wait with exponential backoff
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}

				wait = time.Duration(float64(wait) * config.Multiplier)
				if wait > config.MaxWait {
					wait = config.MaxWait
				}
			}

			return nil, fmt.Errorf("tool call failed after %d attempts: %w", config.MaxAttempts, lastErr)
		}
	}
}

// -----------------------------------------------------------------------------
// Circuit Breaker Middleware
// -----------------------------------------------------------------------------

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing, reject calls
	CircuitHalfOpen                     // Testing if recovered
)

// String returns the string representation of a CircuitState.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Failures before opening
	SuccessThreshold int           // Successes in half-open to close
	OpenDuration     time.Duration // How long to stay open
}

// DefaultCircuitBreakerConfig returns sensible circuit breaker defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		OpenDuration:     30 * time.Second,
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open: too many failures")

// WithCircuitBreaker creates middleware that implements the circuit breaker pattern.
func WithCircuitBreaker(config CircuitBreakerConfig) Middleware {
	var (
		mu          sync.Mutex
		state       CircuitState
		failures    int
		successes   int
		lastFailure time.Time
	)

	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			mu.Lock()

			// Check if circuit should transition from open to half-open
			if state == CircuitOpen && time.Since(lastFailure) > config.OpenDuration {
				state = CircuitHalfOpen
				successes = 0
			}

			// Reject if circuit is open
			if state == CircuitOpen {
				mu.Unlock()
				return nil, ErrCircuitOpen
			}

			mu.Unlock()

			// Execute tool
			result, err := next(ctx, args)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failures++
				lastFailure = time.Now()

				if state == CircuitHalfOpen {
					// Failure in half-open returns to open
					state = CircuitOpen
				} else if failures >= config.FailureThreshold {
					// Too many failures, open circuit
					state = CircuitOpen
				}

				return nil, err
			}

			// Success
			if state == CircuitHalfOpen {
				successes++
				if successes >= config.SuccessThreshold {
					// Enough successes, close circuit
					state = CircuitClosed
					failures = 0
				}
			} else {
				// Reset failure count on success in closed state
				failures = 0
			}

			return result, nil
		}
	}
}

// -----------------------------------------------------------------------------
// Conditional Middleware
// -----------------------------------------------------------------------------

// ForTools applies middleware only to tools with the specified names.
func ForTools(toolNames []string, middleware Middleware) Middleware {
	nameSet := make(map[string]bool)
	for _, name := range toolNames {
		nameSet[name] = true
	}

	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			if tc != nil && nameSet[tc.ToolName] {
				// Apply middleware
				return middleware(next)(ctx, args)
			}
			// Skip middleware
			return next(ctx, args)
		}
	}
}

// ExceptTools applies middleware to all tools except those with the specified names.
func ExceptTools(toolNames []string, middleware Middleware) Middleware {
	nameSet := make(map[string]bool)
	for _, name := range toolNames {
		nameSet[name] = true
	}

	return func(next ToolCallFunc) ToolCallFunc {
		return func(ctx context.Context, args json.RawMessage) (any, error) {
			tc := ToolContextFromContext(ctx)
			if tc == nil || !nameSet[tc.ToolName] {
				// Apply middleware
				return middleware(next)(ctx, args)
			}
			// Skip middleware
			return next(ctx, args)
		}
	}
}
