package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
)

// Safeguard defaults and API limits for advanced voice management and cloning.
const (
	// MaxVoiceCloneSamples caps audio samples per clone/edit request (ElevenLabs IVC limit).
	MaxVoiceCloneSamples = 25

	// DefaultMaxVoiceSampleBytes caps each uploaded audio sample (10 MiB).
	DefaultMaxVoiceSampleBytes int64 = 10 * 1024 * 1024

	// DefaultMaxVoiceUploadBytes caps the total combined audio bytes across all samples (50 MiB).
	DefaultMaxVoiceUploadBytes int64 = 50 * 1024 * 1024

	maxConsentFutureSkew = 5 * time.Minute
)

// Advanced voice management sentinel errors.
var (
	// ErrVoiceManagementDisabled indicates a mutating voice operation was called
	// on a provider without [WithVoiceManagementEnabled]. Wraps [core.ErrNotSupported].
	ErrVoiceManagementDisabled = fmt.Errorf("elevenlabs: voice management is disabled (enable explicitly with WithVoiceManagementEnabled): %w", core.ErrNotSupported)

	// ErrVoiceConsentRequired indicates a voice cloning or sample-upload operation
	// lacked a valid, confirmed [VoiceConsent] record.
	ErrVoiceConsentRequired = errors.New("elevenlabs: explicit speaker consent is required for voice cloning and sample uploads")

	// ErrVoiceActionDenied indicates the configured [VoiceAuthorizer] rejected a
	// mutating voice management request.
	ErrVoiceActionDenied = errors.New("elevenlabs: voice management action denied by authorizer")
)

// VoiceAction identifies a mutating voice library operation passed to [VoiceAuthorizer].
type VoiceAction string

const (
	VoiceActionClone        VoiceAction = "clone"
	VoiceActionEdit         VoiceAction = "edit"
	VoiceActionEditSettings VoiceAction = "edit_settings"
	VoiceActionDelete       VoiceAction = "delete"
)

// VoiceAuthRequest describes a pending mutating voice operation for
// application-level authorization, RBAC, or audit enforcement.
//
// Privacy: intentionally excludes raw audio sample bytes and API keys.
type VoiceAuthRequest struct {
	Action      VoiceAction
	VoiceID     string
	VoiceName   string
	SampleCount int
	Consent     *VoiceConsent
}

// VoiceAuthorizer enforces application-level authorization or audit policies
// before any mutating voice management HTTP request is sent.
type VoiceAuthorizer interface {
	AuthorizeVoiceAction(ctx context.Context, req VoiceAuthRequest) error
}

// VoiceAuthorizerFunc adapts a function to [VoiceAuthorizer].
type VoiceAuthorizerFunc func(ctx context.Context, req VoiceAuthRequest) error

// AuthorizeVoiceAction invokes f(ctx, req).
func (f VoiceAuthorizerFunc) AuthorizeVoiceAction(ctx context.Context, req VoiceAuthRequest) error {
	return f(ctx, req)
}

// VoiceConsent records verifiable informed speaker consent required before
// uploading audio samples for instant voice cloning ([ElevenLabs.CreateVoice])
// or appending voice samples ([ElevenLabs.EditVoice]).
//
// Privacy: String and GoString omit SubjectID and EvidenceRef so personal
// identifiers and audit references are not emitted in default logs.
type VoiceConsent struct {
	// Confirmed must be true to affirm informed speaker consent was obtained.
	Confirmed bool `json:"confirmed"`

	// SubjectID identifies the consenting speaker (e.g., opaque user identifier). Required.
	SubjectID string `json:"subject_id"`

	// GrantedBy identifies the actor recording consent (e.g., "self" or operator ID). Required.
	GrantedBy string `json:"granted_by"`

	// GrantedAt records when consent was granted. Required and must not be in the future.
	GrantedAt time.Time `json:"granted_at"`

	// EvidenceRef optionally references an external signed consent or audit record.
	EvidenceRef string `json:"evidence_ref,omitempty"`
}

