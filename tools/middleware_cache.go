package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

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

			// Check cache.
			if cached, ok := cache.Get(key); ok {
				return cached, nil
			}

			// Execute tool.
			result, err := next(ctx, args)
			if err != nil {
				return nil, err
			}

			// Cache successful result.
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
