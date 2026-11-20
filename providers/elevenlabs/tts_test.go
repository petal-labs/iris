package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func floatPtr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64     { return &v }
func boolPtr(v bool) *bool        { return &v }

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
func (r errReader) Close() error             { return nil }

func TestResolveOutputFormat(t *testing.T) {
	validCases := []struct {
		name     string
		in       core.AudioFormat
		wantWire string
		wantEnc  core.AudioEncoding
		wantSR   int
		wantBR   int
	}{
		{"zero default", core.AudioFormat{}, "mp3_44100_128", core.AudioEncodingMP3, 44100, 128},
		{"bare mp3", core.AudioFormat{Encoding: core.AudioEncodingMP3}, "mp3_44100_128", core.AudioEncodingMP3, 44100, 128},
		{"mp3 22050 32", core.AudioFormatMP3_22050_32, "mp3_22050_32", core.AudioEncodingMP3, 22050, 32},
		{"mp3 44100 64", core.AudioFormatMP3_44100_64, "mp3_44100_64", core.AudioEncodingMP3, 44100, 64},
		{"mp3 44100 128", core.AudioFormatMP3_44100_128, "mp3_44100_128", core.AudioEncodingMP3, 44100, 128},
		{"mp3 44100 192", core.AudioFormatMP3_44100_192, "mp3_44100_192", core.AudioEncodingMP3, 44100, 192},
		{"bare pcm", core.AudioFormat{Encoding: core.AudioEncodingPCM}, "pcm_16000", core.AudioEncodingPCM, 16000, 0},
		{"pcm 24000", core.AudioFormatPCM24000, "pcm_24000", core.AudioEncodingPCM, 24000, 0},
		{"pcm 44100", core.AudioFormatPCM44100, "pcm_44100", core.AudioEncodingPCM, 44100, 0},
		{"pcm 48000", core.AudioFormat{Encoding: core.AudioEncodingPCM, SampleRate: 48000}, "pcm_48000", core.AudioEncodingPCM, 48000, 0},
		{"ulaw 8000", core.AudioFormatULaw8000, "ulaw_8000", core.AudioEncodingULaw, 8000, 0},
		{"bare ulaw", core.AudioFormat{Encoding: core.AudioEncodingULaw}, "ulaw_8000", core.AudioEncodingULaw, 8000, 0},
		{"alaw 8000", core.AudioFormatALaw8000, "alaw_8000", core.AudioEncodingALaw, 8000, 0},
		{"opus 48000 64", core.AudioFormatOpus48000_64, "opus_48000_64", core.AudioEncodingOpus, 48000, 64},
		{"bare opus", core.AudioFormat{Encoding: core.AudioEncodingOpus}, "opus_48000_64", core.AudioEncodingOpus, 48000, 64},
		{"opus 48000 128", core.AudioFormat{Encoding: core.AudioEncodingOpus, SampleRate: 48000, BitRate: 128}, "opus_48000_128", core.AudioEncodingOpus, 48000, 128},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			wire, resolved, err := resolveOutputFormat(tc.in)
			if err != nil {
				t.Fatalf("resolveOutputFormat(%+v) err = %v", tc.in, err)
			}
			if wire != tc.wantWire {
				t.Errorf("wire = %q, want %q", wire, tc.wantWire)
			}
			if resolved.Encoding != tc.wantEnc || resolved.SampleRate != tc.wantSR || resolved.BitRate != tc.wantBR || resolved.Channels != 1 {
				t.Errorf("resolved = %+v, want enc=%s sr=%d br=%d ch=1", resolved, tc.wantEnc, tc.wantSR, tc.wantBR)
			}
		})
	}

	invalidCases := []struct {
		name string
		in   core.AudioFormat
	}{
		{"invalid encoding", core.AudioFormat{Encoding: "wma"}},
		{"unsupported wav", core.AudioFormatWAV16000},
		{"unsupported flac", core.AudioFormat{Encoding: core.AudioEncodingFLAC}},
		{"stereo rejected", core.AudioFormat{Encoding: core.AudioEncodingMP3, Channels: 2}},
		{"mp3 bad sample rate", core.AudioFormat{Encoding: core.AudioEncodingMP3, SampleRate: 16000}},
		{"mp3 22050 bad bitrate", core.AudioFormat{Encoding: core.AudioEncodingMP3, SampleRate: 22050, BitRate: 128}},
		{"mp3 44100 bad bitrate", core.AudioFormat{Encoding: core.AudioEncodingMP3, SampleRate: 44100, BitRate: 256}},
		{"pcm bad sample rate", core.AudioFormat{Encoding: core.AudioEncodingPCM, SampleRate: 11025}},
		{"ulaw bad sample rate", core.AudioFormat{Encoding: core.AudioEncodingULaw, SampleRate: 16000}},
		{"alaw bad sample rate", core.AudioFormat{Encoding: core.AudioEncodingALaw, SampleRate: 16000}},
		{"opus bad sample rate", core.AudioFormat{Encoding: core.AudioEncodingOpus, SampleRate: 24000}},
		{"opus bad bitrate", core.AudioFormat{Encoding: core.AudioEncodingOpus, SampleRate: 48000, BitRate: 256}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := resolveOutputFormat(tc.in)
			if !errors.Is(err, core.ErrInvalidAudioFormat) {
				t.Errorf("resolveOutputFormat(%+v) err = %v, want core.ErrInvalidAudioFormat", tc.in, err)
			}
		})
	}
}