// Validate checks that explicit, non-future speaker consent is recorded.
// Returned errors wrap [ErrVoiceConsentRequired].
func (c *VoiceConsent) Validate() error {
	if c == nil || !c.Confirmed {
		return fmt.Errorf("%w: consent.confirmed must be true", ErrVoiceConsentRequired)
	}
	if strings.TrimSpace(c.SubjectID) == "" {
		return fmt.Errorf("%w: consent.subject_id is required", ErrVoiceConsentRequired)
	}
	if strings.TrimSpace(c.GrantedBy) == "" {
		return fmt.Errorf("%w: consent.granted_by is required", ErrVoiceConsentRequired)
	}
	if c.GrantedAt.IsZero() {
		return fmt.Errorf("%w: consent.granted_at is required", ErrVoiceConsentRequired)
	}
	if c.GrantedAt.After(time.Now().Add(maxConsentFutureSkew)) {
		return fmt.Errorf("%w: consent.granted_at cannot be in the future", ErrVoiceConsentRequired)
	}
	return nil
}

// String returns a log-safe summary omitting SubjectID and EvidenceRef.
func (c VoiceConsent) String() string {
	return fmt.Sprintf("VoiceConsent{confirmed=%t, granted_at=%s, has_evidence=%t}",
		c.Confirmed, c.GrantedAt.UTC().Format(time.RFC3339), strings.TrimSpace(c.EvidenceRef) != "")
}

// GoString returns a log-safe representation for %#v formatting.
func (c VoiceConsent) GoString() string {
	return "elevenlabs." + c.String()
}

// VoiceSample represents a single audio recording uploaded for instant voice
// cloning or voice editing. Exactly one of Data or Reader must be provided.
//
// Privacy: String and GoString omit raw audio bytes.
type VoiceSample struct {
	// Filename is the sanitized audio filename (e.g., "sample1.mp3"). Required.
	// Path separators ("/", "\") and traversal segments ("..") are rejected.
	Filename string

	// Data holds in-memory audio bytes. Mutually exclusive with Reader.
	Data []byte

	// Reader supplies streaming audio bytes without buffering the full file in Data.
	// Mutually exclusive with Data.
	Reader io.Reader
}

// Validate checks filename safety and ensures exactly one non-empty audio source is set.
func (s VoiceSample) Validate(maxSampleBytes int64) error {
	if err := validateVoiceSampleFilename(s.Filename); err != nil {
		return err
	}
	hasData := len(s.Data) > 0
	hasReader := s.Reader != nil
	if !hasData && !hasReader {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("voice sample %q must provide either Data or Reader", s.Filename),
			Err:      core.ErrBadRequest,
		}
	}
	if hasData && hasReader {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("voice sample %q cannot provide both Data and Reader", s.Filename),
			Err:      core.ErrBadRequest,
		}
	}
	if hasData && maxSampleBytes > 0 && int64(len(s.Data)) > maxSampleBytes {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("voice sample %q size %d exceeds max_sample_bytes %d", s.Filename, len(s.Data), maxSampleBytes),
			Err:      core.ErrBadRequest,
		}
	}
	return nil
}

// String returns a log-safe summary without raw audio bytes.
func (s VoiceSample) String() string {
	return fmt.Sprintf("VoiceSample{filename=%q, bytes=%d, has_reader=%t}",
		s.Filename, len(s.Data), s.Reader != nil)
}

// GoString returns a log-safe representation for %#v formatting.
func (s VoiceSample) GoString() string {
	return "elevenlabs." + s.String()
}

func validateVoiceSampleFilename(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice sample filename is required",
			Err:      core.ErrBadRequest,
		}
	}
	if strings.ContainsAny(trimmed, "/\\\r\n\x00") || strings.Contains(trimmed, "..") {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("voice sample filename %q contains forbidden path or control characters", name),
			Err:      core.ErrBadRequest,
		}
	}
	return nil
}

