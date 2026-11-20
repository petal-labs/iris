package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// Audio validation and streaming sentinel errors.
var (
	// ErrInvalidAudioFormat indicates an unrecognized or malformed AudioFormat.
	ErrInvalidAudioFormat = errors.New("invalid audio format")

	// ErrInvalidAudioData indicates missing, malformed, or oversized AudioData.
	ErrInvalidAudioData = errors.New("invalid audio data")

	// ErrInvalidAudioChunk indicates a malformed AudioChunk.
	ErrInvalidAudioChunk = errors.New("invalid audio chunk")

	// ErrInvalidSpeechRequest indicates a SpeechRequest failed validation.
	ErrInvalidSpeechRequest = errors.New("invalid speech request")

	// ErrInvalidTranscriptionRequest indicates a TranscriptionRequest failed validation.
	ErrInvalidTranscriptionRequest = errors.New("invalid transcription request")

	// ErrInvalidTranscriptionResponse indicates a TranscriptionResponse or its timing metadata is malformed.
	ErrInvalidTranscriptionResponse = errors.New("invalid transcription response")

	// ErrNilSpeechStream indicates that DrainSpeechStream was called with a nil stream.
	ErrNilSpeechStream = errors.New("nil speech stream")
)

// AudioEncoding identifies the wire codec or container of an audio payload.
type AudioEncoding string

const (
	// AudioEncodingMP3 is MPEG-1/2 Audio Layer III.
	AudioEncodingMP3 AudioEncoding = "mp3"
	// AudioEncodingPCM is raw linear signed 16-bit little-endian PCM audio.
	AudioEncodingPCM AudioEncoding = "pcm"
	// AudioEncodingWAV is RIFF/WAVE container audio.
	AudioEncodingWAV AudioEncoding = "wav"
	// AudioEncodingOpus is Opus-encoded audio.
	AudioEncodingOpus AudioEncoding = "opus"
	// AudioEncodingOGG is Ogg container audio.
	AudioEncodingOGG AudioEncoding = "ogg"
	// AudioEncodingFLAC is Free Lossless Audio Codec.
	AudioEncodingFLAC AudioEncoding = "flac"
	// AudioEncodingAAC is Advanced Audio Coding.
	AudioEncodingAAC AudioEncoding = "aac"
	// AudioEncodingWebM is WebM container audio.
	AudioEncodingWebM AudioEncoding = "webm"
	// AudioEncodingULaw is G.711 mu-law telephony audio.
	AudioEncodingULaw AudioEncoding = "ulaw"
	// AudioEncodingALaw is G.711 A-law telephony audio.
	AudioEncodingALaw AudioEncoding = "alaw"
)

// IsValid reports whether the audio encoding is a recognized value.
func (e AudioEncoding) IsValid() bool {
	switch e {
	case AudioEncodingMP3,
		AudioEncodingPCM,
		AudioEncodingWAV,
		AudioEncodingOpus,
		AudioEncodingOGG,
		AudioEncodingFLAC,
		AudioEncodingAAC,
		AudioEncodingWebM,
		AudioEncodingULaw,
		AudioEncodingALaw:
		return true
	default:
		return false
	}
}

// MIMEType returns the canonical MIME type for the encoding, or "" if unknown.
func (e AudioEncoding) MIMEType() string {
	switch e {
	case AudioEncodingMP3:
		return "audio/mpeg"
	case AudioEncodingPCM:
		return "audio/pcm"
	case AudioEncodingWAV:
		return "audio/wav"
	case AudioEncodingOpus:
		return "audio/opus"
	case AudioEncodingOGG:
		return "audio/ogg"
	case AudioEncodingFLAC:
		return "audio/flac"
	case AudioEncodingAAC:
		return "audio/aac"
	case AudioEncodingWebM:
		return "audio/webm"
	case AudioEncodingULaw:
		return "audio/basic"
	case AudioEncodingALaw:
		return "audio/PCMA"
	default:
		return ""
	}
}

// FileExtension returns the canonical file extension (without leading dot)
// for the encoding, or "" if unknown.
func (e AudioEncoding) FileExtension() string {
	if !e.IsValid() {
		return ""
	}
	return string(e)
}

// AudioFormat describes the encoding and audio parameters of an audio payload
// or requested synthesis output.
//
// A zero-value AudioFormat means "provider default" when supplied on requests.
// SampleRate is in Hz and BitRate is in kilobits per second (kbps).
type AudioFormat struct {
	// Encoding is the codec or container format (e.g., mp3, pcm, wav, opus, ulaw).
	Encoding AudioEncoding `json:"encoding"`

	// SampleRate is the sampling frequency in Hz (e.g., 8000, 16000, 22050, 24000, 44100, 48000).
	// Zero means unspecified or provider default.
	SampleRate int `json:"sample_rate,omitempty"`

	// BitRate is the target bitrate in kbps (e.g., 32, 64, 128, 192).
	// Zero means unspecified, lossless, or provider default.
	BitRate int `json:"bit_rate,omitempty"`

	// Channels is the number of audio channels (e.g., 1 for mono, 2 for stereo).
	// Zero means unspecified or provider default.
	Channels int `json:"channels,omitempty"`

	// MIMEType optionally overrides the default MIME type derived from Encoding.
	MIMEType string `json:"mime_type,omitempty"`
}

