package normalize

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestOpenAIStyleProviderError(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         []byte
		requestID    string
		wantCode     string
		wantMsg      string
		wantSentinel error
	}{
		{
			name:         "bad request",
			status:       http.StatusBadRequest,
			body:         []byte(`{"error":{"message":"Invalid model","type":"invalid_request_error","code":"invalid_model"}}`),
			requestID:    "req-123",
			wantCode:     "invalid_model",
			wantMsg:      "Invalid model",
			wantSentinel: core.ErrBadRequest,
		},
		{
			name:         "fallback to type",
			status:       http.StatusUnauthorized,
			body:         []byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`),
			requestID:    "req-456",
			wantCode:     "authentication_error",
			wantMsg:      "Invalid API key",
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "fallback to status text",
			status:       http.StatusBadGateway,
			body:         []byte{},
			requestID:    "",
			wantCode:     "",
			wantMsg:      "Bad Gateway",
			wantSentinel: core.ErrServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := OpenAIStyleProviderError("test-provider", tt.status, tt.body, tt.requestID)

			var provErr *core.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatal("expected *core.ProviderError")
			}

			if provErr.Provider != "test-provider" {
				t.Errorf("Provider = %q, want test-provider", provErr.Provider)
			}
			if provErr.Status != tt.status {
				t.Errorf("Status = %d, want %d", provErr.Status, tt.status)
			}
			if provErr.RequestID != tt.requestID {
				t.Errorf("RequestID = %q, want %q", provErr.RequestID, tt.requestID)
			}
			if provErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", provErr.Code, tt.wantCode)
			}
			if provErr.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", provErr.Message, tt.wantMsg)
			}
			if !errors.Is(err, tt.wantSentinel) {
				t.Errorf("error should wrap %v", tt.wantSentinel)
			}
		})
	}
}

func TestOpenAIStyleErrorFallsBackToBody(t *testing.T) {
	// Body is valid text but NOT the {"error":{"message"}} envelope.
	body := []byte(`{"detail":"model gpt-x does not exist"}`)
	err := OpenAIStyleProviderError("openai", 404, body, "req-1")
	var pe *core.ProviderError
	if !errors.As(err, &pe) {
		t.Fatal("want *core.ProviderError")
	}
	if !strings.Contains(pe.Message+pe.Body, "does not exist") {
		t.Errorf("real body text lost: msg=%q body=%q", pe.Message, pe.Body)
	}
}

func TestNetworkError(t *testing.T) {
	err := NetworkError("test-provider", errors.New("connection refused"))

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if provErr.Provider != "test-provider" {
		t.Errorf("Provider = %q, want test-provider", provErr.Provider)
	}
	if provErr.Message != "connection refused" {
		t.Errorf("Message = %q, want connection refused", provErr.Message)
	}
	if !errors.Is(err, core.ErrNetwork) {
		t.Error("error should wrap core.ErrNetwork")
	}
}

func TestDecodeError(t *testing.T) {
	err := DecodeError("test-provider", errors.New("unexpected EOF"))

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if provErr.Provider != "test-provider" {
		t.Errorf("Provider = %q, want test-provider", provErr.Provider)
	}
	if provErr.Message != "unexpected EOF" {
		t.Errorf("Message = %q, want unexpected EOF", provErr.Message)
	}
	if !errors.Is(err, core.ErrDecode) {
		t.Error("error should wrap core.ErrDecode")
	}
}

func TestProviderErrorDefaults(t *testing.T) {
	err := ProviderError("test-provider", http.StatusBadGateway, "req-1", "", "", nil)

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if provErr.Message != "Bad Gateway" {
		t.Errorf("Message = %q, want Bad Gateway", provErr.Message)
	}
	if !errors.Is(err, core.ErrServer) {
		t.Error("error should wrap core.ErrServer")
	}
}

func TestWithRetryAfterAttachesServerDelay(t *testing.T) {
	err := ProviderError("test-provider", http.StatusTooManyRequests, "", "", "busy", core.ErrRateLimited)
	wrapped := WithRetryAfter(err, http.Header{"Retry-After": []string{"30"}})

	var providerErr *core.ProviderError
	if !errors.As(wrapped, &providerErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if providerErr.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v, want 30s", providerErr.RetryAfter)
	}
}

func TestWithRetryAfterSupportsHTTPDate(t *testing.T) {
	err := ProviderError("test-provider", http.StatusTooManyRequests, "", "", "busy", core.ErrRateLimited)
	when := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
	wrapped := WithRetryAfter(err, http.Header{"Retry-After": []string{when}})

	var providerErr *core.ProviderError
	if !errors.As(wrapped, &providerErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if providerErr.RetryAfter <= 0 || providerErr.RetryAfter > 2*time.Second {
		t.Fatalf("RetryAfter = %v, want a positive delay no greater than 2s", providerErr.RetryAfter)
	}
}

func TestWithRetryAfterNoopCases(t *testing.T) {
	if WithRetryAfter(nil, http.Header{"Retry-After": []string{"30"}}) != nil {
		t.Fatal("nil error should remain nil")
	}

	original := errors.New("not a provider error")
	if got := WithRetryAfter(original, http.Header{"Retry-After": []string{"30"}}); got != original {
		t.Fatal("non-provider error should remain unchanged")
	}

	providerErr := ProviderError("test-provider", http.StatusTooManyRequests, "", "", "busy", core.ErrRateLimited)
	if got := WithRetryAfter(providerErr); got != providerErr {
		t.Fatal("provider error without headers should remain unchanged")
	}
}

func TestWithRetryAfterSupportsProviderHeaders(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		value   string
		wantMin time.Duration
	}{
		{name: "azure milliseconds", header: "x-ms-retry-after-ms", value: "1500", wantMin: 1500 * time.Millisecond},
		{name: "reset duration", header: "x-ratelimit-reset-requests", value: "2s", wantMin: 2 * time.Second},
		{name: "token reset duration", header: "x-ratelimit-reset-tokens", value: "3s", wantMin: 3 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProviderError("test-provider", http.StatusTooManyRequests, "", "", "busy", core.ErrRateLimited)
			headers := http.Header{}
			headers.Set(tt.header, tt.value)
			WithRetryAfter(err, headers)

			var providerErr *core.ProviderError
			if !errors.As(err, &providerErr) {
				t.Fatal("expected *core.ProviderError")
			}
			if providerErr.RetryAfter != tt.wantMin {
				t.Fatalf("RetryAfter = %v, want %v", providerErr.RetryAfter, tt.wantMin)
			}
		})
	}
}

func TestWithRetryAfterIgnoresInvalidHints(t *testing.T) {
	err := ProviderError("test-provider", http.StatusTooManyRequests, "", "", "busy", core.ErrRateLimited)
	WithRetryAfter(err, http.Header{"Retry-After": []string{"not-a-delay"}})

	var providerErr *core.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatal("expected *core.ProviderError")
	}
	if providerErr.RetryAfter != 0 {
		t.Fatalf("RetryAfter = %v, want 0", providerErr.RetryAfter)
	}

	WithRetryAfter(err, http.Header{"Retry-After": []string{"1.5s"}})
	if providerErr.RetryAfter != 1500*time.Millisecond {
		t.Fatalf("duration RetryAfter = %v, want 1.5s", providerErr.RetryAfter)
	}

	WithRetryAfter(err, http.Header{"Retry-After": []string{time.Unix(1, 0).UTC().Format(http.TimeFormat)}})
	if providerErr.RetryAfter != 0 {
		t.Fatalf("expired RetryAfter = %v, want 0", providerErr.RetryAfter)
	}
}

func TestNetworkErrorPreservesChain(t *testing.T) {
	underlying := fmt.Errorf("dial tcp: %w", context.DeadlineExceeded)
	err := NetworkError("openai", underlying)
	if !errors.Is(err, core.ErrNetwork) {
		t.Error("want errors.Is(err, core.ErrNetwork)")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("want errors.Is(err, context.DeadlineExceeded)")
	}
}

func TestDecodeErrorPreservesChain(t *testing.T) {
	underlying := errors.New("invalid character '<'")
	err := DecodeError("openai", underlying)
	if !errors.Is(err, core.ErrDecode) {
		t.Error("want errors.Is(err, core.ErrDecode)")
	}
	if !errors.Is(err, underlying) {
		t.Error("want errors.Is(err, underlying)")
	}
}

func TestSentinelForStatusWithOverrides(t *testing.T) {
	sentinel := SentinelForStatusWithOverrides(http.StatusNotFound, map[int]error{
		http.StatusNotFound: core.ErrNotFound,
	})
	if !errors.Is(sentinel, core.ErrNotFound) {
		t.Errorf("sentinel = %v, want ErrNotFound", sentinel)
	}
}

func TestSentinelForStatusNotFound(t *testing.T) {
	sentinel := SentinelForStatus(http.StatusNotFound)
	if !errors.Is(sentinel, core.ErrNotFound) {
		t.Errorf("sentinel = %v, want ErrNotFound", sentinel)
	}
}

func TestRequireAPIKey(t *testing.T) {
	if err := RequireAPIKey("acme", core.NewSecret("sk-x")); err != nil {
		t.Fatalf("non-empty key should pass: %v", err)
	}
	err := RequireAPIKey("acme", core.NewSecret("  "))
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("empty key err = %v, want ErrUnauthorized", err)
	}
	var pe *core.ProviderError
	if !errors.As(err, &pe) || pe.Provider != "acme" {
		t.Errorf("want *core.ProviderError with Provider=acme, got %#v", err)
	}
}
