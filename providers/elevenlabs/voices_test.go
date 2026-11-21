package elevenlabs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/v2/core"
)

func TestListVoicesSuccessFiltersAndValidation(t *testing.T) {
	t.Run("full filters and pagination response", func(t *testing.T) {
		var capturedPath, capturedQuery, capturedKey string
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				capturedPath = req.URL.Path
				capturedQuery = req.URL.RawQuery
				capturedKey = req.Header.Get("xi-api-key")
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"voices": [
							{
								"voice_id": "v_george",
								"name": "George",
								"category": "premade",
								"description": "Warm British storyteller",
								"preview_url": "https://cdn.elevenlabs.io/preview.mp3",
								"labels": {"accent": "british", "gender": "male"},
								"available_for_tiers": ["free", "creator"],
								"settings": {
									"stability": 0.5,
									"similarity_boost": 0.75,
									"style": 0.0,
									"use_speaker_boost": true,
									"speed": 1.0
								},
								"is_owner": false
							}
						],
						"has_more": true,
						"next_page_token": "tok_page_2",
						"total_count": 42
					}`)),
				}, nil
			}),
		}

		p := New("sk_voice_key", WithHTTPClient(client))
		resp, err := p.ListVoices(context.Background(), &VoiceListRequest{
			NextPageToken:     "tok_page_1",
			PageSize:          25,
			Search:            "George",
			Sort:              "name",
			SortDirection:     "asc",
			VoiceType:         "default",
			Category:          "premade",
			IncludeTotalCount: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("ListVoices() error = %v", err)
		}

		if capturedPath != "/v2/voices" || capturedKey != "sk_voice_key" {
			t.Errorf("path=%q key=%q", capturedPath, capturedKey)
		}
		for _, expected := range []string{
			"next_page_token=tok_page_1",
			"page_size=25",
			"search=George",
			"sort=name",
			"sort_direction=asc",
			"voice_type=default",
			"category=premade",
			"include_total_count=true",
		} {
			if !strings.Contains(capturedQuery, expected) {
				t.Errorf("query %q missing %q", capturedQuery, expected)
			}
		}

		if !resp.HasMore || resp.NextPageToken != "tok_page_2" || resp.TotalCount != 42 || len(resp.Voices) != 1 {
			t.Fatalf("unexpected VoiceListResponse: %+v", resp)
		}
		v := resp.Voices[0]
		if v.VoiceID != "v_george" || v.Name != "George" || v.Category != "premade" || v.Labels["accent"] != "british" {
			t.Errorf("unexpected voice metadata: %+v", v)
		}
		if v.Settings == nil || *v.Settings.Stability != 0.5 || *v.Settings.SimilarityBoost != 0.75 || *v.Settings.Speed != 1.0 || !*v.Settings.UseSpeakerBoost {
			t.Errorf("unexpected voice settings: %+v", v.Settings)
		}
		if err := v.Settings.Validate(); err != nil {
			t.Errorf("returned voice settings failed validation: %v", err)
		}
	})

	t.Run("nil request sends clean /v2/voices", func(t *testing.T) {
		var capturedRawQuery string
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				capturedRawQuery = req.URL.RawQuery
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"voices":[],"has_more":false}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		resp, err := p.ListVoices(context.Background(), nil)
		if err != nil {
			t.Fatalf("ListVoices(nil) error = %v", err)
		}
		if capturedRawQuery != "" || len(resp.Voices) != 0 || resp.HasMore {
			t.Errorf("query=%q resp=%+v", capturedRawQuery, resp)
		}
	})

	t.Run("invalid request options rejected before HTTP", func(t *testing.T) {
		hit := false
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				hit = true
				return nil, nil
			}),
		}
		p := New("k", WithHTTPClient(client))

		invalidReqs := []*VoiceListRequest{
			{PageSize: -1},
			{PageSize: MaxVoicesPageSize + 1},
			{SortDirection: "up"},
		}
		for _, req := range invalidReqs {
			if _, err := p.ListVoices(context.Background(), req); !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("ListVoices(%+v) err = %v, want ErrBadRequest", req, err)
			}
		}
		if hit {
			t.Error("invalid VoiceListRequest must not issue HTTP calls")
		}
	})
}

func TestListAllVoicesPaginationCycleGuardAndCancellation(t *testing.T) {
	t.Run("collects all pages without mutating caller request", func(t *testing.T) {
		var tokens []string
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				tok := req.URL.Query().Get("next_page_token")
				tokens = append(tokens, tok)
				switch tok {
				case "":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"voices":[{"voice_id":"v1","name":"One"}],"has_more":true,"next_page_token":"p2"}`)),
					}, nil
				case "p2":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"voices":[{"voice_id":"v2","name":"Two"}],"has_more":true,"next_page_token":"p3"}`)),
					}, nil
				default:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"voices":[{"voice_id":"v3","name":"Three"}],"has_more":false}`)),
					}, nil
				}
			}),
		}

		p := New("k", WithHTTPClient(client))
		callerReq := &VoiceListRequest{Category: "premade"}
		voices, err := p.ListAllVoices(context.Background(), callerReq)
		if err != nil {
			t.Fatalf("ListAllVoices() error = %v", err)
		}
		if len(voices) != 3 || voices[0].VoiceID != "v1" || voices[2].VoiceID != "v3" {
			t.Fatalf("voices = %+v, want 3 voices", voices)
		}
		if callerReq.NextPageToken != "" || callerReq.PageSize != 0 {
			t.Errorf("caller request was mutated: %+v", callerReq)
		}
		if len(tokens) != 3 || tokens[0] != "" || tokens[1] != "p2" || tokens[2] != "p3" {
			t.Errorf("tokens = %v", tokens)
		}
	})

	t.Run("duplicate next_page_token aborts with ErrDecode", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"voices":[{"voice_id":"v1"}],"has_more":true,"next_page_token":"stuck"}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		_, err := p.ListAllVoices(context.Background(), nil)
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("cyclic token err = %v, want core.ErrDecode", err)
		}
	})

	t.Run("cancellation between pages stops pagination", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var calls int
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				cancel()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"voices":[{"voice_id":"v1"}],"has_more":true,"next_page_token":"p2"}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		_, err := p.ListAllVoices(ctx, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
	})
}