// Common AudioFormat presets aligned with speech synthesis, transcription, and telephony workflows.
var (
	AudioFormatMP3_22050_32  = AudioFormat{Encoding: AudioEncodingMP3, SampleRate: 22050, BitRate: 32, Channels: 1, MIMEType: "audio/mpeg"}
	AudioFormatMP3_44100_64  = AudioFormat{Encoding: AudioEncodingMP3, SampleRate: 44100, BitRate: 64, Channels: 1, MIMEType: "audio/mpeg"}
	AudioFormatMP3_44100_128 = AudioFormat{Encoding: AudioEncodingMP3, SampleRate: 44100, BitRate: 128, Channels: 1, MIMEType: "audio/mpeg"}
	AudioFormatMP3_44100_192 = AudioFormat{Encoding: AudioEncodingMP3, SampleRate: 44100, BitRate: 192, Channels: 1, MIMEType: "audio/mpeg"}
	AudioFormatPCM16000      = AudioFormat{Encoding: AudioEncodingPCM, SampleRate: 16000, Channels: 1, MIMEType: "audio/pcm"}
	AudioFormatPCM22050      = AudioFormat{Encoding: AudioEncodingPCM, SampleRate: 22050, Channels: 1, MIMEType: "audio/pcm"}
	AudioFormatPCM24000      = AudioFormat{Encoding: AudioEncodingPCM, SampleRate: 24000, Channels: 1, MIMEType: "audio/pcm"}
	AudioFormatPCM44100      = AudioFormat{Encoding: AudioEncodingPCM, SampleRate: 44100, Channels: 1, MIMEType: "audio/pcm"}
	AudioFormatULaw8000      = AudioFormat{Encoding: AudioEncodingULaw, SampleRate: 8000, Channels: 1, MIMEType: "audio/basic"}
	AudioFormatALaw8000      = AudioFormat{Encoding: AudioEncodingALaw, SampleRate: 8000, Channels: 1, MIMEType: "audio/PCMA"}
	AudioFormatOpus48000_64  = AudioFormat{Encoding: AudioEncodingOpus, SampleRate: 48000, BitRate: 64, Channels: 1, MIMEType: "audio/opus"}
	AudioFormatWAV16000      = AudioFormat{Encoding: AudioEncodingWAV, SampleRate: 16000, Channels: 1, MIMEType: "audio/wav"}
)

// IsZero reports whether all fields of the AudioFormat are unset.
func (f AudioFormat) IsZero() bool {
	return f.Encoding == "" &&
		f.SampleRate == 0 &&
		f.BitRate == 0 &&
		f.Channels == 0 &&
		f.MIMEType == ""
}

// EffectiveMIMEType returns MIMEType if explicitly set, otherwise the canonical
// MIME type for Encoding.
func (f AudioFormat) EffectiveMIMEType() string {
	if f.MIMEType != "" {
		return f.MIMEType
	}
	return f.Encoding.MIMEType()
}

// Validate checks that the AudioFormat carries a recognized encoding and
// non-negative audio parameters. Returned errors wrap [ErrInvalidAudioFormat].
func (f AudioFormat) Validate() error {
	if !f.Encoding.IsValid() {
		return fmt.Errorf("%w: unsupported encoding %q", ErrInvalidAudioFormat, f.Encoding)
	}
	if f.SampleRate < 0 {
		return fmt.Errorf("%w: sample_rate must be non-negative, got %d", ErrInvalidAudioFormat, f.SampleRate)
	}
	if f.BitRate < 0 {
		return fmt.Errorf("%w: bit_rate must be non-negative, got %d", ErrInvalidAudioFormat, f.BitRate)
	}
	if f.Channels < 0 {
		return fmt.Errorf("%w: channels must be non-negative, got %d", ErrInvalidAudioFormat, f.Channels)
	}
	if f.MIMEType != "" && strings.ContainsAny(f.MIMEType, " \t\r\n") {
		return fmt.Errorf("%w: mime_type must not contain whitespace", ErrInvalidAudioFormat)
	}
	return nil
}

// IsValid reports whether the AudioFormat passes [AudioFormat.Validate].
func (f AudioFormat) IsValid() bool {
	return f.Validate() == nil
}

// String returns a concise, log-safe description of the AudioFormat.
func (f AudioFormat) String() string {
	if f.IsZero() {
		return "default"
	}
	var details []string
	if f.SampleRate > 0 {
		details = append(details, fmt.Sprintf("%dHz", f.SampleRate))
	}
	if f.BitRate > 0 {
		details = append(details, fmt.Sprintf("%dkbps", f.BitRate))
	}
	if f.Channels > 0 {
		details = append(details, fmt.Sprintf("%dch", f.Channels))
	}
	if len(details) == 0 {
		return string(f.Encoding)
	}
	return fmt.Sprintf("%s (%s)", f.Encoding, strings.Join(details, ", "))
}