// CreateVoiceRequest configures an opt-in, consent-gated instant voice clone
// via ElevenLabs POST /v1/voices/add.
//
// Privacy: String and GoString omit raw sample bytes and consent identifiers.
type CreateVoiceRequest struct {
	// Name is the display name for the new voice. Required.
	Name string

	// Description optionally describes the voice.
	Description string

	// Labels optionally attaches key-value descriptors (e.g., accent, age, gender).
	Labels map[string]string

	// RemoveBackgroundNoise strips background noise from samples via ElevenLabs
	// audio isolation before cloning.
	RemoveBackgroundNoise bool

	// Samples contains 1..25 audio recordings of the consenting speaker. Required.
	Samples []VoiceSample

	// Consent records verifiable informed speaker consent. Required.
	Consent *VoiceConsent

	// MaxSampleBytes optionally overrides [DefaultMaxVoiceSampleBytes] (10 MiB).
	MaxSampleBytes int64
}

// Validate enforces consent, name, sample count, filename safety, and byte bounds.
func (r *CreateVoiceRequest) Validate() error {
	if r == nil {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "create voice request cannot be nil",
			Err:      core.ErrBadRequest,
		}
	}
	if err := r.Consent.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.Name) == "" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice name is required",
			Err:      core.ErrBadRequest,
		}
	}
	if len(r.Samples) == 0 {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "at least one audio sample is required for voice cloning",
			Err:      core.ErrBadRequest,
		}
	}
	if len(r.Samples) > MaxVoiceCloneSamples {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("voice cloning accepts at most %d samples, got %d", MaxVoiceCloneSamples, len(r.Samples)),
			Err:      core.ErrBadRequest,
		}
	}
	if r.MaxSampleBytes < 0 {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("max_sample_bytes must be non-negative, got %d", r.MaxSampleBytes),
			Err:      core.ErrBadRequest,
		}
	}
	sampleCap := effectiveSampleLimit(r.MaxSampleBytes)
	for _, s := range r.Samples {
		if err := s.Validate(sampleCap); err != nil {
			return err
		}
	}
	return nil
}

// String returns a log-safe summary without raw audio bytes or personal identifiers.
func (r CreateVoiceRequest) String() string {
	return fmt.Sprintf("CreateVoiceRequest{name=%q, samples=%d, remove_bg_noise=%t}",
		r.Name, len(r.Samples), r.RemoveBackgroundNoise)
}

// GoString returns a log-safe representation for %#v formatting.
func (r CreateVoiceRequest) GoString() string {
	return "elevenlabs." + r.String()
}

// CreateVoiceResponse is returned by [ElevenLabs.CreateVoice].
type CreateVoiceResponse struct {
	// VoiceID is the newly created ElevenLabs voice identifier.
	VoiceID string `json:"voice_id"`

	// RequiresVerification indicates whether ElevenLabs flagged the voice for
	// additional verification before synthesis is permitted.
	RequiresVerification bool `json:"requires_verification"`
}

// EditVoiceRequest configures an opt-in voice metadata or sample update via
// ElevenLabs POST /v1/voices/{voice_id}/edit.
//
// When Samples is non-empty, a valid [VoiceConsent] record is required.
type EditVoiceRequest struct {
	// VoiceID identifies the owned voice to modify. Required.
	VoiceID string

	// Name is the updated voice name. Required by ElevenLabs' edit endpoint.
	Name string

	// Description optionally updates the voice description.
	Description string

	// Labels optionally updates key-value metadata descriptors.
	Labels map[string]string

	// RemoveBackgroundNoise cleans newly uploaded samples when Samples is non-empty.
	RemoveBackgroundNoise bool

	// Samples optionally appends additional audio samples to the voice.
	// Requires Consent when non-empty.
	Samples []VoiceSample

	// Consent is required whenever Samples is non-empty.
	Consent *VoiceConsent

	// MaxSampleBytes optionally overrides [DefaultMaxVoiceSampleBytes] (10 MiB).
	MaxSampleBytes int64
}

