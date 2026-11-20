package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }
func int64Ptr(v int64) *int64     { return &v }
func boolPtr(v bool) *bool        { return &v }

func TestAudioEncodingValidationAndMetadata(t *testing.T) {
	tests := []struct {
		enc   AudioEncoding
		valid bool
		mime  string
		ext   string
	}{
		{AudioEncodingMP3, true, "audio/mpeg", "mp3"},
		{AudioEncodingPCM, true, "audio/pcm", "pcm"},
		{AudioEncodingWAV, true, "audio/wav", "wav"},
		{AudioEncodingOpus, true, "audio/opus", "opus"},
		{AudioEncodingOGG, true, "audio/ogg", "ogg"},
		{AudioEncodingFLAC, true, "audio/flac", "flac"},
		{AudioEncodingAAC, true, "audio/aac", "aac"},
		{AudioEncodingWebM, true, "audio/webm", "webm"},
		{AudioEncodingULaw, true, "audio/basic", "ulaw"},
		{AudioEncodingALaw, true, "audio/PCMA", "alaw"},
		{AudioEncoding(""), false, "", ""},
		{AudioEncoding("midi"), false, "", ""},
	}

	for _, tt := range tests {
		if got := tt.enc.IsValid(); got != tt.valid {
			t.Errorf("AudioEncoding(%q).IsValid() = %v, want %v", tt.enc, got, tt.valid)
		}
		if got := tt.enc.MIMEType(); got != tt.mime {
			t.Errorf("AudioEncoding(%q).MIMEType() = %q, want %q", tt.enc, got, tt.mime)
		}
		if got := tt.enc.FileExtension(); got != tt.ext {
			t.Errorf("AudioEncoding(%q).FileExtension() = %q, want %q", tt.enc, got, tt.ext)
		}
	}
}

func TestAudioFormatPresetsAndValidation(t *testing.T) {
	presets := []AudioFormat{
		AudioFormatMP3_22050_32,
		AudioFormatMP3_44100_64,
		AudioFormatMP3_44100_128,
		AudioFormatMP3_44100_192,
		AudioFormatPCM16000,
		AudioFormatPCM22050,
		AudioFormatPCM24000,
		AudioFormatPCM44100,
		AudioFormatULaw8000,
		AudioFormatALaw8000,
		AudioFormatOpus48000_64,
		AudioFormatWAV16000,
	}
	for _, p := range presets {
		if p.IsZero() {
			t.Errorf("preset %v reported IsZero() == true", p)
		}
		if err := p.Validate(); err != nil {
			t.Errorf("preset %v Validate() error = %v", p, err)
		}
		if !p.IsValid() {
			t.Errorf("preset %v IsValid() = false", p)
		}
		if p.EffectiveMIMEType() == "" {
			t.Errorf("preset %v EffectiveMIMEType() is empty", p)
		}
	}

	var zero AudioFormat
	if !zero.IsZero() {
		t.Error("zero AudioFormat should report IsZero() == true")
	}
	if got := zero.String(); got != "default" {
		t.Errorf("zero AudioFormat.String() = %q, want %q", got, "default")
	}

	minimal := AudioFormat{Encoding: AudioEncodingMP3}
	if err := minimal.Validate(); err != nil {
		t.Fatalf("minimal AudioFormat.Validate() = %v", err)
	}
	if got := minimal.EffectiveMIMEType(); got != "audio/mpeg" {
		t.Errorf("minimal EffectiveMIMEType() = %q, want audio/mpeg", got)
	}
	if got := minimal.String(); got != "mp3" {
		t.Errorf("minimal String() = %q, want mp3", got)
	}

	customMIME := AudioFormat{Encoding: AudioEncodingPCM, SampleRate: 16000, Channels: 1, MIMEType: "audio/L16;rate=16000"}
	if err := customMIME.Validate(); err != nil {
		t.Fatalf("customMIME.Validate() = %v", err)
	}
	if got := customMIME.EffectiveMIMEType(); got != "audio/L16;rate=16000" {
		t.Errorf("customMIME.EffectiveMIMEType() = %q", got)
	}

	invalidCases := []struct {
		name   string
		format AudioFormat
	}{
		{"empty", AudioFormat{}},
		{"unknown encoding", AudioFormat{Encoding: "wma"}},
		{"negative sample rate", AudioFormat{Encoding: AudioEncodingPCM, SampleRate: -16000}},
		{"negative bit rate", AudioFormat{Encoding: AudioEncodingMP3, BitRate: -128}},
		{"negative channels", AudioFormat{Encoding: AudioEncodingWAV, Channels: -1}},
		{"whitespace mime", AudioFormat{Encoding: AudioEncodingMP3, MIMEType: "audio/mpeg bad"}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.format.Validate()
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !errors.Is(err, ErrInvalidAudioFormat) {
				t.Errorf("err = %v, want errors.Is(_, ErrInvalidAudioFormat)", err)
			}
			if tc.format.IsValid() {
				t.Errorf("IsValid() = true for invalid format %s", tc.name)
			}
		})
	}
}