// AudioData represents a complete in-memory audio payload with its format and
// metadata.
//
// Callers may populate either Data (raw audio bytes) or Base64 (standard
// base64-encoded audio bytes). Data takes precedence when both are present.
//
// Privacy: String and GoString emit metadata summaries only and never include
// raw audio bytes or base64 payloads.
type AudioData struct {
	// Data holds the raw audio bytes. In JSON, encoding/json marshals []byte as base64.
	Data []byte `json:"data,omitempty"`

	// Base64 holds standard base64-encoded audio bytes when Data is not populated.
	Base64 string `json:"base64,omitempty"`

	// Format describes the encoding and audio parameters of the payload.
	Format AudioFormat `json:"format"`

	// Duration is the playback duration of the audio if known.
	Duration time.Duration `json:"duration_ns,omitempty"`

	// Filename is an optional filename hint (e.g., "clip.mp3") for multipart uploads.
	Filename string `json:"filename,omitempty"`
}

// IsEmpty reports whether neither Data nor Base64 carries audio content.
func (d AudioData) IsEmpty() bool {
	return len(d.Data) == 0 && d.Base64 == ""
}

// GetBytes returns the raw audio bytes, decoding Base64 if Data is empty.
// Decoding failures wrap [ErrInvalidAudioData].
func (d AudioData) GetBytes() ([]byte, error) {
	if len(d.Data) > 0 {
		return d.Data, nil
	}
	if d.Base64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(d.Base64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid base64 payload: %w", ErrInvalidAudioData, err)
		}
		return decoded, nil
	}
	return nil, nil
}

// Validate checks that AudioData has non-empty, decodable bytes, a valid
// Format, and a non-negative Duration. Returned errors wrap [ErrInvalidAudioData]
// (and [ErrInvalidAudioFormat] when Format is invalid).
func (d AudioData) Validate() error {
	if d.IsEmpty() {
		return fmt.Errorf("%w: audio payload is empty", ErrInvalidAudioData)
	}
	if len(d.Data) == 0 && d.Base64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(d.Base64)
		if err != nil {
			return fmt.Errorf("%w: invalid base64 payload: %w", ErrInvalidAudioData, err)
		}
		if len(decoded) == 0 {
			return fmt.Errorf("%w: decoded audio payload is empty", ErrInvalidAudioData)
		}
	}
	if err := d.Format.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAudioData, err)
	}
	if d.Duration < 0 {
		return fmt.Errorf("%w: duration must be non-negative, got %s", ErrInvalidAudioData, d.Duration)
	}
	return nil
}

// String returns a log-safe metadata summary without raw audio bytes.
func (d AudioData) String() string {
	size := len(d.Data)
	sizeKind := "bytes"
	if size == 0 && d.Base64 != "" {
		size = len(d.Base64)
		sizeKind = "base64_chars"
	}
	return fmt.Sprintf("AudioData{format=%s, %s=%d, duration=%s, filename=%q}",
		d.Format.String(), sizeKind, size, d.Duration, d.Filename)
}

// GoString returns a log-safe representation for %#v formatting without raw audio bytes.
func (d AudioData) GoString() string {
	return "core." + d.String()
}

// AudioChunk represents a single incremental chunk of streaming audio.
//
// Chunks carry raw encoded or PCM bytes along with ordering and timing metadata.
// A terminal chunk with IsFinal set may carry empty Data to signal end-of-stream.
//
// Privacy: String and GoString omit raw audio bytes to prevent accidental logging.
type AudioChunk struct {
	// Data contains the raw audio bytes for this chunk.
	Data []byte `json:"data,omitempty"`

	// Format optionally identifies the chunk's audio format. Zero means inherited
	// from the enclosing stream or session configuration.
	Format AudioFormat `json:"format,omitempty"`

	// Sequence is the zero-based, monotonically increasing chunk index.
	Sequence int64 `json:"sequence"`

	// Timestamp is the optional offset from the start of the stream or turn.
	Timestamp time.Duration `json:"timestamp_ns,omitempty"`

	// Duration is the optional playback duration of this chunk.
	Duration time.Duration `json:"duration_ns,omitempty"`

	// IsFinal indicates the terminal chunk of a stream or synthesized turn.
	IsFinal bool `json:"is_final,omitempty"`
}

// Validate checks that AudioChunk has non-negative ordering/timing values,
// non-empty Data unless IsFinal is true, and a valid Format when specified.
// Returned errors wrap [ErrInvalidAudioChunk].
func (c AudioChunk) Validate() error {
	if c.Sequence < 0 {
		return fmt.Errorf("%w: sequence must be non-negative, got %d", ErrInvalidAudioChunk, c.Sequence)
	}
	if c.Timestamp < 0 {
		return fmt.Errorf("%w: timestamp must be non-negative, got %s", ErrInvalidAudioChunk, c.Timestamp)
	}
	if c.Duration < 0 {
		return fmt.Errorf("%w: duration must be non-negative, got %s", ErrInvalidAudioChunk, c.Duration)
	}
	if !c.Format.IsZero() {
		if err := c.Format.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAudioChunk, err)
		}
	}
	if len(c.Data) == 0 && !c.IsFinal {
		return fmt.Errorf("%w: non-final chunk must contain audio data", ErrInvalidAudioChunk)
	}
	return nil
}

// String returns a log-safe metadata summary without raw audio bytes.
func (c AudioChunk) String() string {
	return fmt.Sprintf("AudioChunk{seq=%d, bytes=%d, format=%s, final=%t}",
		c.Sequence, len(c.Data), c.Format.String(), c.IsFinal)
}

