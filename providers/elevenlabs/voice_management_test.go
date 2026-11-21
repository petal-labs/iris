package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func validTestConsent() *VoiceConsent {
	return &VoiceConsent{
		Confirmed:   true,
		SubjectID:   "SECRET_SUBJECT_USER_42",
		GrantedBy:   "self",
		GrantedAt:   time.Now().Add(-time.Minute),
		EvidenceRef: "SECRET_CONSENT_DOC_999",
	}
}

func TestVoiceManagementDisabledByDefault(t *testing.T) {
	httpCalled := false
	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			httpCalled = true
			return nil, errors.New("must not reach network when disabled")
		}),
	}

	// Default provider has voice management disabled.
	p := New("xi_key", WithHTTPClient(client))
	stab := 0.5

	if _, err := p.CreateVoice(context.Background(), &CreateVoiceRequest{
		Name:    "Clone",
		Samples: []VoiceSample{{Filename: "s1.mp3", Data: []byte("mp3")}},
		Consent: validTestConsent(),
	}); !errors.Is(err, ErrVoiceManagementDisabled) || !errors.Is(err, core.ErrNotSupported) {
		t.Errorf("CreateVoice disabled err = %v, want ErrVoiceManagementDisabled/ErrNotSupported", err)
	}
	if err := p.EditVoice(context.Background(), &EditVoiceRequest{VoiceID: "v1", Name: "Renamed"}); !errors.Is(err, ErrVoiceManagementDisabled) {
		t.Errorf("EditVoice disabled err = %v, want ErrVoiceManagementDisabled", err)
	}
	if err := p.EditVoiceSettings(context.Background(), "v1", &core.VoiceSettings{Stability: &stab}); !errors.Is(err, ErrVoiceManagementDisabled) {
		t.Errorf("EditVoiceSettings disabled err = %v, want ErrVoiceManagementDisabled", err)
	}
	if err := p.DeleteVoice(context.Background(), "v1"); !errors.Is(err, ErrVoiceManagementDisabled) {
		t.Errorf("DeleteVoice disabled err = %v, want ErrVoiceManagementDisabled", err)
	}
	if httpCalled {
		t.Error("HTTP transport must not be called when voice management is disabled")
	}
}