func TestSynthesizeSuccessFullOptionsAndMetadata(t *testing.T) {
	var capturedPath, capturedQuery, capturedKey, capturedCT string
	var capturedBody ttsRequestBody

	audioOut := []byte{0x49, 0x44, 0x33, 0x04, 0x00, 0x00} // synthetic MP3 header bytes
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedPath = req.URL.Path
			capturedQuery = req.URL.RawQuery
			capturedKey = req.Header.Get("xi-api-key")
			capturedCT = req.Header.Get("Content-Type")
			if err := json.NewDecoder(req.Body).Decode(&capturedBody); err != nil {
				t.Errorf("failed to decode request body: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type":   []string{"audio/mpeg; charset=binary"},
					"Request-Id":     []string{"req_tts_777"},
					"Character-Cost": []string{"17"},
				},
				Body: io.NopCloser(bytes.NewReader(audioOut)),
			}, nil
		}),
	}

	p := New("sk_tts_key", WithHTTPClient(client))
	sdkClient := core.NewClient(p)

	req := &core.SpeechRequest{
		Model:    ModelElevenFlashV25,
		VoiceID:  "JBFqnCBsd6RMkjVDRZzb",
		Text:     "Hello from Iris!!",
		Format:   core.AudioFormatMP3_44100_192,
		Language: "en",
		VoiceSettings: &core.VoiceSettings{
			Stability:       floatPtr(0.6),
			SimilarityBoost: floatPtr(0.8),
			Style:           floatPtr(0.2),
			UseSpeakerBoost: boolPtr(true),
			Speed:           floatPtr(1.1),
		},
		Seed:         int64Ptr(12345),
		PreviousText: "Before.",
		NextText:     "After.",
	}

	resp, err := sdkClient.Synthesize(context.Background(), req)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if capturedPath != "/v1/text-to-speech/JBFqnCBsd6RMkjVDRZzb" {
		t.Errorf("path = %q", capturedPath)
	}
	if capturedQuery != "output_format=mp3_44100_192" {
		t.Errorf("query = %q", capturedQuery)
	}
	if capturedKey != "sk_tts_key" || capturedCT != "application/json" {
		t.Errorf("headers = (key=%q, ct=%q)", capturedKey, capturedCT)
	}
	if capturedBody.Text != "Hello from Iris!!" || capturedBody.ModelID != ModelElevenFlashV25 || capturedBody.LanguageCode != "en" {
		t.Errorf("capturedBody = %+v", capturedBody)
	}
	if capturedBody.VoiceSettings == nil || *capturedBody.VoiceSettings.Stability != 0.6 || *capturedBody.VoiceSettings.Speed != 1.1 || !*capturedBody.VoiceSettings.UseSpeakerBoost {
		t.Errorf("capturedBody.VoiceSettings = %+v", capturedBody.VoiceSettings)
	}
	if capturedBody.Seed == nil || *capturedBody.Seed != 12345 || capturedBody.PreviousText != "Before." || capturedBody.NextText != "After." {
		t.Errorf("capturedBody continuity/seed mismatch: %+v", capturedBody)
	}

	if resp.ID != "req_tts_777" || resp.Model != ModelElevenFlashV25 || resp.VoiceID != "JBFqnCBsd6RMkjVDRZzb" {
		t.Errorf("resp metadata = %+v", resp)
	}
	if !bytes.Equal(resp.Audio.Data, audioOut) {
		t.Errorf("resp.Audio.Data = %x, want %x", resp.Audio.Data, audioOut)
	}
	if resp.Audio.Format.Encoding != core.AudioEncodingMP3 || resp.Audio.Format.BitRate != 192 || resp.Audio.Format.MIMEType != "audio/mpeg" {
		t.Errorf("resp.Audio.Format = %+v", resp.Audio.Format)
	}
	if resp.Usage.InputCharacters != 17 || resp.Usage.OutputBytes != int64(len(audioOut)) {
		t.Errorf("resp.Usage = %+v", resp.Usage)
	}
}