// GoString returns a log-safe representation for %#v formatting without raw audio bytes.
func (c AudioChunk) GoString() string {
	return "core." + c.String()
}

// VoiceSettings configures optional, provider-agnostic voice synthesis tuning.
// Pointer fields allow distinguishing unset (provider default) from explicit zero values.
type VoiceSettings struct {
	// Speed controls playback speed multiplier (must be > 0; e.g., 1.0 is normal).
	Speed *float64 `json:"speed,omitempty"`

	// Stability controls voice consistency versus expressiveness in [0.0, 1.0].
	Stability *float64 `json:"stability,omitempty"`

	// SimilarityBoost controls adherence to the target voice timbre in [0.0, 1.0].
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`

	// Style controls style exaggeration in [0.0, 1.0].
	Style *float64 `json:"style,omitempty"`

	// UseSpeakerBoost enables additional speaker similarity enhancement when supported.
	UseSpeakerBoost *bool `json:"use_speaker_boost,omitempty"`
}

// Validate checks that any populated VoiceSettings fields are finite and within
// their valid ranges. Returned errors wrap [ErrInvalidSpeechRequest].
func (v *VoiceSettings) Validate() error {
	if v == nil {
		return nil
	}
	if v.Speed != nil {
		if math.IsNaN(*v.Speed) || math.IsInf(*v.Speed, 0) || *v.Speed <= 0 {
			return fmt.Errorf("%w: voice_settings.speed must be a finite number > 0, got %v", ErrInvalidSpeechRequest, *v.Speed)
		}
	}
	if err := validateUnitInterval("voice_settings.stability", v.Stability, ErrInvalidSpeechRequest); err != nil {
		return err
	}
	if err := validateUnitInterval("voice_settings.similarity_boost", v.SimilarityBoost, ErrInvalidSpeechRequest); err != nil {
		return err
	}
	if err := validateUnitInterval("voice_settings.style", v.Style, ErrInvalidSpeechRequest); err != nil {
		return err
	}
	return nil
}

// SpeechRequest represents a provider-agnostic text-to-speech synthesis request.
//
// Privacy: String and GoString omit raw synthesis text and report text length
// instead so accidental logging does not expose sensitive content.
type SpeechRequest struct {
	// Model specifies the speech synthesis model ID. Empty uses the provider default.
	Model ModelID `json:"model,omitempty"`

	// VoiceID identifies the voice to synthesize with. Required.
	VoiceID string `json:"voice_id"`

	// Text is the text content to synthesize into speech. Required.
	Text string `json:"text"`

	// Format specifies the desired output audio format. Zero uses the provider default.
	Format AudioFormat `json:"format,omitempty"`

	// Language is an optional ISO-639 / BCP-47 language code hint (e.g., "en").
	Language string `json:"language,omitempty"`

	// VoiceSettings specifies optional synthesis tuning knobs.
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`

	// Seed requests deterministic synthesis when supported by the provider (must be >= 0).
	Seed *int64 `json:"seed,omitempty"`

	// PreviousText provides preceding context to improve prosody continuity across chunks.
	PreviousText string `json:"previous_text,omitempty"`

	// NextText provides following context to improve prosody continuity across chunks.
	NextText string `json:"next_text,omitempty"`

	// RequestID is an optional caller-supplied correlation or idempotency identifier.
	RequestID string `json:"request_id,omitempty"`

	// Metadata holds optional provider-agnostic key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks that SpeechRequest contains required fields and valid options.
// Returned errors wrap [ErrInvalidSpeechRequest].
func (r *SpeechRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: request cannot be nil", ErrInvalidSpeechRequest)
	}
	if strings.TrimSpace(r.VoiceID) == "" {
		return fmt.Errorf("%w: voice_id is required", ErrInvalidSpeechRequest)
	}
	if strings.TrimSpace(r.Text) == "" {
		return fmt.Errorf("%w: text is required", ErrInvalidSpeechRequest)
	}
	if !r.Format.IsZero() {
		if err := r.Format.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSpeechRequest, err)
		}
	}
	if err := r.VoiceSettings.Validate(); err != nil {
		return err
	}
	if r.Seed != nil && *r.Seed < 0 {
		return fmt.Errorf("%w: seed must be non-negative, got %d", ErrInvalidSpeechRequest, *r.Seed)
	}
	return nil
}

// String returns a log-safe summary omitting synthesis text content.
func (r SpeechRequest) String() string {
	return fmt.Sprintf("SpeechRequest{model=%q, voice_id=%q, text_len=%d, format=%s, language=%q}",
		r.Model, r.VoiceID, len(r.Text), r.Format.String(), r.Language)
}

// GoString returns a log-safe representation for %#v formatting omitting text content.
func (r SpeechRequest) GoString() string {
	return "core." + r.String()
}

// SpeechUsage tracks usage metrics for a speech synthesis operation.
type SpeechUsage struct {
	// InputCharacters is the number of input characters processed.
	InputCharacters int `json:"input_characters,omitempty"`

	// AudioDuration is the duration of synthesized audio if known.
	AudioDuration time.Duration `json:"audio_duration_ns,omitempty"`

	// OutputBytes is the total size of synthesized audio in bytes.
	OutputBytes int64 `json:"output_bytes,omitempty"`
}

