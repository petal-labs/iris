package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/petal-labs/iris/core"
)

const (
	// MaxVoicesPageSize is the maximum page_size supported by ElevenLabs GET /v2/voices.
	MaxVoicesPageSize = 100

	maxVoicesResponseBytes = 16 * 1024 * 1024 // 16 MiB
	maxVoicePages          = 1000
)

// Voice represents metadata and default synthesis settings for an ElevenLabs voice.
//
// Read-only discovery methods ([ElevenLabs.ListVoices],
// [ElevenLabs.ListAllVoices], [ElevenLabs.GetVoice],
// [ElevenLabs.GetVoiceSettings], and [ElevenLabs.GetDefaultVoiceSettings]) are
// always available. Mutating operations ([ElevenLabs.CreateVoice],
// [ElevenLabs.EditVoice], [ElevenLabs.EditVoiceSettings], and
// [ElevenLabs.DeleteVoice]) are strictly opt-in via
// [WithVoiceManagementEnabled] and enforce consent and authorization checks.
type Voice struct {
	// VoiceID is the unique ElevenLabs voice identifier passed to SpeechRequest.VoiceID.
	VoiceID string `json:"voice_id"`

	// Name is the human-readable voice name.
	Name string `json:"name"`

	// Category describes the voice class (e.g., "premade", "cloned", "generated", "professional").
	Category string `json:"category,omitempty"`

	// Description is an optional summary of the voice characteristics.
	Description string `json:"description,omitempty"`

	// PreviewURL is an optional URL to an audio preview sample for the voice.
	PreviewURL string `json:"preview_url,omitempty"`

	// Labels holds optional key-value descriptors such as accent, age, gender, and use_case.
	Labels map[string]string `json:"labels,omitempty"`

	// AvailableForTiers lists subscription tiers that may use this voice, if reported.
	AvailableForTiers []string `json:"available_for_tiers,omitempty"`

	// Settings holds the voice's default synthesis settings when returned by the API.
	Settings *core.VoiceSettings `json:"settings,omitempty"`

	// IsOwner reports whether the authenticated account owns this voice.
	IsOwner bool `json:"is_owner,omitempty"`

	// IsLegacy reports whether this voice is a legacy voice.
	IsLegacy bool `json:"is_legacy,omitempty"`

	// IsMixed reports whether this voice is a mixed voice.
	IsMixed bool `json:"is_mixed,omitempty"`
}

// VoiceListRequest configures filtering, sorting, and cursor pagination for
// [ElevenLabs.ListVoices] and [ElevenLabs.ListAllVoices].
type VoiceListRequest struct {
	// NextPageToken is the pagination cursor from a previous VoiceListResponse.
	NextPageToken string

	// PageSize limits voices returned per page (0 uses ElevenLabs' default of 10; max 100).
	PageSize int

	// Search filters voices by substring across name, description, labels, and category.
	Search string

	// Sort specifies the sort field (e.g., "created_at_unix", "name").
	Sort string

	// SortDirection specifies sort order: "asc" or "desc".
	SortDirection string

	// VoiceType filters by voice source (e.g., "personal", "community", "default", "workspace", "non-default").
	VoiceType string

	// Category filters by voice category (e.g., "premade", "cloned", "generated", "professional").
	Category string

	// IncludeTotalCount controls whether the response computes TotalCount.
	IncludeTotalCount *bool
}

// Validate checks that pagination and sorting parameters are within supported bounds.
func (r *VoiceListRequest) Validate() error {
	if r == nil {
		return nil
	}
	if r.PageSize < 0 || r.PageSize > MaxVoicesPageSize {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("page_size must be between 0 and %d, got %d", MaxVoicesPageSize, r.PageSize),
			Err:      core.ErrBadRequest,
		}
	}
	if r.SortDirection != "" && r.SortDirection != "asc" && r.SortDirection != "desc" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("sort_direction must be \"asc\" or \"desc\", got %q", r.SortDirection),
			Err:      core.ErrBadRequest,
		}
	}
	return nil
}

// VoiceListResponse contains a single page of voices from ElevenLabs GET /v2/voices.
type VoiceListResponse struct {
	// Voices holds the voices in this page.
	Voices []Voice `json:"voices"`

	// HasMore reports whether additional pages are available.
	HasMore bool `json:"has_more"`

	// NextPageToken is the cursor to pass in VoiceListRequest.NextPageToken for the next page.
	NextPageToken string `json:"next_page_token,omitempty"`

	// TotalCount is the total matching voice count when IncludeTotalCount is enabled.
	TotalCount int `json:"total_count,omitempty"`
}