func TestSynthesizeDefaultsAndVoiceEscaping(t *testing.T) {
	var capturedEscapedPath, capturedQuery string
	var capturedBody ttsRequestBody

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedEscapedPath = req.URL.EscapedPath()
			capturedQuery = req.URL.RawQuery
			_ = json.NewDecoder(req.Body).Decode(&capturedBody)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Character-Cost": []string{"not-a-number"}},
				Body:       io.NopCloser(strings.NewReader("pcm-bytes")),
			}, nil
		}),
	}

	p := New("k", WithHTTPClient(client))
	resp, err := p.Synthesize(context.Background(), &core.SpeechRequest{
		VoiceID: "voice/with space",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if capturedEscapedPath != "/v1/text-to-speech/voice%2Fwith%20space" {
		t.Errorf("EscapedPath = %q", capturedEscapedPath)
	}
	if capturedQuery != "output_format=mp3_44100_128" {
		t.Errorf("query = %q, want default mp3_44100_128", capturedQuery)
	}
	if capturedBody.ModelID != DefaultTTSModel {
		t.Errorf("ModelID = %q, want %q", capturedBody.ModelID, DefaultTTSModel)
	}
	if resp.Model != DefaultTTSModel {
		t.Errorf("resp.Model = %q, want %q", resp.Model, DefaultTTSModel)
	}
	if resp.Usage.InputCharacters != 0 {
		t.Errorf("invalid character-cost should yield 0, got %d", resp.Usage.InputCharacters)
	}
}

func TestSynthesizeFailuresAndCancellation(t *testing.T) {
	t.Run("preflight validation and format errors never hit HTTP", func(t *testing.T) {
		hit := false
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				hit = true
				return nil, errors.New("unexpected HTTP call")
			}),
		}
		p := New("k", WithHTTPClient(client))

		if _, err := p.Synthesize(context.Background(), nil); !errors.Is(err, core.ErrInvalidSpeechRequest) {
			t.Errorf("nil req err = %v, want ErrInvalidSpeechRequest", err)
		}
		if _, err := p.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi", Format: core.AudioFormatWAV16000}); !errors.Is(err, core.ErrInvalidAudioFormat) {
			t.Errorf("unsupported wav err = %v, want ErrInvalidAudioFormat", err)
		}
		if hit {
			t.Error("preflight failures must not issue HTTP requests")
		}
	})

	t.Run("empty API key fails before HTTP", func(t *testing.T) {
		hit := false
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				hit = true
				return nil, nil
			}),
		}
		p := New("", WithHTTPClient(client))
		if _, err := p.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"}); !errors.Is(err, core.ErrUnauthorized) {
			t.Errorf("err = %v, want ErrUnauthorized", err)
		}
		if hit {
			t.Error("empty API key must not issue HTTP request")
		}
	})

	t.Run("typed HTTP errors propagate", func(t *testing.T) {
		statuses := []struct {
			code int
			body string
			want error
		}{
			{http.StatusUnauthorized, `{"detail":{"status":"invalid_api_key","message":"bad key"}}`, core.ErrUnauthorized},
			{http.StatusNotFound, `{"detail":"voice not found"}`, core.ErrNotFound},
			{http.StatusUnprocessableEntity, `{"detail":{"code":"invalid_parameters","message":"bad param"}}`, core.ErrBadRequest},
			{http.StatusTooManyRequests, `{"detail":"rate limited"}`, core.ErrRateLimited},
			{http.StatusInternalServerError, `server boom`, core.ErrServer},
		}
		for _, tc := range statuses {
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tc.code,
						Header:     http.Header{"Request-Id": []string{"req-err"}},
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					}, nil
				}),
			}
			p := New("k", WithHTTPClient(client))
			_, err := p.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
			if !errors.Is(err, tc.want) {
				t.Errorf("status %d err = %v, want %v", tc.code, err, tc.want)
			}
		}
	})

	t.Run("empty audio body returns ErrDecode", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		_, err := p.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("empty audio err = %v, want ErrDecode", err)
		}
	})

	t.Run("body read failures preserve cancellation and network errors", func(t *testing.T) {
		netClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       errReader{err: errors.New("broken pipe")},
				}, nil
			}),
		}
		pNet := New("k", WithHTTPClient(netClient))
		if _, err := pNet.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"}); !errors.Is(err, core.ErrNetwork) {
			t.Errorf("read err = %v, want ErrNetwork", err)
		}

		cancelClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       errReader{err: context.Canceled},
				}, nil
			}),
		}
		pCancel := New("k", WithHTTPClient(cancelClient))
		if _, err := pCancel.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"}); !errors.Is(err, context.Canceled) {
			t.Errorf("read cancel err = %v, want context.Canceled", err)
		}
	})

	t.Run("context cancellation and timeout", func(t *testing.T) {
		blockingClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		}
		p := New("k", WithHTTPClient(blockingClient), WithTimeout(20*time.Millisecond))
		if _, err := p.Synthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"}); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("timeout err = %v, want context.DeadlineExceeded", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		p2 := New("k", WithHTTPClient(blockingClient), WithTimeout(time.Second))
		if _, err := p2.Synthesize(ctx, &core.SpeechRequest{VoiceID: "v1", Text: "hi"}); !errors.Is(err, context.Canceled) {
			t.Errorf("cancel err = %v, want context.Canceled", err)
		}
	})
}

func TestStreamSynthesizeNotYetSupported(t *testing.T) {
	p := New("k")
	_, err := p.StreamSynthesize(context.Background(), &core.SpeechRequest{VoiceID: "v1", Text: "hi"})
	if !errors.Is(err, core.ErrNotSupported) {
		t.Fatalf("StreamSynthesize() err = %v, want core.ErrNotSupported", err)
	}
}