// SpeechResponse contains synthesized audio and response metadata.
type SpeechResponse struct {
	// ID is the provider request or generation identifier, if available.
	ID string `json:"id,omitempty"`

	// Model is the model that generated the speech.
	Model ModelID `json:"model,omitempty"`

	// VoiceID is the voice identifier used for synthesis.
	VoiceID string `json:"voice_id,omitempty"`

	// Audio holds the complete synthesized audio payload.
	Audio AudioData `json:"audio"`

	// Usage holds synthesis consumption metrics.
	Usage SpeechUsage `json:"usage,omitempty"`
}

// String returns a log-safe summary without raw audio bytes.
func (r SpeechResponse) String() string {
	return fmt.Sprintf("SpeechResponse{id=%q, model=%q, voice_id=%q, audio=%s}",
		r.ID, r.Model, r.VoiceID, r.Audio.String())
}

// GoString returns a log-safe representation for %#v formatting without raw audio bytes.
func (r SpeechResponse) GoString() string {
	return "core." + r.String()
}

// SpeechStream represents a streaming text-to-speech response from a provider.
//
// Channel Rules (mirroring [ChatStream] and [ImageStream]):
//   - Providers MUST close Ch, Err, and Final when finished
//   - On context cancellation, providers MUST terminate promptly and close channels
//   - Err emits at most one error
//   - Final emits at most once after stream completion with response metadata/usage
type SpeechStream struct {
	// Ch emits ordered audio chunks. Closed when the stream ends.
	Ch <-chan AudioChunk

	// Err emits at most one error. MUST be closed when the stream ends.
	Err <-chan error

	// Final emits at most one final SpeechResponse containing metadata and usage.
	// Providers may leave Final's Audio.Data empty when chunks were already streamed on Ch.
	Final <-chan *SpeechResponse
}

