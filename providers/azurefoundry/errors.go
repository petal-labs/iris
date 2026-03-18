package azurefoundry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
)

// Sentinel errors for Azure AI Foundry.
var (
	// ErrContentFiltered is returned when Azure's content safety filters block content.
	ErrContentFiltered = errors.New("azurefoundry: content filtered by safety policy")

	// ErrToolArgsInvalidJSON is returned when tool call arguments contain invalid JSON.
	ErrToolArgsInvalidJSON = errors.New("azurefoundry: tool arguments are not valid JSON")

	// ErrDeploymentNotFound is returned when the specified deployment does not exist.
	ErrDeploymentNotFound = errors.New("azurefoundry: deployment not found")

	// ErrModelNotFound is returned when the specified model is not available.
	ErrModelNotFound = errors.New("azurefoundry: model not found")

	// ErrQuotaExceeded is returned when the account quota has been exceeded.
	ErrQuotaExceeded = errors.New("azurefoundry: quota exceeded")

	// ErrContextLengthExceeded is returned when the request exceeds the model's context length.
	ErrContextLengthExceeded = errors.New("azurefoundry: context length exceeded")

	// ErrInvalidAPIKey is returned when the API key is invalid or expired.
	ErrInvalidAPIKey = errors.New("azurefoundry: invalid API key")

	// ErrTokenExpired is returned when the Entra ID token has expired.
	ErrTokenExpired = errors.New("azurefoundry: token expired")
)

// Azure-specific error codes.
const (
	// Content safety error codes
	codeContentFilter          = "content_filter"
	codeResponsibleAIViolation = "ResponsibleAIPolicyViolation"
	codeContentFilterResult    = "content_filter_result"

	// Deployment and model error codes
	codeDeploymentNotFound = "DeploymentNotFound"
	codeModelNotFound      = "model_not_found"
	codeInvalidModel       = "invalid_model"

	// Quota and limit error codes
	codeQuotaExceeded         = "quota_exceeded"
	codeTokensPerMinute       = "tokens_per_minute"
	codeRequestsPerMinute     = "requests_per_minute"
	codeContextLengthExceeded = "context_length_exceeded"
	codeMaxTokensExceeded     = "max_tokens_exceeded"

	// Authentication error codes
	codeInvalidAPIKey       = "invalid_api_key"
	codeInvalidSubscription = "invalid_subscription_key"
	codeTokenExpired        = "TokenExpired"
	codeInvalidToken        = "InvalidAuthenticationToken"
)

// azureErrorResponse represents an Azure API error response.
type azureErrorResponse struct {
	Error azureErrorDetail `json:"error"`
}

// azureErrorDetail contains the error details.
type azureErrorDetail struct {
	Message    string           `json:"message"`
	Type       string           `json:"type"`
	Code       string           `json:"code"`
	Param      string           `json:"param,omitempty"`
	InnerError *azureInnerError `json:"innererror,omitempty"`
}

// azureInnerError contains additional error details.
type azureInnerError struct {
	Code                 string                 `json:"code"`
	Message              string                 `json:"message"`
	ContentFilterResults map[string]interface{} `json:"content_filter_result,omitempty"`
}

// normalizeError converts an HTTP error response to a ProviderError with the appropriate sentinel.
func normalizeError(status int, body []byte, requestID string) error {
	var errResp azureErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// Could not parse error response, use generic handling
		return normalize.OpenAIStyleProviderError("azurefoundry", status, body, requestID)
	}

	detail := errResp.Error
	code := detail.Code
	message := detail.Message

	// Get inner error code if available
	innerCode := ""
	if detail.InnerError != nil {
		innerCode = detail.InnerError.Code
		if message == "" && detail.InnerError.Message != "" {
			message = detail.InnerError.Message
		}
	}

	// Classify error based on code
	sentinel := classifyErrorCode(code, innerCode, status)

	return &core.ProviderError{
		Provider:  "azurefoundry",
		Status:    status,
		RequestID: requestID,
		Code:      code,
		Message:   message,
		Err:       sentinel,
	}
}

// classifyErrorCode maps Azure error codes to sentinel errors.
func classifyErrorCode(code, innerCode string, status int) error {
	// Check inner code first for more specific errors
	switch innerCode {
	case codeResponsibleAIViolation:
		return ErrContentFiltered
	case codeTokenExpired, codeInvalidToken:
		return ErrTokenExpired
	}

	// Check main code
	switch code {
	// Content filtering
	case codeContentFilter, codeContentFilterResult:
		return ErrContentFiltered

	// Deployment and model errors
	case codeDeploymentNotFound:
		return ErrDeploymentNotFound
	case codeModelNotFound, codeInvalidModel:
		return ErrModelNotFound

	// Quota and limits
	case codeQuotaExceeded, codeTokensPerMinute, codeRequestsPerMinute:
		return ErrQuotaExceeded
	case codeContextLengthExceeded, codeMaxTokensExceeded:
		return ErrContextLengthExceeded

	// Authentication
	case codeInvalidAPIKey, codeInvalidSubscription:
		return ErrInvalidAPIKey
	}

	// Fall back to status-based classification
	return normalize.SentinelForStatus(status)
}

