package azurefoundry

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func TestNormalizeError400(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid model","type":"invalid_request_error","code":"invalid_model"}}`)
	err := normalizeError(400, body, "req-123")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if pErr.Status != 400 {
		t.Errorf("Status = %d, want 400", pErr.Status)
	}

	if pErr.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", pErr.RequestID, "req-123")
	}

	if pErr.Message != "Invalid model" {
		t.Errorf("Message = %q, want %q", pErr.Message, "Invalid model")
	}

	if !errors.Is(err, ErrModelNotFound) {
		t.Error("expected error to wrap ErrModelNotFound")
	}
}

func TestNormalizeError401(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid API key","code":"invalid_api_key"}}`)
	err := normalizeError(401, body, "")

	if !errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("expected error to wrap ErrInvalidAPIKey, got %v", err)
	}
}

func TestNormalizeError401Generic(t *testing.T) {
	body := []byte(`{"error":{"message":"Unauthorized"}}`)
	err := normalizeError(401, body, "")

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Error("expected error to wrap ErrUnauthorized")
	}
}

func TestNormalizeError403(t *testing.T) {
	body := []byte(`{"error":{"message":"Access denied"}}`)
	err := normalizeError(403, body, "")

	if !errors.Is(err, core.ErrUnauthorized) {
		t.Error("expected error to wrap ErrUnauthorized")
	}
}

func TestNormalizeError429(t *testing.T) {
	body := []byte(`{"error":{"message":"Rate limit exceeded","code":"quota_exceeded"}}`)
	err := normalizeError(429, body, "req-456")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if !errors.Is(err, ErrQuotaExceeded) {
		t.Errorf("expected error to wrap ErrQuotaExceeded, got %v", err)
	}

	if pErr.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want %q", pErr.RequestID, "req-456")
	}
}

func TestNormalizeError500(t *testing.T) {
	body := []byte(`{"error":{"message":"Internal server error"}}`)
	err := normalizeError(500, body, "")

	if !errors.Is(err, core.ErrServer) {
		t.Error("expected error to wrap ErrServer")
	}
}

func TestNormalizeError502(t *testing.T) {
	body := []byte(`{}`)
	err := normalizeError(502, body, "")

	if !errors.Is(err, core.ErrServer) {
		t.Error("expected error to wrap ErrServer")
	}
}

func TestNormalizeError503(t *testing.T) {
	body := []byte(`{"error":{"message":"Service unavailable"}}`)
	err := normalizeError(503, body, "")

	if !errors.Is(err, core.ErrServer) {
		t.Error("expected error to wrap ErrServer")
	}
}

func TestNormalizeErrorContentFilter(t *testing.T) {
	body := []byte(`{"error":{"message":"Content filtered","code":"content_filter"}}`)
	err := normalizeError(400, body, "")

	if !errors.Is(err, ErrContentFiltered) {
		t.Errorf("expected error to wrap ErrContentFiltered, got %v", err)
	}
}

func TestNormalizeErrorDeploymentNotFound(t *testing.T) {
	body := []byte(`{"error":{"message":"Deployment not found","code":"DeploymentNotFound"}}`)
	err := normalizeError(404, body, "")

	if !errors.Is(err, ErrDeploymentNotFound) {
		t.Errorf("expected error to wrap ErrDeploymentNotFound, got %v", err)
	}
}

func TestNormalizeErrorContextLength(t *testing.T) {
	body := []byte(`{"error":{"message":"Context too long","code":"context_length_exceeded"}}`)
	err := normalizeError(400, body, "")

	if !errors.Is(err, ErrContextLengthExceeded) {
		t.Errorf("expected error to wrap ErrContextLengthExceeded, got %v", err)
	}
}

func TestNormalizeErrorInnerCode(t *testing.T) {
	body := []byte(`{"error":{"message":"Policy violation","code":"content_filter","innererror":{"code":"ResponsibleAIPolicyViolation"}}}`)
	err := normalizeError(400, body, "")

	if !errors.Is(err, ErrContentFiltered) {
		t.Errorf("expected error to wrap ErrContentFiltered, got %v", err)
	}
}

func TestNormalizeErrorTokenExpired(t *testing.T) {
	body := []byte(`{"error":{"message":"Token expired","innererror":{"code":"TokenExpired"}}}`)
	err := normalizeError(401, body, "")

	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected error to wrap ErrTokenExpired, got %v", err)
	}
}

func TestNormalizeErrorProvider(t *testing.T) {
	err := normalizeError(400, []byte(`{}`), "")

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if pErr.Provider != "azurefoundry" {
		t.Errorf("Provider = %q, want %q", pErr.Provider, "azurefoundry")
	}
}

func TestNewNetworkError(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := newNetworkError(originalErr)

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if pErr.Provider != "azurefoundry" {
		t.Errorf("Provider = %q, want %q", pErr.Provider, "azurefoundry")
	}

	if !errors.Is(err, core.ErrNetwork) {
		t.Error("expected error to wrap ErrNetwork")
	}
}