func TestVoiceConsentAuthorizerAndPrivacySafeguards(t *testing.T) {
	t.Run("consent privacy omits SubjectID and EvidenceRef", func(t *testing.T) {
		c := validTestConsent()
		for _, out := range []string{c.String(), fmt.Sprintf("%#v", c)} {
			if strings.Contains(out, "SECRET_SUBJECT_USER_42") || strings.Contains(out, "SECRET_CONSENT_DOC_999") {
				t.Errorf("VoiceConsent leaked PII: %s", out)
			}
		}

		sample := VoiceSample{Filename: "clip.mp3", Data: []byte("SECRET_VOICE_SAMPLE_BYTES")}
		for _, out := range []string{sample.String(), fmt.Sprintf("%#v", sample)} {
			if strings.Contains(out, "SECRET_VOICE_SAMPLE_BYTES") {
				t.Errorf("VoiceSample leaked audio bytes: %s", out)
			}
		}

		cr := CreateVoiceRequest{Name: "Test", Samples: []VoiceSample{sample}, Consent: c}
		er := EditVoiceRequest{VoiceID: "v1", Name: "Test", Samples: []VoiceSample{sample}, Consent: c}
		for _, out := range []string{cr.String(), fmt.Sprintf("%#v", cr), er.String(), fmt.Sprintf("%#v", er)} {
			if strings.Contains(out, "SECRET_VOICE_SAMPLE_BYTES") || strings.Contains(out, "SECRET_SUBJECT_USER_42") {
				t.Errorf("request String/GoString leaked sensitive data: %s", out)
			}
		}
	})

	t.Run("missing or invalid consent blocks cloning and sample edits", func(t *testing.T) {
		p := New("xi_key", WithVoiceManagementEnabled())
		badConsents := []*VoiceConsent{
			nil,
			{Confirmed: false, SubjectID: "u1", GrantedBy: "self", GrantedAt: time.Now()},
			{Confirmed: true, SubjectID: "  ", GrantedBy: "self", GrantedAt: time.Now()},
			{Confirmed: true, SubjectID: "u1", GrantedBy: "", GrantedAt: time.Now()},
			{Confirmed: true, SubjectID: "u1", GrantedBy: "self"},
			{Confirmed: true, SubjectID: "u1", GrantedBy: "self", GrantedAt: time.Now().Add(time.Hour)},
		}
		for i, bc := range badConsents {
			if _, err := p.CreateVoice(context.Background(), &CreateVoiceRequest{
				Name:    "Clone",
				Samples: []VoiceSample{{Filename: "a.mp3", Data: []byte("abc")}},
				Consent: bc,
			}); !errors.Is(err, ErrVoiceConsentRequired) {
				t.Errorf("CreateVoice badConsent[%d] err = %v, want ErrVoiceConsentRequired", i, err)
			}
		}

		// EditVoice with new samples also requires consent.
		if err := p.EditVoice(context.Background(), &EditVoiceRequest{
			VoiceID: "v1",
			Name:    "Updated",
			Samples: []VoiceSample{{Filename: "a.mp3", Data: []byte("abc")}},
		}); !errors.Is(err, ErrVoiceConsentRequired) {
			t.Errorf("EditVoice with samples without consent err = %v, want ErrVoiceConsentRequired", err)
		}
	})

	t.Run("authorizer denial blocks mutation before HTTP", func(t *testing.T) {
		httpCalled := false
		var seenAction VoiceAction
		p := New("xi_key",
			WithVoiceManagementEnabled(),
			WithVoiceAuthorizer(VoiceAuthorizerFunc(func(_ context.Context, req VoiceAuthRequest) error {
				seenAction = req.Action
				return errors.New("role lacks voice:write permission")
			})),
			WithHTTPClient(&http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					httpCalled = true
					return nil, errors.New("unexpected http")
				}),
			}),
		)

		err := p.DeleteVoice(context.Background(), "voice_owned_1")
		if !errors.Is(err, ErrVoiceActionDenied) {
			t.Fatalf("DeleteVoice denied err = %v, want ErrVoiceActionDenied", err)
		}
		if seenAction != VoiceActionDelete || httpCalled {
			t.Errorf("seenAction=%q httpCalled=%t", seenAction, httpCalled)
		}
	})

	t.Run("sample filename traversal and size validation", func(t *testing.T) {
		p := New("xi_key", WithVoiceManagementEnabled())
		badSamples := []VoiceSample{
			{Filename: "", Data: []byte("x")},
			{Filename: "../etc/passwd", Data: []byte("x")},
			{Filename: "dir/sample.mp3", Data: []byte("x")},
			{Filename: `dir\sample.mp3`, Data: []byte("x")},
			{Filename: "empty.mp3"},
			{Filename: "both.mp3", Data: []byte("x"), Reader: strings.NewReader("x")},
			{Filename: "big.mp3", Data: []byte("1234567890")},
		}
		for i, bs := range badSamples {
			_, err := p.CreateVoice(context.Background(), &CreateVoiceRequest{
				Name:           "Test",
				Samples:        []VoiceSample{bs},
				Consent:        validTestConsent(),
				MaxSampleBytes: 5,
			})
			if !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("badSamples[%d] err = %v, want core.ErrBadRequest", i, err)
			}
		}
	})
}