func TestAudioFormatJSONRoundTrip(t *testing.T) {
	in := AudioFormatMP3_44100_128
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var out AudioFormat
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestAudioDataValidationBytesAndPrivacy(t *testing.T) {
	rawPayload := []byte("SENSITIVE_RAW_AUDIO_BYTES_12345")
	b64Payload := base64.StdEncoding.EncodeToString(rawPayload)

	validRaw := AudioData{
		Data:     rawPayload,
		Format:   AudioFormatMP3_44100_128,
		Duration: 1500 * time.Millisecond,
		Filename: "greeting.mp3",
	}
	if validRaw.IsEmpty() {
		t.Error("validRaw.IsEmpty() = true")
	}
	if err := validRaw.Validate(); err != nil {
		t.Fatalf("validRaw.Validate() = %v", err)
	}
	gotBytes, err := validRaw.GetBytes()
	if err != nil || !bytes.Equal(gotBytes, rawPayload) {
		t.Fatalf("validRaw.GetBytes() = (%q, %v), want %q", gotBytes, err, rawPayload)
	}

	// Privacy: String and GoString must never emit raw bytes or base64 payload.
	if s := validRaw.String(); strings.Contains(s, "SENSITIVE_RAW_AUDIO") {
		t.Errorf("AudioData.String() leaked raw audio: %s", s)
	}
	if s := fmt.Sprintf("%#v", validRaw); strings.Contains(s, "SENSITIVE_RAW_AUDIO") {
		t.Errorf("AudioData.GoString() leaked raw audio: %s", s)
	}

	validB64 := AudioData{
		Base64: b64Payload,
		Format: AudioFormatPCM16000,
	}
	if err := validB64.Validate(); err != nil {
		t.Fatalf("validB64.Validate() = %v", err)
	}
	gotBytes, err = validB64.GetBytes()
	if err != nil || !bytes.Equal(gotBytes, rawPayload) {
		t.Fatalf("validB64.GetBytes() = (%q, %v), want %q", gotBytes, err, rawPayload)
	}
	if s := validB64.String(); strings.Contains(s, b64Payload) {
		t.Errorf("AudioData.String() leaked base64 audio: %s", s)
	}

	// Data takes precedence over Base64 when both are populated.
	both := AudioData{
		Data:   []byte("primary"),
		Base64: base64.StdEncoding.EncodeToString([]byte("secondary")),
		Format: AudioFormatWAV16000,
	}
	gotBytes, err = both.GetBytes()
	if err != nil || string(gotBytes) != "primary" {
		t.Fatalf("Data precedence failed: got (%q, %v)", gotBytes, err)
	}

	// Empty GetBytes returns nil, nil
	var empty AudioData
	if b, err := empty.GetBytes(); b != nil || err != nil {
		t.Errorf("empty.GetBytes() = (%v, %v), want (nil, nil)", b, err)
	}

	invalidCases := []struct {
		name    string
		data    AudioData
		wantErr error
	}{
		{
			name:    "empty",
			data:    AudioData{Format: AudioFormatMP3_44100_128},
			wantErr: ErrInvalidAudioData,
		},
		{
			name:    "invalid base64",
			data:    AudioData{Base64: "not-valid-base64!!!", Format: AudioFormatMP3_44100_128},
			wantErr: ErrInvalidAudioData,
		},
		{
			name:    "invalid format",
			data:    AudioData{Data: []byte("abc"), Format: AudioFormat{Encoding: "bogus"}},
			wantErr: ErrInvalidAudioFormat,
		},
		{
			name:    "negative duration",
			data:    AudioData{Data: []byte("abc"), Format: AudioFormatPCM16000, Duration: -time.Second},
			wantErr: ErrInvalidAudioData,
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.data.Validate()
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !errors.Is(err, ErrInvalidAudioData) {
				t.Errorf("err = %v, want errors.Is(_, ErrInvalidAudioData)", err)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}

	// JSON round-trip
	b, err := json.Marshal(validRaw)
	if err != nil {
		t.Fatalf("Marshal AudioData error: %v", err)
	}
	var decoded AudioData
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("Unmarshal AudioData error: %v", err)
	}
	if !bytes.Equal(decoded.Data, validRaw.Data) || decoded.Format != validRaw.Format || decoded.Duration != validRaw.Duration || decoded.Filename != validRaw.Filename {
		t.Errorf("AudioData JSON round-trip mismatch: got %+v, want %+v", decoded, validRaw)
	}
}

func TestAudioChunkValidationSerializationAndPrivacy(t *testing.T) {
	validChunk := AudioChunk{
		Data:      []byte("SECRET_AUDIO_CHUNK_BYTES"),
		Format:    AudioFormatPCM24000,
		Sequence:  0,
		Timestamp: 20 * time.Millisecond,
		Duration:  20 * time.Millisecond,
	}
	if err := validChunk.Validate(); err != nil {
		t.Fatalf("validChunk.Validate() = %v", err)
	}
	if s := validChunk.String(); strings.Contains(s, "SECRET_AUDIO_CHUNK") {
		t.Errorf("AudioChunk.String() leaked audio bytes: %s", s)
	}
	if s := fmt.Sprintf("%#v", validChunk); strings.Contains(s, "SECRET_AUDIO_CHUNK") {
		t.Errorf("AudioChunk.GoString() leaked audio bytes: %s", s)
	}

	// Terminal chunk with empty data is allowed.
	finalMarker := AudioChunk{Sequence: 5, IsFinal: true}
	if err := finalMarker.Validate(); err != nil {
		t.Fatalf("finalMarker.Validate() = %v", err)
	}

	// Chunk with inherited (zero) format and data is allowed.
	inherited := AudioChunk{Data: []byte{1, 2, 3}, Sequence: 1}
	if err := inherited.Validate(); err != nil {
		t.Fatalf("inherited.Validate() = %v", err)
	}

	invalidCases := []struct {
		name  string
		chunk AudioChunk
	}{
		{"negative sequence", AudioChunk{Data: []byte{1}, Sequence: -1}},
		{"negative timestamp", AudioChunk{Data: []byte{1}, Timestamp: -time.Millisecond}},
		{"negative duration", AudioChunk{Data: []byte{1}, Duration: -time.Millisecond}},
		{"invalid format", AudioChunk{Data: []byte{1}, Format: AudioFormat{Encoding: "nope"}}},
		{"non-final empty data", AudioChunk{Sequence: 0, IsFinal: false}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.chunk.Validate()
			if !errors.Is(err, ErrInvalidAudioChunk) {
				t.Errorf("Validate() = %v, want ErrInvalidAudioChunk", err)
			}
		})
	}

	// JSON round-trip
	b, err := json.Marshal(validChunk)
	if err != nil {
		t.Fatalf("Marshal AudioChunk error: %v", err)
	}
	var out AudioChunk
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal AudioChunk error: %v", err)
	}
	if !bytes.Equal(out.Data, validChunk.Data) || out.Format != validChunk.Format || out.Sequence != validChunk.Sequence || out.Timestamp != validChunk.Timestamp || out.Duration != validChunk.Duration {
		t.Errorf("AudioChunk round-trip mismatch: got %+v, want %+v", out, validChunk)
	}
}