// DrainSpeechStream consumes all audio chunks from s and returns the assembled
// SpeechResponse. Blocks until the stream completes or ctx is cancelled.
//
// Returns [ErrNilSpeechStream] when s is nil. Follows the same non-lossy
// draining contract as [DrainStream]:
//   - Continues waiting after Ch closes until both Err and Final have resolved
//     so late stream errors are never dropped.
//   - On context cancellation, performs a non-blocking drain of Err (and Final)
//     so a concrete provider error takes precedence over a generic context error.
//   - If Final provides Audio bytes, preserves them; otherwise populates
//     Audio.Data from the accumulated chunks and sets Audio.Format from the
//     first chunk with a non-zero format when Final's format is unset.
func DrainSpeechStream(ctx context.Context, s *SpeechStream) (*SpeechResponse, error) {
	if s == nil {
		return nil, ErrNilSpeechStream
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var buf bytes.Buffer
	var firstFormat AudioFormat
	var streamErr error
	var finalResp *SpeechResponse

	chClosed := s.Ch == nil
	errResolved := s.Err == nil
	finalResolved := s.Final == nil

	for !chClosed || !errResolved || !finalResolved {
		var chRecv <-chan AudioChunk
		if !chClosed {
			chRecv = s.Ch
		}
		var errRecv <-chan error
		if !errResolved {
			errRecv = s.Err
		}
		var finalRecv <-chan *SpeechResponse
		if !finalResolved {
			finalRecv = s.Final
		}

		select {
		case <-ctx.Done():
			if s.Err != nil {
				select {
				case err, ok := <-s.Err:
					if ok && err != nil {
						streamErr = err
					}
				default:
				}
			}
			if s.Final != nil {
				select {
				case <-s.Final:
				default:
				}
			}
			if streamErr != nil {
				return nil, streamErr
			}
			return nil, ctx.Err()

		case chunk, ok := <-chRecv:
			if !ok {
				chClosed = true
				continue
			}
			if firstFormat.IsZero() && !chunk.Format.IsZero() {
				firstFormat = chunk.Format
			}
			if len(chunk.Data) > 0 {
				buf.Write(chunk.Data)
			}

		case err, ok := <-errRecv:
			errResolved = true
			if ok && err != nil {
				streamErr = err
			}

		case resp, ok := <-finalRecv:
			finalResolved = true
			if ok {
				finalResp = resp
			}
		}
	}

	if streamErr != nil {
		return nil, streamErr
	}

	if finalResp == nil {
		data := buf.Bytes()
		return &SpeechResponse{
			Audio: AudioData{
				Data:   data,
				Format: firstFormat,
			},
			Usage: SpeechUsage{
				OutputBytes: int64(len(data)),
			},
		}, nil
	}

	if finalResp.Audio.IsEmpty() && buf.Len() > 0 {
		finalResp.Audio.Data = buf.Bytes()
	}
	if finalResp.Audio.Format.IsZero() && !firstFormat.IsZero() {
		finalResp.Audio.Format = firstFormat
	}
	if finalResp.Usage.OutputBytes == 0 && len(finalResp.Audio.Data) > 0 {
		finalResp.Usage.OutputBytes = int64(len(finalResp.Audio.Data))
	}

	return finalResp, nil
}

// TranscriptionTimestampGranularity controls the level of timestamp alignment
// requested from a speech-to-text provider.
type TranscriptionTimestampGranularity string

const (
	// TimestampGranularityNone uses the provider default.
	TimestampGranularityNone TranscriptionTimestampGranularity = ""
	// TimestampGranularityWord requests word-level timestamps.
	TimestampGranularityWord TranscriptionTimestampGranularity = "word"
	// TimestampGranularitySegment requests segment/utterance-level timestamps.
	TimestampGranularitySegment TranscriptionTimestampGranularity = "segment"
	// TimestampGranularityCharacter requests character-level timestamps.
	TimestampGranularityCharacter TranscriptionTimestampGranularity = "character"
)

// IsValid reports whether the timestamp granularity is a recognized value
// (including the empty default).
func (g TranscriptionTimestampGranularity) IsValid() bool {
	switch g {
	case TimestampGranularityNone,
		TimestampGranularityWord,
		TimestampGranularitySegment,
		TimestampGranularityCharacter:
		return true
	default:
		return false
	}
}

// TranscriptionRequest represents a provider-agnostic speech-to-text request.
//
// Exactly one input source must be provided: either Audio (in-memory payload)
// or Reader (streaming/file reader, omitted from JSON serialization).
type TranscriptionRequest struct {
	// Model specifies the transcription model ID. Empty uses the provider default.
	Model ModelID `json:"model,omitempty"`

	// Audio holds an in-memory audio payload. Mutually exclusive with Reader.
	Audio *AudioData `json:"audio,omitempty"`

	// Reader supplies streaming or file-backed audio bytes without buffering the
	// entire payload in AudioData. Mutually exclusive with Audio.
	Reader io.Reader `json:"-"`

	// Format optionally describes the encoding of Reader input when not inferred
	// from container headers or Filename.
	Format AudioFormat `json:"format,omitempty"`

	// Filename optionally provides a filename hint (e.g., "meeting.wav") for
	// multipart uploads. Overrides Audio.Filename when both are set.
	Filename string `json:"filename,omitempty"`

	// Language is an optional ISO-639-1, ISO-639-3, or BCP-47 language code hint.
	// Empty enables provider auto-detection when supported.
	Language string `json:"language,omitempty"`

	// Prompt supplies optional conditioning context or terminology hints.
	Prompt string `json:"prompt,omitempty"`

	// Keyterms supplies domain-specific words or proper nouns to bias recognition
	// (e.g., ElevenLabs Scribe keyterms).
	Keyterms []string `json:"keyterms,omitempty"`

	// Diarize enables speaker diarization when supported by the provider/model.
	Diarize bool `json:"diarize,omitempty"`

	// NumSpeakers optionally hints at the expected number of speakers (must be > 0).
	NumSpeakers *int `json:"num_speakers,omitempty"`

	// Timestamps requests word, segment, or character timestamp granularity.
	Timestamps TranscriptionTimestampGranularity `json:"timestamps,omitempty"`

	// TagAudioEvents enables tagging of non-speech audio events (e.g., laughter,
	// applause) when supported by the provider/model.
	TagAudioEvents bool `json:"tag_audio_events,omitempty"`

	// Temperature optionally controls decoding randomness (must be finite and >= 0).
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxBytes optionally bounds the accepted audio input size in bytes.
	// Zero disables this limit.
	MaxBytes int64 `json:"max_bytes,omitempty"`

	// RequestID is an optional caller-supplied correlation or idempotency identifier.
	RequestID string `json:"request_id,omitempty"`

	// Metadata holds optional provider-agnostic key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks that TranscriptionRequest provides exactly one valid audio
// source and valid optional parameters. Returned errors wrap
// [ErrInvalidTranscriptionRequest].
func (r *TranscriptionRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: request cannot be nil", ErrInvalidTranscriptionRequest)
	}
	hasAudio := r.Audio != nil
	hasReader := r.Reader != nil
	if !hasAudio && !hasReader {
		return fmt.Errorf("%w: either audio or reader must be provided", ErrInvalidTranscriptionRequest)
	}
	if hasAudio && hasReader {
		return fmt.Errorf("%w: audio and reader are mutually exclusive", ErrInvalidTranscriptionRequest)
	}
	if r.MaxBytes < 0 {
		return fmt.Errorf("%w: max_bytes must be non-negative, got %d", ErrInvalidTranscriptionRequest, r.MaxBytes)
	}
	if hasAudio {
		if err := r.Audio.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTranscriptionRequest, err)
		}
		if r.MaxBytes > 0 {
			raw, err := r.Audio.GetBytes()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidTranscriptionRequest, err)
			}
			if int64(len(raw)) > r.MaxBytes {
				return fmt.Errorf("%w: audio size %d exceeds max_bytes %d", ErrInvalidTranscriptionRequest, len(raw), r.MaxBytes)
			}
		}
	}
	if !r.Format.IsZero() {
		if err := r.Format.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTranscriptionRequest, err)
		}
	}
	if !r.Timestamps.IsValid() {
		return fmt.Errorf("%w: unsupported timestamps granularity %q", ErrInvalidTranscriptionRequest, r.Timestamps)
	}
	if r.NumSpeakers != nil && *r.NumSpeakers <= 0 {
		return fmt.Errorf("%w: num_speakers must be > 0, got %d", ErrInvalidTranscriptionRequest, *r.NumSpeakers)
	}
	if r.Temperature != nil {
		if math.IsNaN(*r.Temperature) || math.IsInf(*r.Temperature, 0) || *r.Temperature < 0 {
			return fmt.Errorf("%w: temperature must be a finite number >= 0, got %v", ErrInvalidTranscriptionRequest, *r.Temperature)
		}
	}
	for i, term := range r.Keyterms {
		if strings.TrimSpace(term) == "" {
			return fmt.Errorf("%w: keyterms[%d] must not be empty", ErrInvalidTranscriptionRequest, i)
		}
	}
	return nil
}

