package anthropic

import (
	"errors"
	"testing"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      []byte
		requestID string
		wantErr   error
		wantMsg   string
		wantCode  string
	}{
		{
			name:      "400 bad request",
			status:    400,
			body:      []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Invalid request"}}`),
			requestID: "req_123",
			wantErr:   core.ErrBadRequest,
			wantMsg:   "Invalid request",
			wantCode:  "invalid_request_error",
		},
		{
			name:      "401 unauthorized",
			status:    401,
			body:      []byte(`{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`),
			requestID: "req_456",
			wantErr:   core.ErrUnauthorized,
			wantMsg:   "Invalid API key",
			wantCode:  "authentication_error",
		},
		{
			name:      "403 forbidden",
			status:    403,
			body:      []byte(`{"type":"error","error":{"type":"permission_error","message":"Not allowed"}}`),
			requestID: "",
			wantErr:   core.ErrUnauthorized,
			wantMsg:   "Not allowed",
			wantCode:  "permission_error",
		},
		{
			name:      "429 rate limited",
			status:    429,
			body:      []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Too many requests"}}`),
			requestID: "req_789",
			wantErr:   core.ErrRateLimited,
			wantMsg:   "Too many requests",
			wantCode:  "rate_limit_error",
		},
		{
			name:      "500 server error",
			status:    500,
			body:      []byte(`{"type":"error","error":{"type":"api_error","message":"Internal error"}}`),
			requestID: "req_abc",
			wantErr:   core.ErrServer,
			wantMsg:   "Internal error",
			wantCode:  "api_error",
		},
		{
			name:      "503 overloaded",
			status:    503,
			body:      []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Service overloaded"}}`),
			requestID: "",
			wantErr:   core.ErrServer,
			wantMsg:   "Service overloaded",
			wantCode:  "overloaded_error",
		},
		{
			name:      "invalid json body",
			status:    500,
			body:      []byte(`not json`),
			requestID: "",
			wantErr:   core.ErrServer,
			wantMsg:   "Internal Server Error",
			wantCode:  "unknown_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := normalizeError(tt.status, tt.body, tt.requestID)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}

			var provErr *core.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatal("error should be ProviderError")
			}

			if provErr.Provider != "anthropic" {
				t.Errorf("Provider = %q, want 'anthropic'", provErr.Provider)
			}

			if provErr.Status != tt.status {
				t.Errorf("Status = %d, want %d", provErr.Status, tt.status)
			}

			if provErr.RequestID != tt.requestID {
				t.Errorf("RequestID = %q, want %q", provErr.RequestID, tt.requestID)
			}

			if provErr.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", provErr.Message, tt.wantMsg)
			}

			if provErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", provErr.Code, tt.wantCode)
			}
		})
	}
}

func TestNewNetworkError(t *testing.T) {
	origErr := errors.New("connection refused")
	err := newNetworkError(origErr)

	if !errors.Is(err, core.ErrNetwork) {
		t.Errorf("error = %v, want ErrNetwork", err)
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("error should be ProviderError")
	}

	if provErr.Provider != "anthropic" {
		t.Errorf("Provider = %q, want 'anthropic'", provErr.Provider)
	}

	if provErr.Message != "connection refused" {
		t.Errorf("Message = %q, want 'connection refused'", provErr.Message)
	}
}

func TestNewDecodeError(t *testing.T) {
	origErr := errors.New("unexpected EOF")
	err := newDecodeError(origErr)

	if !errors.Is(err, core.ErrDecode) {
		t.Errorf("error = %v, want ErrDecode", err)
	}

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatal("error should be ProviderError")
	}

	if provErr.Provider != "anthropic" {
		t.Errorf("Provider = %q, want 'anthropic'", provErr.Provider)
	}

	if provErr.Message != "unexpected EOF" {
		t.Errorf("Message = %q, want 'unexpected EOF'", provErr.Message)
	}
}

func TestErrFileNotDownloadable(t *testing.T) {
	if ErrFileNotDownloadable == nil {
		t.Error("ErrFileNotDownloadable should not be nil")
	}
	if ErrFileNotDownloadable.Error() != "file not downloadable" {
		t.Errorf("unexpected error message: %s", ErrFileNotDownloadable.Error())
	}
}

func TestNormalizeError404(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"not_found_error","message":"File not found"}}`)
	err := normalizeError(404, body, "req-123")

	var provErr *core.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !errors.Is(provErr, core.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", provErr.Err)
	}
}