func TestSpeechRequestValidationSerializationAndPrivacy(t *testing.T) {
	validReq := &SpeechRequest{
		Model:    "eleven_multilingual_v2",
		VoiceID:  "voice_123",
		Text:     "CONFIDENTIAL_SPOKEN_MESSAGE",
		Format:   AudioFormatMP3_44100_128,
		Language: "en",
		VoiceSettings: &VoiceSettings{
			Speed:           floatPtr(1.0),
			Stability:       floatPtr(0.5),
			SimilarityBoost: floatPtr(0.75),
			Style:           floatPtr(0.0),
			UseSpeakerBoost: boolPtr(true),
		},
		Seed:         int64Ptr(42),
		PreviousText: "Prior sentence.",
		NextText:     "Next sentence.",
		RequestID:    "req-1",
		Metadata:     map[string]string{"turn_id": "t-1"},
	}
	if err := validReq.Validate(); err != nil {
		t.Fatalf("validReq.Validate() = %v", err)
	}

	// Privacy: String and GoString must omit raw text.
	if s := validReq.String(); strings.Contains(s, "CONFIDENTIAL_SPOKEN_MESSAGE") {
		t.Errorf("SpeechRequest.String() leaked text: %s", s)
	}
	if s := fmt.Sprintf("%#v", *validReq); strings.Contains(s, "CONFIDENTIAL_SPOKEN_MESSAGE") {
		t.Errorf("SpeechRequest.GoString() leaked text: %s", s)
	}

	// Zero-value Format and nil VoiceSettings are allowed.
	minimal := &SpeechRequest{VoiceID: "v1", Text: "hello", Seed: int64Ptr(0)}
	if err := minimal.Validate(); err != nil {
		t.Fatalf("minimal.Validate() = %v", err)
	}

	invalidCases := []struct {
		name string
		req  *SpeechRequest
	}{
		{"nil request", nil},
		{"missing voice", &SpeechRequest{Text: "hello"}},
		{"whitespace voice", &SpeechRequest{VoiceID: "   ", Text: "hello"}},
		{"missing text", &SpeechRequest{VoiceID: "v1"}},
		{"whitespace text", &SpeechRequest{VoiceID: "v1", Text: "\t\n"}},
		{"invalid format", &SpeechRequest{VoiceID: "v1", Text: "hi", Format: AudioFormat{Encoding: "bad"}}},
		{"negative seed", &SpeechRequest{VoiceID: "v1", Text: "hi", Seed: int64Ptr(-1)}},
		{"speed zero", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Speed: floatPtr(0)}}},
		{"speed negative", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Speed: floatPtr(-1)}}},
		{"speed NaN", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Speed: floatPtr(math.NaN())}}},
		{"speed Inf", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Speed: floatPtr(math.Inf(1))}}},
		{"stability < 0", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Stability: floatPtr(-0.1)}}},
		{"stability > 1", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Stability: floatPtr(1.1)}}},
		{"similarity_boost > 1", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{SimilarityBoost: floatPtr(1.5)}}},
		{"style NaN", &SpeechRequest{VoiceID: "v1", Text: "hi", VoiceSettings: &VoiceSettings{Style: floatPtr(math.NaN())}}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if !errors.Is(err, ErrInvalidSpeechRequest) {
				t.Errorf("Validate() = %v, want ErrInvalidSpeechRequest", err)
			}
		})
	}

	// JSON round-trip
	b, err := json.Marshal(validReq)
	if err != nil {
		t.Fatalf("Marshal SpeechRequest error: %v", err)
	}
	var out SpeechRequest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal SpeechRequest error: %v", err)
	}
	if out.Model != validReq.Model || out.VoiceID != validReq.VoiceID || out.Text != validReq.Text || out.Format != validReq.Format {
		t.Errorf("SpeechRequest round-trip mismatch: got %+v", out)
	}
	if out.VoiceSettings == nil || *out.VoiceSettings.Stability != 0.5 || *out.VoiceSettings.Style != 0.0 || !*out.VoiceSettings.UseSpeakerBoost {
		t.Errorf("VoiceSettings round-trip mismatch: got %+v", out.VoiceSettings)
	}
	if out.Seed == nil || *out.Seed != 42 {
		t.Errorf("Seed round-trip mismatch: got %v", out.Seed)
	}
}