// Validate checks voice ID, name, optional sample safety, and consent whenever
// new audio samples are uploaded.
func (r *EditVoiceRequest) Validate() error {
	if r == nil {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "edit voice request cannot be nil",
			Err:      core.ErrBadRequest,
		}
	}
	if strings.TrimSpace(r.VoiceID) == "" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice_id is required",
			Err:      core.ErrBadRequest,
		}
	}
	if strings.TrimSpace(r.Name) == "" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice name is required",
			Err:      core.ErrBadRequest,
		}
	}
	if len(r.Samples) > MaxVoiceCloneSamples {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("voice edit accepts at most %d samples, got %d", MaxVoiceCloneSamples, len(r.Samples)),
			Err:      core.ErrBadRequest,
		}
	}
	if r.MaxSampleBytes < 0 {
		return &core.ProviderError{
			Provider: providerID,
			Message:  fmt.Sprintf("max_sample_bytes must be non-negative, got %d", r.MaxSampleBytes),
			Err:      core.ErrBadRequest,
		}
	}
	if len(r.Samples) > 0 {
		if err := r.Consent.Validate(); err != nil {
			return err
		}
		sampleCap := effectiveSampleLimit(r.MaxSampleBytes)
		for _, s := range r.Samples {
			if err := s.Validate(sampleCap); err != nil {
				return err
			}
		}
	}
	return nil
}

// String returns a log-safe summary without raw audio bytes or personal identifiers.
func (r EditVoiceRequest) String() string {
	return fmt.Sprintf("EditVoiceRequest{voice_id=%q, name=%q, samples=%d}",
		r.VoiceID, r.Name, len(r.Samples))
}

// GoString returns a log-safe representation for %#v formatting.
func (r EditVoiceRequest) GoString() string {
	return "elevenlabs." + r.String()
}

// CreateVoice creates an instant voice clone via POST /v1/voices/add.
//
// Security and consent safeguards:
//   - Requires [WithVoiceManagementEnabled] on the provider (otherwise returns
//     [ErrVoiceManagementDisabled] without sending HTTP traffic).
//   - Requires a confirmed, non-future [VoiceConsent] record (otherwise returns
//     [ErrVoiceConsentRequired]).
//   - Invokes [WithVoiceAuthorizer] if configured (otherwise returns
//     [ErrVoiceActionDenied] on denial).
//   - Streams multipart uploads through bounded byte limits so oversized files
//     cannot exhaust memory.
func (p *ElevenLabs) CreateVoice(ctx context.Context, req *CreateVoiceRequest) (*CreateVoiceResponse, error) {
	if err := p.requireVoiceManagementEnabled(); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := p.authorizeVoiceAction(ctx, VoiceAuthRequest{
		Action:      VoiceActionClone,
		VoiceName:   req.Name,
		SampleCount: len(req.Samples),
		Consent:     req.Consent,
	}); err != nil {
		return nil, err
	}

	body, err := p.doVoiceMultipart(ctx, "/v1/voices/add", req.Name, req.Description, req.Labels, req.RemoveBackgroundNoise, req.Samples, effectiveSampleLimit(req.MaxSampleBytes))
	if err != nil {
		return nil, err
	}

	var out CreateVoiceResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, redactSecretError(newDecodeError(err, body), p.config.APIKey)
	}
	if strings.TrimSpace(out.VoiceID) == "" {
		return nil, newDecodeError(errors.New("elevenlabs: create voice response missing voice_id"), nil)
	}
	return &out, nil
}