// String returns a log-safe metadata summary without audio bytes or prompt text.
func (r TranscriptionRequest) String() string {
	source := "none"
	if r.Audio != nil {
		source = r.Audio.String()
	} else if r.Reader != nil {
		source = fmt.Sprintf("reader(format=%s, filename=%q)", r.Format.String(), r.Filename)
	}
	return fmt.Sprintf("TranscriptionRequest{model=%q, source=%s, language=%q, diarize=%t, timestamps=%q}",
		r.Model, source, r.Language, r.Diarize, r.Timestamps)
}

// GoString returns a log-safe representation for %#v formatting without audio bytes.
func (r TranscriptionRequest) GoString() string {
	return "core." + r.String()
}

// TranscriptionWord represents a single timed word, spacing token, or inline
// event in a transcription response.
type TranscriptionWord struct {
	// Text is the transcribed token or word text.
	Text string `json:"text"`

	// Start is the token start offset from the beginning of the audio.
	Start time.Duration `json:"start_ns"`

	// End is the token end offset from the beginning of the audio.
	End time.Duration `json:"end_ns"`

	// Type optionally categorizes the token (e.g., "word", "spacing", "audio_event").
	Type string `json:"type,omitempty"`

	// SpeakerID identifies the active speaker when diarization is enabled.
	SpeakerID string `json:"speaker_id,omitempty"`

	// Confidence is an optional normalized confidence score in [0.0, 1.0].
	Confidence *float64 `json:"confidence,omitempty"`

	// LogProb is an optional token log-probability reported by the provider.
	LogProb *float64 `json:"logprob,omitempty"`
}

// Validate checks that word timestamps are non-negative, ordered, and have
// finite optional score fields. Returned errors wrap [ErrInvalidTranscriptionResponse].
func (w TranscriptionWord) Validate() error {
	if w.Start < 0 {
		return fmt.Errorf("%w: word start must be non-negative, got %s", ErrInvalidTranscriptionResponse, w.Start)
	}
	if w.End < w.Start {
		return fmt.Errorf("%w: word end %s precedes start %s", ErrInvalidTranscriptionResponse, w.End, w.Start)
	}
	if err := validateUnitInterval("word.confidence", w.Confidence, ErrInvalidTranscriptionResponse); err != nil {
		return err
	}
	if w.LogProb != nil && (math.IsNaN(*w.LogProb) || math.IsInf(*w.LogProb, 0)) {
		return fmt.Errorf("%w: word.logprob must be finite", ErrInvalidTranscriptionResponse)
	}
	return nil
}

// TranscriptionSegment represents a timed utterance or sentence-level segment
// in a transcription response.
type TranscriptionSegment struct {
	// ID is the optional zero-based segment index.
	ID int `json:"id,omitempty"`

	// Text is the transcribed text for this segment.
	Text string `json:"text"`

	// Start is the segment start offset from the beginning of the audio.
	Start time.Duration `json:"start_ns"`

	// End is the segment end offset from the beginning of the audio.
	End time.Duration `json:"end_ns"`

	// SpeakerID identifies the active speaker when diarization is enabled.
	SpeakerID string `json:"speaker_id,omitempty"`

	// Words optionally contains word-level alignment within this segment.
	Words []TranscriptionWord `json:"words,omitempty"`

	// Confidence is an optional normalized confidence score in [0.0, 1.0].
	Confidence *float64 `json:"confidence,omitempty"`
}

// Validate checks that segment timestamps and any nested words are valid.
// Returned errors wrap [ErrInvalidTranscriptionResponse].
func (s TranscriptionSegment) Validate() error {
	if s.Start < 0 {
		return fmt.Errorf("%w: segment start must be non-negative, got %s", ErrInvalidTranscriptionResponse, s.Start)
	}
	if s.End < s.Start {
		return fmt.Errorf("%w: segment end %s precedes start %s", ErrInvalidTranscriptionResponse, s.End, s.Start)
	}
	if err := validateUnitInterval("segment.confidence", s.Confidence, ErrInvalidTranscriptionResponse); err != nil {
		return err
	}
	for i, w := range s.Words {
		if err := w.Validate(); err != nil {
			return fmt.Errorf("segment word[%d]: %w", i, err)
		}
	}
	return nil
}

// AudioEvent represents a detected non-speech event in a transcription response
// (such as laughter, applause, or background music).
type AudioEvent struct {
	// Type is the event label (e.g., "laughter", "applause").
	Type string `json:"type"`

	// Start is the event start offset from the beginning of the audio.
	Start time.Duration `json:"start_ns,omitempty"`

	// End is the event end offset from the beginning of the audio.
	End time.Duration `json:"end_ns,omitempty"`

	// SpeakerID optionally identifies the associated speaker.
	SpeakerID string `json:"speaker_id,omitempty"`

	// Confidence is an optional normalized confidence score in [0.0, 1.0].
	Confidence *float64 `json:"confidence,omitempty"`
}

