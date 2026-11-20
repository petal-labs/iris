package elevenlabs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/internal/normalize"
)

const (
	providerID        = "elevenlabs"
	maxBodyLen        = 4096
	maxErrorBodyBytes = 64 * 1024
)

var statusOverrides = map[int]error{
	http.StatusUnprocessableEntity: core.ErrBadRequest,
}

func truncateBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxBodyLen {
		return s[:maxBodyLen] + "…(truncated)"
	}
	return s
}

type elevenLabsErrorEnvelope struct {
	Detail json.RawMessage `json:"detail"`
}

type elevenLabsDetailObject struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Param     string `json:"param"`
}

type elevenLabsValidationItem struct {
	Loc  []any  `json:"loc"`
	Msg  string `json:"msg"`
	Type string `json:"type"`
}

// normalizeError converts an ElevenLabs HTTP error response into a typed
// [*core.ProviderError] wrapping the appropriate core sentinel error, with
// request ID and Retry-After metadata attached when available.
func normalizeError(status int, body []byte, requestID string, headers ...http.Header) error {
	var code, message string

	var env elevenLabsErrorEnvelope
	if len(body) > 0 && json.Unmarshal(body, &env) == nil && len(env.Detail) > 0 {
		var detailObj elevenLabsDetailObject
		var detailStr string
		var detailList []elevenLabsValidationItem

		switch {
		case json.Unmarshal(env.Detail, &detailObj) == nil && (detailObj.Message != "" || detailObj.Code != "" || detailObj.Status != "" || detailObj.Type != ""):
			message = detailObj.Message
			code = detailObj.Code
			if code == "" {
				code = detailObj.Status
			}
			if code == "" {
				code = detailObj.Type
			}
			if requestID == "" && detailObj.RequestID != "" {
				requestID = detailObj.RequestID
			}
		case json.Unmarshal(env.Detail, &detailStr) == nil && detailStr != "":
			message = detailStr
		case json.Unmarshal(env.Detail, &detailList) == nil && len(detailList) > 0:
			parts := make([]string, 0, len(detailList))
			for _, item := range detailList {
				if item.Msg == "" {
					continue
				}
				if loc := formatValidationLoc(item.Loc); loc != "" {
					parts = append(parts, fmt.Sprintf("%s: %s", loc, item.Msg))
				} else {
					parts = append(parts, item.Msg)
				}
			}
			message = strings.Join(parts, "; ")
			code = detailList[0].Type
			if code == "" {
				code = "validation_error"
			}
		}
	}

	bodyStr := truncateBody(body)
	if message == "" {
		if bodyStr != "" {
			message = bodyStr
		} else {
			message = http.StatusText(status)
		}
	}

	sentinel := normalize.SentinelForStatusWithOverrides(status, statusOverrides)
	pe := normalize.ProviderError(providerID, status, requestID, code, message, sentinel)
	pe.(*core.ProviderError).Body = bodyStr
	return normalize.WithRetryAfter(pe, headers...)
}

func formatValidationLoc(loc []any) string {
	if len(loc) == 0 {
		return ""
	}
	segments := make([]string, 0, len(loc))
	for _, elem := range loc {
		segments = append(segments, fmt.Sprint(elem))
	}
	return strings.Join(segments, ".")
}

func newNetworkError(err error) error {
	return normalize.NetworkError(providerID, err)
}

func newDecodeError(err error, body []byte) error {
	if len(body) > 0 {
		return normalize.DecodeErrorWithBody(providerID, err, body)
	}
	return normalize.DecodeError(providerID, err)
}

// redactSecretError ensures an API key value never appears in ProviderError
// messages or retained raw response bodies, even if an upstream proxy or server
// reflects request headers in an error payload.
func redactSecretError(err error, secret core.Secret) error {
	raw := secret.Expose()
	if err == nil || raw == "" {
		return err
	}
	var pe *core.ProviderError
	if errors.As(err, &pe) {
		pe.Message = strings.ReplaceAll(pe.Message, raw, "[REDACTED]")
		pe.Body = strings.ReplaceAll(pe.Body, raw, "[REDACTED]")
	}
	return err
}