// EditVoice updates metadata and optionally appends audio samples to an owned
// voice via POST /v1/voices/{voice_id}/edit.
//
// Requires [WithVoiceManagementEnabled], passes [WithVoiceAuthorizer] if
// configured, and enforces [VoiceConsent] whenever req.Samples is non-empty.
func (p *ElevenLabs) EditVoice(ctx context.Context, req *EditVoiceRequest) error {
	if err := p.requireVoiceManagementEnabled(); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	if err := p.authorizeVoiceAction(ctx, VoiceAuthRequest{
		Action:      VoiceActionEdit,
		VoiceID:     strings.TrimSpace(req.VoiceID),
		VoiceName:   req.Name,
		SampleCount: len(req.Samples),
		Consent:     req.Consent,
	}); err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/voices/%s/edit", url.PathEscape(strings.TrimSpace(req.VoiceID)))
	_, err := p.doVoiceMultipart(ctx, path, req.Name, req.Description, req.Labels, req.RemoveBackgroundNoise, req.Samples, effectiveSampleLimit(req.MaxSampleBytes))
	return err
}

// EditVoiceSettings updates the saved default synthesis settings for a voice
// via POST /v1/voices/{voice_id}/settings/edit.
//
// Requires [WithVoiceManagementEnabled] and passes [WithVoiceAuthorizer] if configured.
func (p *ElevenLabs) EditVoiceSettings(ctx context.Context, voiceID string, settings *core.VoiceSettings) error {
	if err := p.requireVoiceManagementEnabled(); err != nil {
		return err
	}
	voiceID = strings.TrimSpace(voiceID)
	if voiceID == "" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice_id is required",
			Err:      core.ErrBadRequest,
		}
	}
	if settings == nil {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice settings cannot be nil",
			Err:      core.ErrBadRequest,
		}
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	if err := p.authorizeVoiceAction(ctx, VoiceAuthRequest{
		Action:  VoiceActionEditSettings,
		VoiceID: voiceID,
	}); err != nil {
		return err
	}

	payload, err := json.Marshal(ttsVoiceSettings{
		Stability:       settings.Stability,
		SimilarityBoost: settings.SimilarityBoost,
		Style:           settings.Style,
		UseSpeakerBoost: settings.UseSpeakerBoost,
		Speed:           settings.Speed,
	})
	if err != nil {
		return newDecodeError(fmt.Errorf("marshal voice settings: %w", err), nil)
	}

	path := fmt.Sprintf("/v1/voices/%s/settings/edit", url.PathEscape(voiceID))
	resp, err := p.doRequest(ctx, http.MethodPost, path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	return resp.Body.Close()
}

// DeleteVoice permanently deletes an owned custom/cloned voice via
// DELETE /v1/voices/{voice_id}.
//
// Requires [WithVoiceManagementEnabled] and passes [WithVoiceAuthorizer] if configured.
func (p *ElevenLabs) DeleteVoice(ctx context.Context, voiceID string) error {
	if err := p.requireVoiceManagementEnabled(); err != nil {
		return err
	}
	voiceID = strings.TrimSpace(voiceID)
	if voiceID == "" {
		return &core.ProviderError{
			Provider: providerID,
			Message:  "voice_id is required",
			Err:      core.ErrBadRequest,
		}
	}
	if err := p.authorizeVoiceAction(ctx, VoiceAuthRequest{
		Action:  VoiceActionDelete,
		VoiceID: voiceID,
	}); err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/voices/%s", url.PathEscape(voiceID))
	resp, err := p.doRequest(ctx, http.MethodDelete, path, "", nil)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	return resp.Body.Close()
}

func (p *ElevenLabs) requireVoiceManagementEnabled() error {
	if !p.config.VoiceManagementEnabled {
		return ErrVoiceManagementDisabled
	}
	return nil
}

func (p *ElevenLabs) authorizeVoiceAction(ctx context.Context, req VoiceAuthRequest) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if p.config.VoiceAuthorizer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := p.config.VoiceAuthorizer.AuthorizeVoiceAction(ctx, req); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrVoiceActionDenied, err)
	}
	return nil
}