// parseRetryAfter extracts the retry-after duration from response headers.
// Returns 0 if no retry-after header is present.
func parseRetryAfter(headers http.Header) time.Duration {
	// Try Retry-After header (standard)
	if val := headers.Get("Retry-After"); val != "" {
		return parseRetryAfterValue(val)
	}

	// Try x-ms-retry-after-ms (Azure-specific, in milliseconds)
	if val := headers.Get("x-ms-retry-after-ms"); val != "" {
		if ms, err := strconv.ParseInt(val, 10, 64); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}

	// Try x-ratelimit-reset-requests or x-ratelimit-reset-tokens (OpenAI-style)
	if val := headers.Get("x-ratelimit-reset-requests"); val != "" {
		return parseRetryAfterValue(val)
	}
	if val := headers.Get("x-ratelimit-reset-tokens"); val != "" {
		return parseRetryAfterValue(val)
	}

	return 0
}

// parseRetryAfterValue parses a retry-after value which can be seconds or a duration string.
func parseRetryAfterValue(val string) time.Duration {
	// Try parsing as seconds (integer)
	if seconds, err := strconv.ParseInt(val, 10, 64); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as duration string (e.g., "1m30s")
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}

	// Try parsing as duration with 's' suffix (e.g., "30s")
	val = strings.TrimSuffix(val, "s")
	if seconds, err := strconv.ParseFloat(val, 64); err == nil {
		return time.Duration(seconds * float64(time.Second))
	}

	return 0
}

// newNetworkError creates a ProviderError for network-related failures.
func newNetworkError(err error) error {
	return normalize.NetworkError("azurefoundry", err)
}

// newDecodeError creates a ProviderError for JSON decode failures.
func newDecodeError(err error) error {
	return normalize.DecodeError("azurefoundry", err)
}

// ContentFilterError wraps content filter information into an error.
type ContentFilterError struct {
	Filters  *azureContentFilters
	Category string // The primary category that triggered filtering
	Severity string // The severity level if available
}

// Error implements the error interface.
func (e *ContentFilterError) Error() string {
	if e.Category != "" {
		if e.Severity != "" {
			return fmt.Sprintf("content was filtered by Azure AI safety policy: %s (severity: %s)", e.Category, e.Severity)
		}
		return fmt.Sprintf("content was filtered by Azure AI safety policy: %s", e.Category)
	}
	return "content was filtered by Azure AI safety policy"
}

// Unwrap returns the underlying sentinel error.
func (e *ContentFilterError) Unwrap() error {
	return ErrContentFiltered
}

// FilteredCategories returns a list of categories that triggered filtering.
func (e *ContentFilterError) FilteredCategories() []string {
	if e.Filters == nil {
		return nil
	}

	var categories []string

	if e.Filters.Hate != nil && e.Filters.Hate.Filtered {
		categories = append(categories, "hate")
	}
	if e.Filters.SelfHarm != nil && e.Filters.SelfHarm.Filtered {
		categories = append(categories, "self_harm")
	}
	if e.Filters.Sexual != nil && e.Filters.Sexual.Filtered {
		categories = append(categories, "sexual")
	}
	if e.Filters.Violence != nil && e.Filters.Violence.Filtered {
		categories = append(categories, "violence")
	}
	if e.Filters.Jailbreak != nil && e.Filters.Jailbreak.Filtered {
		categories = append(categories, "jailbreak")
	}
	if e.Filters.ProtectedMaterialText != nil && e.Filters.ProtectedMaterialText.Filtered {
		categories = append(categories, "protected_material_text")
	}
	if e.Filters.ProtectedMaterialCode != nil && e.Filters.ProtectedMaterialCode.Filtered {
		categories = append(categories, "protected_material_code")
	}

	return categories
}

// newContentFilterError creates a ContentFilterError with category details.
func newContentFilterError(filters *azureContentFilters) *ContentFilterError {
	err := &ContentFilterError{Filters: filters}

	if filters == nil {
		return err
	}

	// Determine the primary category and severity
	if filters.Hate != nil && filters.Hate.Filtered {
		err.Category = "hate"
		err.Severity = filters.Hate.Severity
	} else if filters.Violence != nil && filters.Violence.Filtered {
		err.Category = "violence"
		err.Severity = filters.Violence.Severity
	} else if filters.Sexual != nil && filters.Sexual.Filtered {
		err.Category = "sexual"
		err.Severity = filters.Sexual.Severity
	} else if filters.SelfHarm != nil && filters.SelfHarm.Filtered {
		err.Category = "self_harm"
		err.Severity = filters.SelfHarm.Severity
	} else if filters.Jailbreak != nil && filters.Jailbreak.Filtered {
		err.Category = "jailbreak"
	} else if filters.ProtectedMaterialText != nil && filters.ProtectedMaterialText.Filtered {
		err.Category = "protected_material_text"
	} else if filters.ProtectedMaterialCode != nil && filters.ProtectedMaterialCode.Filtered {
		err.Category = "protected_material_code"
	}

	return err
}

// IsRetryable returns true if the error indicates a retryable condition.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for rate limiting
	if errors.Is(err, core.ErrRateLimited) {
		return true
	}

	// Check for quota exceeded (may become available later)
	if errors.Is(err, ErrQuotaExceeded) {
		return true
	}

	// Check for server errors
	if errors.Is(err, core.ErrServer) {
		return true
	}

	// Check for network errors
	if errors.Is(err, core.ErrNetwork) {
		return true
	}

	// Check for expired token (can be refreshed)
	if errors.Is(err, ErrTokenExpired) {
		return true
	}

	return false
}

// IsAuthError returns true if the error is related to authentication.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, core.ErrUnauthorized) ||
		errors.Is(err, ErrInvalidAPIKey) ||
		errors.Is(err, ErrTokenExpired)
}

// IsContentFilterError returns true if the error is related to content filtering.
func IsContentFilterError(err error) bool {
	return errors.Is(err, ErrContentFiltered)
}