func TestNewDecodeError(t *testing.T) {
	originalErr := errors.New("unexpected EOF")
	err := newDecodeError(originalErr)

	var pErr *core.ProviderError
	if !errors.As(err, &pErr) {
		t.Fatal("expected ProviderError")
	}

	if !errors.Is(err, core.ErrDecode) {
		t.Error("expected error to wrap ErrDecode")
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	headers := http.Header{}
	headers.Set("Retry-After", "30")

	d := parseRetryAfter(headers)

	if d != 30*time.Second {
		t.Errorf("parseRetryAfter() = %v, want 30s", d)
	}
}

func TestParseRetryAfterDuration(t *testing.T) {
	headers := http.Header{}
	headers.Set("Retry-After", "1m30s")

	d := parseRetryAfter(headers)

	if d != 90*time.Second {
		t.Errorf("parseRetryAfter() = %v, want 1m30s", d)
	}
}

func TestParseRetryAfterAzureMs(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-ms-retry-after-ms", "5000")

	d := parseRetryAfter(headers)

	if d != 5*time.Second {
		t.Errorf("parseRetryAfter() = %v, want 5s", d)
	}
}

func TestParseRetryAfterOpenAIStyle(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-ratelimit-reset-requests", "60")

	d := parseRetryAfter(headers)

	if d != 60*time.Second {
		t.Errorf("parseRetryAfter() = %v, want 60s", d)
	}
}

func TestParseRetryAfterNone(t *testing.T) {
	headers := http.Header{}

	d := parseRetryAfter(headers)

	if d != 0 {
		t.Errorf("parseRetryAfter() = %v, want 0", d)
	}
}

func TestContentFilterError(t *testing.T) {
	filters := &azureContentFilters{
		Hate: &azureFilterSeverity{Filtered: true, Severity: "high"},
	}

	err := newContentFilterError(filters)

	if err.Category != "hate" {
		t.Errorf("Category = %q, want hate", err.Category)
	}

	if err.Severity != "high" {
		t.Errorf("Severity = %q, want high", err.Severity)
	}

	if !errors.Is(err, ErrContentFiltered) {
		t.Error("expected error to wrap ErrContentFiltered")
	}

	// Check error message
	msg := err.Error()
	if msg == "" {
		t.Error("Error() returned empty string")
	}
}

func TestContentFilterErrorCategories(t *testing.T) {
	filters := &azureContentFilters{
		Hate:     &azureFilterSeverity{Filtered: true, Severity: "high"},
		Violence: &azureFilterSeverity{Filtered: true, Severity: "medium"},
	}

	err := newContentFilterError(filters)
	categories := err.FilteredCategories()

	if len(categories) != 2 {
		t.Fatalf("len(FilteredCategories()) = %d, want 2", len(categories))
	}

	hasHate := false
	hasViolence := false
	for _, c := range categories {
		if c == "hate" {
			hasHate = true
		}
		if c == "violence" {
			hasViolence = true
		}
	}

	if !hasHate {
		t.Error("FilteredCategories() missing hate")
	}
	if !hasViolence {
		t.Error("FilteredCategories() missing violence")
	}
}

func TestContentFilterErrorNilFilters(t *testing.T) {
	err := newContentFilterError(nil)

	if err.Category != "" {
		t.Errorf("Category = %q, want empty", err.Category)
	}

	categories := err.FilteredCategories()
	if categories != nil {
		t.Errorf("FilteredCategories() = %v, want nil", categories)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"rate limited", core.ErrRateLimited, true},
		{"quota exceeded", ErrQuotaExceeded, true},
		{"server error", core.ErrServer, true},
		{"network error", core.ErrNetwork, true},
		{"token expired", ErrTokenExpired, true},
		{"bad request", core.ErrBadRequest, false},
		{"content filtered", ErrContentFiltered, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unauthorized", core.ErrUnauthorized, true},
		{"invalid api key", ErrInvalidAPIKey, true},
		{"token expired", ErrTokenExpired, true},
		{"bad request", core.ErrBadRequest, false},
		{"rate limited", core.ErrRateLimited, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuthError(tt.err)
			if got != tt.want {
				t.Errorf("IsAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsContentFilterError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"content filtered", ErrContentFiltered, true},
		{"bad request", core.ErrBadRequest, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsContentFilterError(tt.err)
			if got != tt.want {
				t.Errorf("IsContentFilterError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestClassifyErrorCode(t *testing.T) {
	tests := []struct {
		code      string
		innerCode string
		status    int
		want      error
	}{
		{"content_filter", "", 400, ErrContentFiltered},
		{"DeploymentNotFound", "", 404, ErrDeploymentNotFound},
		{"model_not_found", "", 404, ErrModelNotFound},
		{"quota_exceeded", "", 429, ErrQuotaExceeded},
		{"context_length_exceeded", "", 400, ErrContextLengthExceeded},
		{"invalid_api_key", "", 401, ErrInvalidAPIKey},
		{"", "ResponsibleAIPolicyViolation", 400, ErrContentFiltered},
		{"", "TokenExpired", 401, ErrTokenExpired},
		{"unknown", "", 400, core.ErrBadRequest},
		{"unknown", "", 500, core.ErrServer},
	}

	for _, tt := range tests {
		t.Run(tt.code+"_"+tt.innerCode, func(t *testing.T) {
			got := classifyErrorCode(tt.code, tt.innerCode, tt.status)
			if !errors.Is(got, tt.want) {
				t.Errorf("classifyErrorCode(%q, %q, %d) = %v, want %v", tt.code, tt.innerCode, tt.status, got, tt.want)
			}
		})
	}
}

func TestRetryableStatus(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := retryableStatus(tt.status)
			if got != tt.want {
				t.Errorf("retryableStatus(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	baseDelay := time.Second

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // capped
		{6, 30 * time.Second}, // capped
	}

	for _, tt := range tests {
		t.Run("attempt_"+string(rune('0'+tt.attempt)), func(t *testing.T) {
			got := calculateBackoff(tt.attempt, baseDelay)
			if got != tt.want {
				t.Errorf("calculateBackoff(%d, %v) = %v, want %v", tt.attempt, baseDelay, got, tt.want)
			}
		})
	}
}