// Validate checks that AudioEvent carries a non-empty type and valid timing
// and confidence fields. Returned errors wrap [ErrInvalidTranscriptionResponse].
func (e AudioEvent) Validate() error {
	if strings.TrimSpace(e.Type) == "" {
		return fmt.Errorf("%w: audio event type is required", ErrInvalidTranscriptionResponse)
	}
	if e.Start < 0 {
		return fmt.Errorf("%w: audio event start must be non-negative, got %s", ErrInvalidTranscriptionResponse, e.Start)
	}
	if e.End < 0 {
		return fmt.Errorf("%w: audio event end must be non-negative, got %s", ErrInvalidTranscriptionResponse, e.End)
	}
	if e.End != 0 && e.End < e.Start {
		return fmt.Errorf("%w: audio event end %s precedes start %s", ErrInvalidTranscriptionResponse, e.End, e.Start)
	}
	if err := validateUnitInterval("audio_event.confidence", e.Confidence, ErrInvalidTranscriptionResponse); err != nil {
		return err
	}
	return nil
}

// TranscriptionUsage tracks usage metrics for a transcription operation.
type TranscriptionUsage struct {
	// AudioDuration is the duration of transcribed audio if known.
	AudioDuration time.Duration `json:"audio_duration_ns,omitempty"`

	// InputBytes is the total size of input audio bytes processed.
	InputBytes int64 `json:"input_bytes,omitempty"`
}

// TranscriptionResponse contains the result of a speech-to-text transcription.
//
// IsFinal distinguishes committed utterances (safe for downstream persistence
// such as Cortex memory ingestion) from interim partial transcripts.
//
// Privacy: String and GoString omit raw transcript text and report text length
// and metadata counts instead so default logging never exposes transcripts.
type TranscriptionResponse struct {
	// ID is the provider request or transcription identifier, if available.
	ID string `json:"id,omitempty"`

	// Model is the model that performed the transcription.
	Model ModelID `json:"model,omitempty"`

	// Text is the full concatenated transcript.
	Text string `json:"text"`

	// Language is the detected or enforced ISO-639 / BCP-47 language code.
	Language string `json:"language,omitempty"`

	// LanguageProbability is the provider's confidence in Language in [0.0, 1.0].
	LanguageProbability *float64 `json:"language_probability,omitempty"`

	// Duration is the total audio duration if reported by the provider.
	Duration time.Duration `json:"duration_ns,omitempty"`

	// Words contains word/token-level timestamps and speaker tags when available.
	Words []TranscriptionWord `json:"words,omitempty"`

	// Segments contains utterance/segment-level timestamps when available.
	Segments []TranscriptionSegment `json:"segments,omitempty"`

	// AudioEvents contains detected non-speech audio events when requested/supported.
	AudioEvents []AudioEvent `json:"audio_events,omitempty"`

	// IsFinal indicates a committed, final transcript as opposed to a partial hypothesis.
	IsFinal bool `json:"is_final,omitempty"`

	// Usage holds transcription consumption metrics.
	Usage TranscriptionUsage `json:"usage,omitempty"`
}

// Validate checks that response metadata, timestamps, segments, and audio events
// are internally consistent. Returned errors wrap [ErrInvalidTranscriptionResponse].
func (r *TranscriptionResponse) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: response cannot be nil", ErrInvalidTranscriptionResponse)
	}
	if r.Duration < 0 {
		return fmt.Errorf("%w: duration must be non-negative, got %s", ErrInvalidTranscriptionResponse, r.Duration)
	}
	if err := validateUnitInterval("language_probability", r.LanguageProbability, ErrInvalidTranscriptionResponse); err != nil {
		return err
	}
	if r.Usage.AudioDuration < 0 {
		return fmt.Errorf("%w: usage.audio_duration must be non-negative, got %s", ErrInvalidTranscriptionResponse, r.Usage.AudioDuration)
	}
	if r.Usage.InputBytes < 0 {
		return fmt.Errorf("%w: usage.input_bytes must be non-negative, got %d", ErrInvalidTranscriptionResponse, r.Usage.InputBytes)
	}
	for i, w := range r.Words {
		if err := w.Validate(); err != nil {
			return fmt.Errorf("words[%d]: %w", i, err)
		}
	}
	for i, s := range r.Segments {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("segments[%d]: %w", i, err)
		}
	}
	for i, e := range r.AudioEvents {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("audio_events[%d]: %w", i, err)
		}
	}
	return nil
}

// String returns a log-safe metadata summary without raw transcript text.
func (r TranscriptionResponse) String() string {
	return fmt.Sprintf("TranscriptionResponse{id=%q, model=%q, language=%q, text_len=%d, words=%d, segments=%d, final=%t}",
		r.ID, r.Model, r.Language, len(r.Text), len(r.Words), len(r.Segments), r.IsFinal)
}

// GoString returns a log-safe representation for %#v formatting without transcript text.
func (r TranscriptionResponse) GoString() string {
	return "core." + r.String()
}

func validateUnitInterval(field string, v *float64, sentinel error) error {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 1 {
		return fmt.Errorf("%w: %s must be in [0, 1], got %v", sentinel, field, *v)
	}
	return nil
}