func TestCreateEditSettingsAndDeleteVoiceSuccess(t *testing.T) {
	var (
		gotMethod     string
		gotPath       string
		gotFields     map[string]string
		gotFiles      map[string][]byte
		gotSettings   ttsVoiceSettings
		authorizedOps []VoiceAction
	)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotMethod = req.Method
			gotPath = req.URL.Path

			switch {
			case req.Method == http.MethodPost && (req.URL.Path == "/v1/voices/add" || strings.HasSuffix(req.URL.Path, "/edit")):
				if strings.HasSuffix(req.URL.Path, "/settings/edit") {
					body, _ := io.ReadAll(req.Body)
					_ = json.Unmarshal(body, &gotSettings)
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
					}, nil
				}

				_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
				if err != nil {
					return nil, err
				}
				mr := multipart.NewReader(req.Body, params["boundary"])
				gotFields = make(map[string]string)
				gotFiles = make(map[string][]byte)
				for {
					part, err := mr.NextPart()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						return nil, err
					}
					data, _ := io.ReadAll(part)
					if part.FileName() != "" {
						gotFiles[part.FileName()] = data
					} else {
						gotFields[part.FormName()] = string(data)
					}
				}
				if req.URL.Path == "/v1/voices/add" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"voice_id":"voice_cloned_7","requires_verification":false}`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				}, nil

			case req.Method == http.MethodDelete && req.URL.Path == "/v1/voices/voice_cloned_7":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				}, nil
			}
			return nil, fmt.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}),
	}

	p := New("xi_live_key",
		WithVoiceManagementEnabled(),
		WithVoiceAuthorizer(VoiceAuthorizerFunc(func(_ context.Context, req VoiceAuthRequest) error {
			authorizedOps = append(authorizedOps, req.Action)
			return nil
		})),
		WithHTTPClient(client),
	)

	// 1. CreateVoice (instant voice clone with both in-memory Data and streaming Reader).
	created, err := p.CreateVoice(context.Background(), &CreateVoiceRequest{
		Name:                  "Studio Host",
		Description:           "Warm narration voice",
		Labels:                map[string]string{"accent": "american"},
		RemoveBackgroundNoise: true,
		Samples: []VoiceSample{
			{Filename: "take1.mp3", Data: []byte("sample-one")},
			{Filename: "take2.wav", Reader: bytes.NewReader([]byte("sample-two"))},
		},
		Consent: validTestConsent(),
	})
	if err != nil {
		t.Fatalf("CreateVoice() err = %v", err)
	}
	if created.VoiceID != "voice_cloned_7" || created.RequiresVerification {
		t.Errorf("unexpected CreateVoiceResponse: %+v", created)
	}
	if gotFields["name"] != "Studio Host" ||
		gotFields["description"] != "Warm narration voice" ||
		gotFields["remove_background_noise"] != "true" ||
		!strings.Contains(gotFields["labels"], `"accent":"american"`) {
		t.Errorf("unexpected multipart fields: %+v", gotFields)
	}
	if string(gotFiles["take1.mp3"]) != "sample-one" || string(gotFiles["take2.wav"]) != "sample-two" {
		t.Errorf("unexpected uploaded files: %+v", gotFiles)
	}

	// 2. EditVoice (metadata-only update does not require Consent).
	if err := p.EditVoice(context.Background(), &EditVoiceRequest{
		VoiceID:     "voice_cloned_7",
		Name:        "Studio Host v2",
		Description: "Updated description",
	}); err != nil {
		t.Fatalf("EditVoice() err = %v", err)
	}
	if gotPath != "/v1/voices/voice_cloned_7/edit" || gotFields["name"] != "Studio Host v2" {
		t.Errorf("unexpected EditVoice request: path=%q fields=%+v", gotPath, gotFields)
	}

	// 3. EditVoiceSettings.
	stab, sim, spd := 0.65, 0.85, 1.05
	boost := true
	if err := p.EditVoiceSettings(context.Background(), "voice_cloned_7", &core.VoiceSettings{
		Stability:       &stab,
		SimilarityBoost: &sim,
		UseSpeakerBoost: &boost,
		Speed:           &spd,
	}); err != nil {
		t.Fatalf("EditVoiceSettings() err = %v", err)
	}
	if gotPath != "/v1/voices/voice_cloned_7/settings/edit" ||
		gotSettings.Stability == nil || *gotSettings.Stability != 0.65 ||
		gotSettings.Speed == nil || *gotSettings.Speed != 1.05 {
		t.Errorf("unexpected EditVoiceSettings payload: %+v", gotSettings)
	}

	// 4. DeleteVoice.
	if err := p.DeleteVoice(context.Background(), "voice_cloned_7"); err != nil {
		t.Fatalf("DeleteVoice() err = %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v1/voices/voice_cloned_7" {
		t.Errorf("unexpected DeleteVoice request: %s %s", gotMethod, gotPath)
	}

	if len(authorizedOps) != 4 ||
		authorizedOps[0] != VoiceActionClone ||
		authorizedOps[1] != VoiceActionEdit ||
		authorizedOps[2] != VoiceActionEditSettings ||
		authorizedOps[3] != VoiceActionDelete {
		t.Errorf("unexpected authorizedOps: %v", authorizedOps)
	}
}

func TestVoiceManagementReaderLimitAndCancellation(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if _, err := io.Copy(io.Discard, req.Body); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"voice_id":"v1"}`)),
			}, nil
		}),
	}
	p := New("xi_key", WithVoiceManagementEnabled(), WithHTTPClient(client))

	// Oversized streaming Reader sample is rejected cleanly by pipe limit.
	_, err := p.CreateVoice(context.Background(), &CreateVoiceRequest{
		Name:           "Oversized",
		Samples:        []VoiceSample{{Filename: "big.mp3", Reader: bytes.NewReader(make([]byte, 32))}},
		Consent:        validTestConsent(),
		MaxSampleBytes: 16,
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("oversized reader err = %v, want core.ErrBadRequest", err)
	}

	// Cancelled context fails before sending HTTP.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.DeleteVoice(ctx, "v1"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled DeleteVoice err = %v, want context.Canceled", err)
	}
}