func TestSpeechResponseSerializationAndPrivacy(t *testing.T) {
	resp := SpeechResponse{
		ID:      "gen-1",
		Model:   "eleven_flash_v2_5",
		VoiceID: "voice_123",
		Audio: AudioData{
			Data:     []byte("RAW_AUDIO_SECRET_OUTPUT"),
			Format:   AudioFormatMP3_44100_128,
			Duration: 2 * time.Second,
		},
		Usage: SpeechUsage{
			InputCharacters: 25,
			AudioDuration:   2 * time.Second,
			OutputBytes:     23,
		},
	}

	if s := resp.String(); strings.Contains(s, "RAW_AUDIO_SECRET_OUTPUT") {
		t.Errorf("SpeechResponse.String() leaked audio: %s", s)
	}
	if s := fmt.Sprintf("%#v", resp); strings.Contains(s, "RAW_AUDIO_SECRET_OUTPUT") {
		t.Errorf("SpeechResponse.GoString() leaked audio: %s", s)
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal SpeechResponse error: %v", err)
	}
	var out SpeechResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal SpeechResponse error: %v", err)
	}
	if out.ID != resp.ID || out.Model != resp.Model || !bytes.Equal(out.Audio.Data, resp.Audio.Data) || out.Usage != resp.Usage {
		t.Errorf("SpeechResponse round-trip mismatch: got %+v, want %+v", out, resp)
	}
}

func TestSpeechStreamChannelDirections(t *testing.T) {
	ch := make(chan AudioChunk)
	errCh := make(chan error)
	finalCh := make(chan *SpeechResponse)

	stream := &SpeechStream{
		Ch:    ch,
		Err:   errCh,
		Final: finalCh,
	}
	if stream == nil {
		t.Fatal("stream should not be nil")
	}
}

