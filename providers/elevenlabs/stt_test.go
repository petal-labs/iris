package elevenlabs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
)

func intPtr(v int) *int { return &v }

func parseMultipartRequest(t *testing.T, req *http.Request) (fields map[string][]string, filename string, fileBytes []byte) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType(%q) err = %v", req.Header.Get("Content-Type"), err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("mediaType = %q, want multipart/form-data", mediaType)
	}
	mr := multipart.NewReader(req.Body, params["boundary"])
	fields = make(map[string][]string)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() err = %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading part %q err = %v", part.FormName(), err)
		}
		if part.FormName() == "file" {
			filename = part.FileName()
			fileBytes = data
		} else {
			fields[part.FormName()] = append(fields[part.FormName()], string(data))
		}
	}
	return fields, filename, fileBytes
}

func TestTranscribeAudioDataFullOptionsAndResponseMapping(t *testing.T) {
	var capturedPath, capturedKey string
	var capturedFields map[string][]string
	var capturedFilename string
	var capturedFile []byte

	audioBytes := []byte("fake-mp3-audio-payload")
	scribeJSON := `{
		"transcription_id": "tr_scribe_99",
		"language_code": "en",
		"language_probability": 0.99,
		"text": "Hello Iris!",
		"words": [
			{"text": "Hello", "start": 0.0, "end": 0.4, "type": "word", "speaker_id": "speaker_0", "logprob": -0.01},
			{"text": " ", "start": 0.4, "end": 0.45, "type": "spacing", "speaker_id": "speaker_0"},
			{"text": "Iris!", "start": 0.45, "end": 0.9, "type": "word", "speaker_id": "speaker_1", "logprob": -0.03}
		],
		"audio_events": [
			{"type": "laughter", "start": 0.9, "end": 1.25, "speaker_id": "speaker_1", "confidence": 0.92}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedPath = req.URL.Path
			capturedKey = req.Header.Get("xi-api-key")
			capturedFields, capturedFilename, capturedFile = parseMultipartRequest(t, req)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Request-Id": []string{"hdr_req_1"}},
				Body:       io.NopCloser(strings.NewReader(scribeJSON)),
			}, nil
		}),
	}

	p := New("sk_stt_key", WithHTTPClient(client))
	sdkClient := core.NewClient(p)

	resp, err := sdkClient.Transcribe(context.Background(), &core.TranscriptionRequest{
		Model: ModelScribeV2,
		Audio: &core.AudioData{
			Data:     audioBytes,
			Format:   core.AudioFormatMP3_44100_128,
			Filename: "meeting.mp3",
		},
		Language:       "en",
		Keyterms:       []string{"Iris", "PetalFlow"},
		Diarize:        true,
		NumSpeakers:    intPtr(2),
		Timestamps:     core.TimestampGranularityWord,
		TagAudioEvents: true,
		Temperature:    floatPtr(0.2),
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if err := resp.Validate(); err != nil {
		t.Fatalf("resp.Validate() error = %v", err)
	}

	if capturedPath != "/v1/speech-to-text" || capturedKey != "sk_stt_key" {
		t.Errorf("request mismatch: path=%q key=%q", capturedPath, capturedKey)
	}
	if capturedFilename != "meeting.mp3" || !bytes.Equal(capturedFile, audioBytes) {
		t.Errorf("uploaded file mismatch: name=%q bytes=%q", capturedFilename, capturedFile)
	}
	wantSingle := map[string]string{
		"model_id":               string(ModelScribeV2),
		"language_code":          "en",
		"diarize":                "true",
		"num_speakers":           "2",
		"timestamps_granularity": "word",
		"tag_audio_events":       "true",
		"temperature":            "0.2",
	}
	for k, want := range wantSingle {
		if got := capturedFields[k]; len(got) != 1 || got[0] != want {
			t.Errorf("field %q = %v, want [%q]", k, got, want)
		}
	}
	if kt := capturedFields["keyterms"]; len(kt) != 2 || kt[0] != "Iris" || kt[1] != "PetalFlow" {
		t.Errorf("keyterms = %v, want [Iris PetalFlow]", kt)
	}

	if resp.ID != "tr_scribe_99" || resp.Model != ModelScribeV2 || resp.Text != "Hello Iris!" || resp.Language != "en" || !resp.IsFinal {
		t.Errorf("resp core fields mismatch: %+v", resp)
	}
	if resp.LanguageProbability == nil || *resp.LanguageProbability != 0.99 {
		t.Errorf("LanguageProbability = %v, want 0.99", resp.LanguageProbability)
	}
	if len(resp.Words) != 3 || resp.Words[0].Start != 0 || resp.Words[0].End != 400*time.Millisecond || resp.Words[2].SpeakerID != "speaker_1" {
		t.Errorf("Words mismatch: %+v", resp.Words)
	}
	if len(resp.AudioEvents) != 1 || resp.AudioEvents[0].Type != "laughter" || resp.AudioEvents[0].End != 1250*time.Millisecond {
		t.Errorf("AudioEvents mismatch: %+v", resp.AudioEvents)
	}
	if resp.Duration != 1250*time.Millisecond || resp.Usage.AudioDuration != 1250*time.Millisecond || resp.Usage.InputBytes != int64(len(audioBytes)) {
		t.Errorf("Duration/Usage mismatch: dur=%s usage=%+v", resp.Duration, resp.Usage)
	}
}

func TestTranscribeReaderDefaultsPCMAndHeaderIDFallback(t *testing.T) {
	var capturedFields map[string][]string
	var capturedFilename string
	var capturedFile []byte

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedFields, capturedFilename, capturedFile = parseMultipartRequest(t, req)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Request-Id": []string{"hdr_fallback_id"}},
				Body:       io.NopCloser(strings.NewReader(`{"text":"pcm transcript","language_code":"en"}`)),
			}, nil
		}),
	}

	p := New("k", WithHTTPClient(client))
	resp, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
		Reader: bytes.NewReader([]byte("raw-pcm-16k-bytes")),
		Format: core.AudioFormatPCM16000,
	})
	if err != nil {
		t.Fatalf("Transcribe(Reader PCM) error = %v", err)
	}
	if capturedFilename != "audio.pcm" {
		t.Errorf("derived filename = %q, want audio.pcm", capturedFilename)
	}
	if string(capturedFile) != "raw-pcm-16k-bytes" {
		t.Errorf("uploaded bytes = %q", capturedFile)
	}
	if got := capturedFields["model_id"]; len(got) != 1 || got[0] != string(DefaultSTTModel) {
		t.Errorf("model_id = %v, want [%q]", got, DefaultSTTModel)
	}
	if got := capturedFields["file_format"]; len(got) != 1 || got[0] != "pcm_s16le_16" {
		t.Errorf("file_format = %v, want [pcm_s16le_16]", got)
	}
	if resp.ID != "hdr_fallback_id" || resp.Model != DefaultSTTModel || !resp.IsFinal {
		t.Errorf("resp = %+v", resp)
	}

	// Single-byte reader without format or filename falls back to "audio.bin".
	resp2, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
		Reader: bytes.NewReader([]byte("x")),
	})
	if err != nil {
		t.Fatalf("Transcribe(1-byte reader) error = %v", err)
	}
	if capturedFilename != "audio.bin" || string(capturedFile) != "x" || resp2.Usage.InputBytes != 1 {
		t.Errorf("1-byte reader mismatch: name=%q file=%q bytes=%d", capturedFilename, capturedFile, resp2.Usage.InputBytes)
	}
}

func TestTranscribeLimitValidationBeforeHTTP(t *testing.T) {
	hit := false
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hit = true
			return nil, errors.New("should not be called")
		}),
	}
	p := New("k", WithHTTPClient(client))
	audio := &core.AudioData{Data: []byte("abc"), Format: core.AudioFormatMP3_44100_128}

	tooManyKeyterms := make([]string, MaxScribeKeyterms+1)
	for i := range tooManyKeyterms {
		tooManyKeyterms[i] = "term"
	}

	cases := []struct {
		name    string
		req     *core.TranscriptionRequest
		wantErr error
	}{
		{"nil request", nil, core.ErrInvalidTranscriptionRequest},
		{"too many keyterms", &core.TranscriptionRequest{Audio: audio, Keyterms: tooManyKeyterms}, core.ErrInvalidTranscriptionRequest},
		{"keyterms on scribe_v1", &core.TranscriptionRequest{Model: ModelScribeV1, Audio: audio, Keyterms: []string{"Iris"}}, core.ErrInvalidTranscriptionRequest},
		{"too many speakers", &core.TranscriptionRequest{Audio: audio, NumSpeakers: intPtr(MaxScribeSpeakers + 1)}, core.ErrInvalidTranscriptionRequest},
		{"temperature > 2.0", &core.TranscriptionRequest{Audio: audio, Temperature: floatPtr(2.5)}, core.ErrInvalidTranscriptionRequest},
		{"segment timestamps unsupported", &core.TranscriptionRequest{Audio: audio, Timestamps: core.TimestampGranularitySegment}, core.ErrInvalidTranscriptionRequest},
		{"raw pcm wrong sample rate", &core.TranscriptionRequest{Audio: &core.AudioData{Data: []byte("x"), Format: core.AudioFormatPCM24000}}, core.ErrInvalidAudioFormat},
		{"raw pcm stereo", &core.TranscriptionRequest{Audio: &core.AudioData{Data: []byte("x"), Format: core.AudioFormat{Encoding: core.AudioEncodingPCM, SampleRate: 16000, Channels: 2}}}, core.ErrInvalidAudioFormat},
		{"empty reader", &core.TranscriptionRequest{Reader: bytes.NewReader(nil)}, core.ErrInvalidTranscriptionRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Transcribe(context.Background(), tc.req)
			if !errors.Is(err, core.ErrInvalidTranscriptionRequest) || !errors.Is(err, tc.wantErr) {
				t.Errorf("Transcribe() err = %v, want %v", err, tc.wantErr)
			}
		})
	}

	// Empty API key fails before reading Reader or hitting HTTP.
	pEmpty := New("", WithHTTPClient(client))
	if _, err := pEmpty.Transcribe(context.Background(), &core.TranscriptionRequest{Reader: errReader{err: errors.New("should not read")}}); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("empty key err = %v, want ErrUnauthorized", err)
	}

	if hit {
		t.Error("preflight validation failures must not issue HTTP calls")
	}
}

func TestTranscribeReaderExceedingMaxBytes(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			_, err := io.Copy(io.Discard, req.Body)
			return nil, err
		}),
	}
	p := New("k", WithHTTPClient(client))

	_, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
		Reader:   bytes.NewReader(bytes.Repeat([]byte("a"), 100)),
		MaxBytes: 16,
	})
	if !errors.Is(err, core.ErrInvalidTranscriptionRequest) {
		t.Fatalf("oversized reader err = %v, want ErrInvalidTranscriptionRequest", err)
	}
}

func TestTranscribeHTTPErrorsDecodeAndCancellation(t *testing.T) {
	t.Run("typed HTTP error", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
					Header:     http.Header{"Request-Id": []string{"req-stt-422"}},
					Body:       io.NopCloser(strings.NewReader(`{"detail":{"code":"invalid_parameters","message":"bad audio"}}`)),
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		_, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
			Audio: &core.AudioData{Data: []byte("abc"), Format: core.AudioFormatMP3_44100_128},
		})
		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("422 err = %v, want ErrBadRequest", err)
		}
	})

	t.Run("invalid JSON response returns ErrDecode", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, _ = io.Copy(io.Discard, req.Body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{not valid json`)),
				}, nil
			}),
		}
		p := New("k", WithHTTPClient(client))
		_, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
			Audio: &core.AudioData{Data: []byte("abc"), Format: core.AudioFormatMP3_44100_128},
		})
		if !errors.Is(err, core.ErrDecode) {
			t.Fatalf("bad JSON err = %v, want ErrDecode", err)
		}
	})

	t.Run("reader read error during peek", func(t *testing.T) {
		p := New("k")
		_, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
			Reader: errReader{err: context.Canceled},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("reader cancel err = %v, want context.Canceled", err)
		}
	})

	t.Run("context cancellation and timeout clean up writer goroutine", func(t *testing.T) {
		blockingClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		}
		p := New("k", WithHTTPClient(blockingClient), WithTimeout(20*time.Millisecond))
		_, err := p.Transcribe(context.Background(), &core.TranscriptionRequest{
			Audio: &core.AudioData{Data: []byte("abc"), Format: core.AudioFormatMP3_44100_128},
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout err = %v, want context.DeadlineExceeded", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		p2 := New("k", WithHTTPClient(blockingClient), WithTimeout(time.Second))
		_, err = p2.Transcribe(ctx, &core.TranscriptionRequest{
			Audio: &core.AudioData{Data: []byte("abc"), Format: core.AudioFormatMP3_44100_128},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel err = %v, want context.Canceled", err)
		}
	})
}

func TestSecondsToDurationEdgeCases(t *testing.T) {
	if got := secondsToDuration(-1); got != 0 {
		t.Errorf("negative seconds = %s, want 0", got)
	}
	if got := secondsToDuration(0.45); got != 450*time.Millisecond {
		t.Errorf("0.45s = %s, want 450ms", got)
	}
}