// ListVoices retrieves a single paginated page of available voices from
// ElevenLabs GET /v2/voices. A nil req uses server defaults.
func (p *ElevenLabs) ListVoices(ctx context.Context, req *VoiceListRequest) (*VoiceListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	path := "/v2/voices"
	if q := buildVoiceListQuery(req); q != "" {
		path += "?" + q
	}

	var out VoiceListResponse
	if err := p.doGetJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAllVoices automatically paginates through [ElevenLabs.ListVoices] until
// all matching voices have been collected or ctx is cancelled.
//
// If req is nil or req.PageSize is 0, [MaxVoicesPageSize] (100) is used per
// page to minimize round trips. The caller's req struct is never mutated.
func (p *ElevenLabs) ListAllVoices(ctx context.Context, req *VoiceListRequest) ([]Voice, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var pageReq VoiceListRequest
	if req != nil {
		pageReq = *req
	}
	if pageReq.PageSize == 0 {
		pageReq.PageSize = MaxVoicesPageSize
	}

	var all []Voice
	seenTokens := make(map[string]struct{})

	for page := range maxVoicePages {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		resp, err := p.ListVoices(ctx, &pageReq)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Voices...)

		if !resp.HasMore || resp.NextPageToken == "" {
			return all, nil
		}
		if _, dup := seenTokens[resp.NextPageToken]; dup {
			return nil, newDecodeError(fmt.Errorf("elevenlabs returned duplicate next_page_token on page %d", page+1), nil)
		}
		seenTokens[resp.NextPageToken] = struct{}{}
		pageReq.NextPageToken = resp.NextPageToken
	}

	return nil, newDecodeError(fmt.Errorf("voice pagination exceeded maximum of %d pages", maxVoicePages), nil)
}

// GetVoice retrieves metadata and default settings for a single voice via
// ElevenLabs GET /v1/voices/{voice_id}.
func (p *ElevenLabs) GetVoice(ctx context.Context, voiceID string) (*Voice, error) {
	voiceID = strings.TrimSpace(voiceID)
	if voiceID == "" {
		return nil, &core.ProviderError{
			Provider: providerID,
			Message:  "voice_id is required",
			Err:      core.ErrBadRequest,
		}
	}

	path := fmt.Sprintf("/v1/voices/%s", url.PathEscape(voiceID))
	var voice Voice
	if err := p.doGetJSON(ctx, path, &voice); err != nil {
		return nil, err
	}
	return &voice, nil
}

// GetDefaultVoiceSettings retrieves ElevenLabs' account-independent default
// voice settings via GET /v1/voices/settings/default.
func (p *ElevenLabs) GetDefaultVoiceSettings(ctx context.Context) (*core.VoiceSettings, error) {
	var settings core.VoiceSettings
	if err := p.doGetJSON(ctx, "/v1/voices/settings/default", &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetVoiceSettings retrieves the saved synthesis settings for a specific voice
// via ElevenLabs GET /v1/voices/{voice_id}/settings.
func (p *ElevenLabs) GetVoiceSettings(ctx context.Context, voiceID string) (*core.VoiceSettings, error) {
	voiceID = strings.TrimSpace(voiceID)
	if voiceID == "" {
		return nil, &core.ProviderError{
			Provider: providerID,
			Message:  "voice_id is required",
			Err:      core.ErrBadRequest,
		}
	}

	path := fmt.Sprintf("/v1/voices/%s/settings", url.PathEscape(voiceID))
	var settings core.VoiceSettings
	if err := p.doGetJSON(ctx, path, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (p *ElevenLabs) doGetJSON(ctx context.Context, path string, dst any) error {
	resp, err := p.doRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVoicesResponseBytes+1))
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return redactSecretError(newNetworkError(err), p.config.APIKey)
	}
	if int64(len(body)) > maxVoicesResponseBytes {
		return newDecodeError(fmt.Errorf("response exceeds maximum size of %d bytes", maxVoicesResponseBytes), nil)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return redactSecretError(newDecodeError(err, body), p.config.APIKey)
	}
	return nil
}

func buildVoiceListQuery(req *VoiceListRequest) string {
	if req == nil {
		return ""
	}
	q := url.Values{}
	if req.NextPageToken != "" {
		q.Set("next_page_token", req.NextPageToken)
	}
	if req.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(req.PageSize))
	}
	if req.Search != "" {
		q.Set("search", req.Search)
	}
	if req.Sort != "" {
		q.Set("sort", req.Sort)
	}
	if req.SortDirection != "" {
		q.Set("sort_direction", req.SortDirection)
	}
	if req.VoiceType != "" {
		q.Set("voice_type", req.VoiceType)
	}
	if req.Category != "" {
		q.Set("category", req.Category)
	}
	if req.IncludeTotalCount != nil {
		q.Set("include_total_count", strconv.FormatBool(*req.IncludeTotalCount))
	}
	return q.Encode()
}