func TestDrainSpeechStreamAccumulatesChunks(t *testing.T) {
	ch := make(chan AudioChunk, 3)
	errCh := make(chan error, 1)
	finalCh := make(chan *SpeechResponse, 1)

	go func() {
		ch <- AudioChunk{Data: []byte("part1-"), Format: AudioFormatPCM16000, Sequence: 0}
		ch <- AudioChunk{Data: []byte("part2"), Sequence: 1}
		ch <- AudioChunk{Sequence: 2, IsFinal: true}
		close(ch)
		finalCh <- &SpeechResponse{
			ID:      "req-1",
			Model:   "eleven_flash_v2_5",
			VoiceID: "v1",
			Usage:   SpeechUsage{InputCharacters: 11},
		}
		close(finalCh)
		close(errCh)
	}()

	resp, err := DrainSpeechStream(context.Background(), &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Audio.Data) != "part1-part2" {
		t.Errorf("Audio.Data = %q, want %q", resp.Audio.Data, "part1-part2")
	}
	if resp.Audio.Format != AudioFormatPCM16000 {
		t.Errorf("Audio.Format = %+v, want %+v", resp.Audio.Format, AudioFormatPCM16000)
	}
	if resp.Usage.OutputBytes != int64(len("part1-part2")) {
		t.Errorf("Usage.OutputBytes = %d, want %d", resp.Usage.OutputBytes, len("part1-part2"))
	}
	if resp.Usage.InputCharacters != 11 {
		t.Errorf("Usage.InputCharacters = %d, want 11", resp.Usage.InputCharacters)
	}
}

func TestDrainSpeechStreamPreservesFinalAudio(t *testing.T) {
	ch := make(chan AudioChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *SpeechResponse, 1)

	go func() {
		ch <- AudioChunk{Data: []byte("chunk"), Sequence: 0}
		close(ch)
		finalCh <- &SpeechResponse{
			Audio: AudioData{
				Data:   []byte("authoritative-final-audio"),
				Format: AudioFormatMP3_44100_128,
			},
		}
		close(finalCh)
		close(errCh)
	}()

	resp, err := DrainSpeechStream(context.Background(), &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Audio.Data) != "authoritative-final-audio" {
		t.Errorf("Audio.Data = %q, want authoritative-final-audio", resp.Audio.Data)
	}
	if resp.Audio.Format != AudioFormatMP3_44100_128 {
		t.Errorf("Audio.Format = %+v, want MP3 44100 128", resp.Audio.Format)
	}
}

func TestDrainSpeechStreamNoFinalResponse(t *testing.T) {
	ch := make(chan AudioChunk, 2)
	errCh := make(chan error, 1)
	finalCh := make(chan *SpeechResponse, 1)

	go func() {
		ch <- AudioChunk{Data: []byte("abc"), Format: AudioFormatULaw8000, Sequence: 0}
		ch <- AudioChunk{Data: []byte("def"), Sequence: 1}
		close(ch)
		close(finalCh)
		close(errCh)
	}()

	resp, err := DrainSpeechStream(context.Background(), &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Audio.Data) != "abcdef" {
		t.Errorf("Audio.Data = %q, want abcdef", resp.Audio.Data)
	}
	if resp.Audio.Format != AudioFormatULaw8000 {
		t.Errorf("Audio.Format = %+v, want ULaw8000", resp.Audio.Format)
	}
	if resp.Usage.OutputBytes != 6 {
		t.Errorf("Usage.OutputBytes = %d, want 6", resp.Usage.OutputBytes)
	}
}