func TestGetVoiceAndVoiceSettings(t *testing.T) {
	t.Run("GetVoice success and path escaping", func(t *testing.T) {
		var capturedPath string
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				capturedPath = req.URL.EscapedPath()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"voice_id": "voice/1",
						"name": "Custom",
						"category": "premade",
						"settings": {"stability": 0.4, "similarity_boost": 0.8, "speed": 1.05}
					}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		v, err := p.GetVoice(context.Background(), "voice/1")
		if err != nil {
			t.Fatalf("GetVoice() error = %v", err)
		}
		if capturedPath != "/v1/voices/voice%2F1" {
			t.Errorf("EscapedPath = %q", capturedPath)
		}
		if v.Name != "Custom" || v.Settings == nil || *v.Settings.Speed != 1.05 {
			t.Errorf("voice = %+v", v)
		}
	})

	t.Run("GetDefaultVoiceSettings and GetVoiceSettings", func(t *testing.T) {
		var paths []string
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"stability":0.5,"similarity_boost":0.75,"style":0.0,"use_speaker_boost":true,"speed":1.0}`)),
				}, nil
			}),
		}

		p := New("k", WithHTTPClient(client))
		def, err := p.GetDefaultVoiceSettings(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultVoiceSettings() error = %v", err)
		}
		vs, err := p.GetVoiceSettings(context.Background(), "v_123")
		if err != nil {
			t.Fatalf("GetVoiceSettings() error = %v", err)
		}
		if def.Validate() != nil || vs.Validate() != nil {
			t.Error("expected valid VoiceSettings")
		}
		if *def.Stability != 0.5 || *vs.SimilarityBoost != 0.75 || !*vs.UseSpeakerBoost {
			t.Errorf("settings mismatch: def=%+v vs=%+v", def, vs)
		}
		if len(paths) != 2 || paths[0] != "/v1/voices/settings/default" || paths[1] != "/v1/voices/v_123/settings" {
			t.Errorf("paths = %v", paths)
		}
	})

	t.Run("blank voice ID rejected before HTTP", func(t *testing.T) {
		hit := false
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				hit = true
				return nil, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		if _, err := p.GetVoice(context.Background(), "   "); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("GetVoice(blank) err = %v, want ErrBadRequest", err)
		}
		if _, err := p.GetVoiceSettings(context.Background(), ""); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("GetVoiceSettings(blank) err = %v, want ErrBadRequest", err)
		}
		if hit {
			t.Error("blank voice_id must not issue HTTP calls")
		}
	})

	t.Run("HTTP errors, bad JSON, timeout, and cancellation", func(t *testing.T) {
		errClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/missing") {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"detail":"voice not found"}`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{invalid`)),
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(errClient))
		if _, err := p.GetVoice(context.Background(), "missing"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("404 err = %v, want ErrNotFound", err)
		}
		if _, err := p.GetDefaultVoiceSettings(context.Background()); !errors.Is(err, core.ErrDecode) {
			t.Errorf("bad json err = %v, want ErrDecode", err)
		}

		blockingClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		}
		pTimeout := New("k", WithHTTPClient(blockingClient), WithTimeout(20*time.Millisecond))
		if _, err := pTimeout.ListVoices(context.Background(), nil); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("timeout err = %v, want context.DeadlineExceeded", err)
		}
	})
}
