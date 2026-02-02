package gemini

import (
	"errors"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeError(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantSentinel error
	}{
		{
			name:         "bad request",
			status:       400,
			body:         `{"error":{"code":400,"message":"Invalid argument","status":"INVALID_ARGUMENT"}}`,
			wantSentinel: core.ErrBadRequest,
		},
		{
			name:         "unauthorized",
			status:       401,
			body:         `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`,
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "forbidden",
			status:       403,
			body:         `{"error":{"code":403,"message":"Permission denied","status":"PERMISSION_DENIED"}}`,
			wantSentinel: core.ErrUnauthorized,
		},
		{
			name:         "not found",
			status:       404,
			body:         `{"error":{"code":404,"message":"Not found","status":"NOT_FOUND"}}`,
			wantSentinel: core.ErrBadRequest,
		},
		{
			name:         "rate limited",
			status:       429,
			body:         `{"error":{"code":429,"message":"Resource exhausted","status":"RESOURCE_EXHAUSTED"}}`,
			wantSentinel: core.ErrRateLimited,
		},
		{
			name:         "server error",
			status:       500,
			body:         `{"error":{"code":500,"message":"Internal error","status":"INTERNAL"}}`,
			wantSentinel: core.ErrServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := normalizeError(tt.status, []byte(tt.body))

			var provErr *core.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error is not ProviderError: %v", err)
			}

			if !errors.Is(provErr.Err, tt.wantSentinel) {
				t.Errorf("sentinel = %v, want %v", provErr.Err, tt.wantSentinel)
			}

			if provErr.Provider != "gemini" {
				t.Errorf("Provider = %q, want 'gemini'", provErr.Provider)
			}
		})
	}
}

func TestNewNetworkError(t *testing.T) {
	origErr := errors.New("connection refused")
	err := newNetworkError(origErr)

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("error is not ProviderError: %v", err)
	}

	if !errors.Is(provErr.Err, core.ErrNetwork) {
		t.Errorf("sentinel = %v, want ErrNetwork", provErr.Err)
	}
}

func TestNewDecodeError(t *testing.T) {
	origErr := errors.New("invalid json")
	err := newDecodeError(origErr)

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("error is not ProviderError: %v", err)
	}

	if !errors.Is(provErr.Err, core.ErrDecode) {
		t.Errorf("sentinel = %v, want ErrDecode", provErr.Err)
	}
}