func TestDrainSpeechStreamNilAndPartialChannels(t *testing.T) {
	if _, err := DrainSpeechStream(context.Background(), nil); !errors.Is(err, ErrNilSpeechStream) {
		t.Errorf("nil stream err = %v, want ErrNilSpeechStream", err)
	}

	// Nil context and only Ch populated should not panic or deadlock.
	ch := make(chan AudioChunk, 1)
	ch <- AudioChunk{Data: []byte("ok"), Sequence: 0}
	close(ch)

	//nolint:staticcheck // intentionally testing nil context fallback
	resp, err := DrainSpeechStream(nil, &SpeechStream{Ch: ch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Audio.Data) != "ok" {
		t.Errorf("Audio.Data = %q, want ok", resp.Audio.Data)
	}
}

func TestDrainSpeechStreamErrorAndLateError(t *testing.T) {
	ch := make(chan AudioChunk, 1)
	errCh := make(chan error, 1)
	finalCh := make(chan *SpeechResponse, 1)
	expectedErr := &ProviderError{Provider: "elevenlabs", Message: "mid-stream failure", Err: ErrServer}

	ch <- AudioChunk{Data: []byte("partial"), Sequence: 0}
	close(ch) // Ch closes before Err delivers

	go func() {
		time.Sleep(5 * time.Millisecond)
		errCh <- expectedErr
		close(errCh)
		close(finalCh)
	}()

	_, err := DrainSpeechStream(context.Background(), &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("err = %v, want %v", err, expectedErr)
	}
}

func TestDrainSpeechStreamCancellationAndTimeout(t *testing.T) {
	t.Run("cancelled context", func(t *testing.T) {
		ch := make(chan AudioChunk)
		errCh := make(chan error, 1)
		finalCh := make(chan *SpeechResponse, 1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := DrainSpeechStream(ctx, &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("concrete error wins over cancelled context", func(t *testing.T) {
		ch := make(chan AudioChunk)
		errCh := make(chan error, 1)
		finalCh := make(chan *SpeechResponse, 1)
		providerErr := errors.New("provider failure")
		errCh <- providerErr

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := DrainSpeechStream(ctx, &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
		if !errors.Is(err, providerErr) {
			t.Errorf("err = %v, want %v", err, providerErr)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		ch := make(chan AudioChunk)
		errCh := make(chan error, 1)
		finalCh := make(chan *SpeechResponse, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := DrainSpeechStream(ctx, &SpeechStream{Ch: ch, Err: errCh, Final: finalCh})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", err)
		}
	})
}

func TestTranscriptionTimestampGranularity(t *testing.T) {
	tests := []struct {
		g     TranscriptionTimestampGranularity
		valid bool
	}{
		{TimestampGranularityNone, true},
		{TimestampGranularityWord, true},
		{TimestampGranularitySegment, true},
		{TimestampGranularityCharacter, true},
		{TranscriptionTimestampGranularity("paragraph"), false},
	}
	for _, tt := range tests {
		if got := tt.g.IsValid(); got != tt.valid {
			t.Errorf("TranscriptionTimestampGranularity(%q).IsValid() = %v, want %v", tt.g, got, tt.valid)
		}
	}
}

func TestTranscriptionRequestValidationSerializationAndPrivacy(t *testing.T) {
	audio := &AudioData{
		Data:     []byte("SECRET_INPUT_AUDIO_BYTES"),
		Format:   AudioFormatWAV16000,
		Filename: "call.wav",
	}
	validAudioReq := &TranscriptionRequest{
		Model:          "scribe_v2",
		Audio:          audio,
		Language:       "en",
		Prompt:         "CONFIDENTIAL_PROMPT_TEXT",
		Keyterms:       []string{"PetalFlow", "Cortex"},
		Diarize:        true,
		NumSpeakers:    intPtr(2),
		Timestamps:     TimestampGranularityWord,
		TagAudioEvents: true,
		Temperature:    floatPtr(0.0),
		MaxBytes:       1024,
		RequestID:      "tr-1",
		Metadata:       map[string]string{"session": "s-1"},
	}
	if err := validAudioReq.Validate(); err != nil {
		t.Fatalf("validAudioReq.Validate() = %v", err)
	}

	// Privacy: String and GoString must omit raw audio and prompt text.
	for _, formatted := range []string{validAudioReq.String(), fmt.Sprintf("%#v", *validAudioReq)} {
		if strings.Contains(formatted, "SECRET_INPUT_AUDIO_BYTES") {
			t.Errorf("TranscriptionRequest leaked audio bytes: %s", formatted)
		}
		if strings.Contains(formatted, "CONFIDENTIAL_PROMPT_TEXT") {
			t.Errorf("TranscriptionRequest leaked prompt text: %s", formatted)
		}
	}

	// Reader-backed request
	validReaderReq := &TranscriptionRequest{
		Model:    "scribe_v2",
		Reader:   bytes.NewReader([]byte("streamed-audio")),
		Format:   AudioFormatMP3_44100_128,
		Filename: "recording.mp3",
	}
	if err := validReaderReq.Validate(); err != nil {
		t.Fatalf("validReaderReq.Validate() = %v", err)
	}
	if s := validReaderReq.String(); !strings.Contains(s, "recording.mp3") {
		t.Errorf("reader request String() = %s, expected filename hint", s)
	}

	invalidCases := []struct {
		name string
		req  *TranscriptionRequest
	}{
		{"nil request", nil},
		{"no audio or reader", &TranscriptionRequest{Model: "scribe_v2"}},
		{"both audio and reader", &TranscriptionRequest{Audio: audio, Reader: bytes.NewReader([]byte("x"))}},
		{"invalid audio", &TranscriptionRequest{Audio: &AudioData{Format: AudioFormatWAV16000}}},
		{"exceeds max bytes", &TranscriptionRequest{Audio: audio, MaxBytes: 4}},
		{"negative max bytes", &TranscriptionRequest{Audio: audio, MaxBytes: -1}},
		{"invalid format hint", &TranscriptionRequest{Reader: bytes.NewReader([]byte("x")), Format: AudioFormat{Encoding: "nope"}}},
		{"invalid timestamps", &TranscriptionRequest{Audio: audio, Timestamps: "invalid"}},
		{"num speakers zero", &TranscriptionRequest{Audio: audio, NumSpeakers: intPtr(0)}},
		{"num speakers negative", &TranscriptionRequest{Audio: audio, NumSpeakers: intPtr(-2)}},
		{"temperature negative", &TranscriptionRequest{Audio: audio, Temperature: floatPtr(-0.1)}},
		{"temperature NaN", &TranscriptionRequest{Audio: audio, Temperature: floatPtr(math.NaN())}},
		{"temperature Inf", &TranscriptionRequest{Audio: audio, Temperature: floatPtr(math.Inf(1))}},
		{"empty keyterm", &TranscriptionRequest{Audio: audio, Keyterms: []string{"valid", "  "}}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if !errors.Is(err, ErrInvalidTranscriptionRequest) {
				t.Errorf("Validate() = %v, want ErrInvalidTranscriptionRequest", err)
			}
		})
	}

	// JSON round-trip (Reader omitted, other fields preserved)
	b, err := json.Marshal(validAudioReq)
	if err != nil {
		t.Fatalf("Marshal TranscriptionRequest error: %v", err)
	}
	var out TranscriptionRequest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal TranscriptionRequest error: %v", err)
	}
	if out.Model != validAudioReq.Model || out.Language != validAudioReq.Language || !out.Diarize || !out.TagAudioEvents || out.Timestamps != TimestampGranularityWord {
		t.Errorf("TranscriptionRequest round-trip mismatch: got %+v", out)
	}
	if out.Audio == nil || !bytes.Equal(out.Audio.Data, audio.Data) {
		t.Errorf("TranscriptionRequest.Audio round-trip mismatch: got %+v", out.Audio)
	}
	if len(out.Keyterms) != 2 || out.Keyterms[0] != "PetalFlow" {
		t.Errorf("Keyterms round-trip mismatch: got %v", out.Keyterms)
	}
	if out.Reader != nil {
		t.Errorf("expected nil Reader after JSON unmarshal")
	}
}

func TestTranscriptionResponseValidationSerializationAndPrivacy(t *testing.T) {
	validResp := &TranscriptionResponse{
		ID:                  "tr-resp-1",
		Model:               "scribe_v2",
		Text:                "PRIVATE_TRANSCRIPT_CONTENT",
		Language:            "en",
		LanguageProbability: floatPtr(0.99),
		Duration:            2500 * time.Millisecond,
		Words: []TranscriptionWord{
			{
				Text:       "PRIVATE",
				Start:      0,
				End:        400 * time.Millisecond,
				Type:       "word",
				SpeakerID:  "speaker_0",
				Confidence: floatPtr(0.98),
				LogProb:    floatPtr(-0.02),
			},
			{
				Text:      " ",
				Start:     400 * time.Millisecond,
				End:       450 * time.Millisecond,
				Type:      "spacing",
				SpeakerID: "speaker_0",
			},
		},
		Segments: []TranscriptionSegment{
			{
				ID:         0,
				Text:       "PRIVATE_TRANSCRIPT_CONTENT",
				Start:      0,
				End:        2500 * time.Millisecond,
				SpeakerID:  "speaker_0",
				Confidence: floatPtr(0.97),
				Words: []TranscriptionWord{
					{Text: "PRIVATE", Start: 0, End: 400 * time.Millisecond, Type: "word", SpeakerID: "speaker_0"},
				},
			},
		},
		AudioEvents: []AudioEvent{
			{
				Type:       "laughter",
				Start:      1 * time.Second,
				End:        1500 * time.Millisecond,
				SpeakerID:  "speaker_1",
				Confidence: floatPtr(0.9),
			},
		},
		IsFinal: true,
		Usage: TranscriptionUsage{
			AudioDuration: 2500 * time.Millisecond,
			InputBytes:    4096,
		},
	}

	if err := validResp.Validate(); err != nil {
		t.Fatalf("validResp.Validate() = %v", err)
	}

	// Privacy: String and GoString must never emit raw transcript text.
	for _, formatted := range []string{validResp.String(), fmt.Sprintf("%#v", *validResp)} {
		if strings.Contains(formatted, "PRIVATE_TRANSCRIPT_CONTENT") {
			t.Errorf("TranscriptionResponse leaked transcript text: %s", formatted)
		}
	}

	invalidCases := []struct {
		name string
		resp *TranscriptionResponse
	}{
		{"nil response", nil},
		{"negative duration", &TranscriptionResponse{Duration: -time.Second}},
		{"language probability > 1", &TranscriptionResponse{LanguageProbability: floatPtr(1.2)}},
		{"language probability < 0", &TranscriptionResponse{LanguageProbability: floatPtr(-0.1)}},
		{"negative usage duration", &TranscriptionResponse{Usage: TranscriptionUsage{AudioDuration: -time.Second}}},
		{"negative usage input bytes", &TranscriptionResponse{Usage: TranscriptionUsage{InputBytes: -1}}},
		{"word negative start", &TranscriptionResponse{Words: []TranscriptionWord{{Text: "a", Start: -time.Millisecond, End: 0}}}},
		{"word end before start", &TranscriptionResponse{Words: []TranscriptionWord{{Text: "a", Start: time.Second, End: 500 * time.Millisecond}}}},
		{"word invalid confidence", &TranscriptionResponse{Words: []TranscriptionWord{{Text: "a", Confidence: floatPtr(2.0)}}}},
		{"word NaN logprob", &TranscriptionResponse{Words: []TranscriptionWord{{Text: "a", LogProb: floatPtr(math.NaN())}}}},
		{"segment negative start", &TranscriptionResponse{Segments: []TranscriptionSegment{{Text: "a", Start: -time.Second}}}},
		{"segment end before start", &TranscriptionResponse{Segments: []TranscriptionSegment{{Text: "a", Start: time.Second, End: time.Millisecond}}}},
		{"segment invalid confidence", &TranscriptionResponse{Segments: []TranscriptionSegment{{Text: "a", Confidence: floatPtr(-0.5)}}}},
		{"segment invalid nested word", &TranscriptionResponse{Segments: []TranscriptionSegment{{Text: "a", Words: []TranscriptionWord{{Text: "a", Start: time.Second, End: 0}}}}}},
		{"audio event missing type", &TranscriptionResponse{AudioEvents: []AudioEvent{{Type: "  "}}}},
		{"audio event negative start", &TranscriptionResponse{AudioEvents: []AudioEvent{{Type: "laughter", Start: -time.Millisecond}}}},
		{"audio event negative end", &TranscriptionResponse{AudioEvents: []AudioEvent{{Type: "laughter", End: -time.Millisecond}}}},
		{"audio event end before start", &TranscriptionResponse{AudioEvents: []AudioEvent{{Type: "laughter", Start: time.Second, End: 500 * time.Millisecond}}}},
		{"audio event invalid confidence", &TranscriptionResponse{AudioEvents: []AudioEvent{{Type: "laughter", Confidence: floatPtr(1.5)}}}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.resp.Validate()
			if !errors.Is(err, ErrInvalidTranscriptionResponse) {
				t.Errorf("Validate() = %v, want ErrInvalidTranscriptionResponse", err)
			}
		})
	}

	// JSON round-trip
	b, err := json.Marshal(validResp)
	if err != nil {
		t.Fatalf("Marshal TranscriptionResponse error: %v", err)
	}
	var out TranscriptionResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal TranscriptionResponse error: %v", err)
	}
	if out.ID != validResp.ID || out.Text != validResp.Text || out.Language != "en" || !out.IsFinal || out.Duration != validResp.Duration {
		t.Errorf("TranscriptionResponse round-trip mismatch: got %+v", out)
	}
	if len(out.Words) != 2 || out.Words[0].SpeakerID != "speaker_0" || *out.Words[0].LogProb != -0.02 {
		t.Errorf("Words round-trip mismatch: got %+v", out.Words)
	}
	if len(out.Segments) != 1 || len(out.Segments[0].Words) != 1 {
		t.Errorf("Segments round-trip mismatch: got %+v", out.Segments)
	}
	if len(out.AudioEvents) != 1 || out.AudioEvents[0].Type != "laughter" {
		t.Errorf("AudioEvents round-trip mismatch: got %+v", out.AudioEvents)
	}
}