func effectiveSampleLimit(override int64) int64 {
	if override > 0 {
		return override
	}
	return DefaultMaxVoiceSampleBytes
}

func (p *ElevenLabs) doVoiceMultipart(
	ctx context.Context,
	path string,
	name string,
	description string,
	labels map[string]string,
	removeBackgroundNoise bool,
	samples []VoiceSample,
	maxSampleBytes int64,
) ([]byte, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	var labelsJSON string
	if len(labels) > 0 {
		b, err := json.Marshal(labels)
		if err != nil {
			return nil, newDecodeError(fmt.Errorf("marshal voice labels: %w", err), nil)
		}
		labelsJSON = string(b)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	writeErrCh := make(chan error, 1)
	go func() {
		var writeErr error
		defer func() {
			if closeErr := mw.Close(); closeErr != nil && writeErr == nil {
				writeErr = closeErr
			}
			_ = pw.CloseWithError(writeErr)
			writeErrCh <- writeErr
		}()

		if err := mw.WriteField("name", strings.TrimSpace(name)); err != nil {
			writeErr = err
			return
		}
		if strings.TrimSpace(description) != "" {
			if err := mw.WriteField("description", description); err != nil {
				writeErr = err
				return
			}
		}
		if labelsJSON != "" {
			if err := mw.WriteField("labels", labelsJSON); err != nil {
				writeErr = err
				return
			}
		}
		if removeBackgroundNoise {
			if err := mw.WriteField("remove_background_noise", "true"); err != nil {
				writeErr = err
				return
			}
		}

		var totalBytes int64
		for _, sample := range samples {
			part, err := mw.CreateFormFile("files", sample.Filename)
			if err != nil {
				writeErr = err
				return
			}
			var src io.Reader
			if len(sample.Data) > 0 {
				src = bytes.NewReader(sample.Data)
			} else {
				src = sample.Reader
			}

			n, err := io.Copy(part, io.LimitReader(src, maxSampleBytes+1))
			if err != nil {
				writeErr = err
				return
			}
			if n == 0 {
				writeErr = &core.ProviderError{
					Provider: providerID,
					Message:  fmt.Sprintf("voice sample %q is empty", sample.Filename),
					Err:      core.ErrBadRequest,
				}
				return
			}
			if n > maxSampleBytes {
				writeErr = &core.ProviderError{
					Provider: providerID,
					Message:  fmt.Sprintf("voice sample %q exceeds max_sample_bytes %d", sample.Filename, maxSampleBytes),
					Err:      core.ErrBadRequest,
				}
				return
			}
			totalBytes += n
			if totalBytes > DefaultMaxVoiceUploadBytes {
				writeErr = &core.ProviderError{
					Provider: providerID,
					Message:  fmt.Sprintf("total voice upload exceeds maximum of %d bytes", DefaultMaxVoiceUploadBytes),
					Err:      core.ErrBadRequest,
				}
				return
			}
		}
	}()

	resp, reqErr := p.doRequest(ctx, http.MethodPost, path, contentType, pr)
	_ = pr.Close()
	pipeErr := <-writeErrCh

	if pipeErr != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		var pe *core.ProviderError
		if errors.As(pipeErr, &pe) {
			return nil, pipeErr
		}
		if errors.Is(pipeErr, context.Canceled) || errors.Is(pipeErr, context.DeadlineExceeded) {
			return nil, pipeErr
		}
		if reqErr != nil {
			return nil, reqErr
		}
		return nil, redactSecretError(newNetworkError(pipeErr), p.config.APIKey)
	}
	if reqErr != nil {
		return nil, reqErr
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVoicesResponseBytes+1))
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, redactSecretError(newNetworkError(err), p.config.APIKey)
	}
	if int64(len(body)) > maxVoicesResponseBytes {
		return nil, newDecodeError(fmt.Errorf("response exceeds maximum size of %d bytes", maxVoicesResponseBytes), nil)
	}
	return body, nil
}
