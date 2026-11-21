package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/petal-labs/iris/v2/core"
)

// DefaultSTTModel is the default ElevenLabs Scribe model used when
// TranscriptionRequest.Model is empty.
const DefaultSTTModel = ModelScribeV2

// ElevenLabs Scribe request validation limits.
const (
	// DefaultMaxTranscriptionBytes caps uploaded audio size when
	// TranscriptionRequest.MaxBytes is zero (100 MiB).
	DefaultMaxTranscriptionBytes int64 = 100 * 1024 * 1024

	// MaxScribeKeyterms is the maximum number of keyterms accepted by ElevenLabs Scribe.
	MaxScribeKeyterms = 1000

	// MaxScribeSpeakers is the maximum num_speakers hint accepted by ElevenLabs Scribe.
	MaxScribeSpeakers = 32

	// MaxScribeTemperature is the maximum decoding temperature accepted by ElevenLabs Scribe.
	MaxScribeTemperature = 2.0

	maxSTTResponseBytes = 32 * 1024 * 1024 // 32 MiB JSON transcript cap
)

type scribeResponse struct {
	TranscriptionID     string             `json:"transcription_id"`
	LanguageCode        string             `json:"language_code"`
	LanguageProbability *float64           `json:"language_probability,omitempty"`
	Text                string             `json:"text"`
	Words               []scribeWord       `json:"words,omitempty"`
	AudioEvents         []scribeAudioEvent `json:"audio_events,omitempty"`
}

type scribeWord struct {
	Text      string   `json:"text"`
	Start     float64  `json:"start"`
	End       float64  `json:"end"`
	Type      string   `json:"type"`
	SpeakerID string   `json:"speaker_id"`
	LogProb   *float64 `json:"logprob,omitempty"`
}

type scribeAudioEvent struct {
	Type       string   `json:"type"`
	Start      float64  `json:"start"`
	End        float64  `json:"end"`
	SpeakerID  string   `json:"speaker_id"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// Transcribe converts buffered or reader-backed audio into a structured
// transcript using ElevenLabs' POST /v1/speech-to-text endpoint.
//
// If req.Model is empty, [DefaultSTTModel] ("scribe_v2") is used. Returned
// responses always set IsFinal = true to distinguish committed file transcripts
// from real-time interim hypotheses.
//
// Limits enforced before sending HTTP traffic:
//   - Audio size: req.MaxBytes if set, otherwise [DefaultMaxTranscriptionBytes] (100 MiB)
//   - Keyterms: at most [MaxScribeKeyterms] (1,000); not supported on scribe_v1
//   - NumSpeakers: at most [MaxScribeSpeakers] (32)
//   - Temperature: at most [MaxScribeTemperature] (2.0)
//   - Timestamps: none, word, or character (segment granularity is not supported by Scribe)
//   - Raw PCM input: 16000Hz mono signed 16-bit little-endian (pcm_s16le_16)
func (p *ElevenLabs) Transcribe(ctx context.Context, req *core.TranscriptionRequest) (*core.TranscriptionResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = DefaultSTTModel
	}

	fileFormat, err := validateScribeRequest(req, model)
	if err != nil {
		return nil, err
	}

	if err := p.requireAPIKey(); err != nil {
		return nil, err
	}

	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxTranscriptionBytes
	}

	filename := resolveTranscriptionFilename(req)
	var src io.Reader
	if req.Audio != nil {
		raw, err := req.Audio.GetBytes()
		if err != nil {
			return nil, fmt.Errorf("%w: %w", core.ErrInvalidTranscriptionRequest, err)
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("%w: audio payload is empty", core.ErrInvalidTranscriptionRequest)
		}
		if int64(len(raw)) > maxBytes {
			return nil, fmt.Errorf("%w: audio size %d exceeds limit %d", core.ErrInvalidTranscriptionRequest, len(raw), maxBytes)
		}
		src = bytes.NewReader(raw)
	} else {
		// Peek up to 1 byte so empty readers fail fast before opening an HTTP request.
		var first [1]byte
		n, readErr := req.Reader.Read(first[:])
		if n == 0 {
			if readErr == nil || errors.Is(readErr, io.EOF) {
				return nil, fmt.Errorf("%w: reader audio input is empty", core.ErrInvalidTranscriptionRequest)
			}
			if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
				return nil, readErr
			}
			return nil, fmt.Errorf("%w: read audio input: %w", core.ErrInvalidTranscriptionRequest, readErr)
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
				return nil, readErr
			}
			return nil, fmt.Errorf("%w: read audio input: %w", core.ErrInvalidTranscriptionRequest, readErr)
		}
		if errors.Is(readErr, io.EOF) {
			src = bytes.NewReader(first[:n])
		} else {
			src = io.MultiReader(bytes.NewReader(first[:n]), req.Reader)
		}
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	var (
		inputBytes atomic.Int64
		writeMu    sync.Mutex
		writeErr   error
		writerDone = make(chan struct{})
	)
	setWriteErr := func(e error) {
		writeMu.Lock()
		if writeErr == nil {
			writeErr = e
		}
		writeMu.Unlock()
	}
	getWriteErr := func() error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeErr
	}

	go func() {
		defer close(writerDone)
		err := writeScribeMultipart(mw, req, model, fileFormat, filename, src, maxBytes, &inputBytes)
		if err != nil {
			setWriteErr(err)
			_ = pw.CloseWithError(err)
			return
		}
		if closeErr := mw.Close(); closeErr != nil {
			setWriteErr(closeErr)
			_ = pw.CloseWithError(closeErr)
			return
		}
		_ = pw.Close()
	}()

	resp, err := p.doRequest(ctx, http.MethodPost, "/v1/speech-to-text", contentType, pr)
	_ = pr.Close()
	<-writerDone

	if err != nil {
		if wErr := getWriteErr(); wErr != nil {
			if errors.Is(wErr, core.ErrInvalidTranscriptionRequest) {
				return nil, wErr
			}
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxSTTResponseBytes+1))
	if readErr != nil {
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return nil, readErr
		}
		return nil, newNetworkError(readErr)
	}
	if int64(len(respBytes)) > maxSTTResponseBytes {
		return nil, newDecodeError(fmt.Errorf("transcription response exceeds maximum size of %d bytes", maxSTTResponseBytes), nil)
	}

	var parsed scribeResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, newDecodeError(err, respBytes)
	}

	return mapScribeResponse(&parsed, model, extractRequestID(resp.Header), inputBytes.Load()), nil
}

func validateScribeRequest(req *core.TranscriptionRequest, model core.ModelID) (string, error) {
	if len(req.Keyterms) > MaxScribeKeyterms {
		return "", fmt.Errorf("%w: keyterms count %d exceeds ElevenLabs limit of %d",
			core.ErrInvalidTranscriptionRequest, len(req.Keyterms), MaxScribeKeyterms)
	}
	if len(req.Keyterms) > 0 && model == ModelScribeV1 {
		return "", fmt.Errorf("%w: keyterms are not supported with model %q (use %q)",
			core.ErrInvalidTranscriptionRequest, ModelScribeV1, ModelScribeV2)
	}
	if req.NumSpeakers != nil && *req.NumSpeakers > MaxScribeSpeakers {
		return "", fmt.Errorf("%w: num_speakers %d exceeds ElevenLabs limit of %d",
			core.ErrInvalidTranscriptionRequest, *req.NumSpeakers, MaxScribeSpeakers)
	}
	if req.Temperature != nil && *req.Temperature > MaxScribeTemperature {
		return "", fmt.Errorf("%w: temperature %v exceeds ElevenLabs limit of %v",
			core.ErrInvalidTranscriptionRequest, *req.Temperature, MaxScribeTemperature)
	}
	if req.Timestamps == core.TimestampGranularitySegment {
		return "", fmt.Errorf("%w: elevenlabs scribe supports none, word, or character timestamps, not %q",
			core.ErrInvalidTranscriptionRequest, req.Timestamps)
	}

	var fmtHint core.AudioFormat
	if req.Audio != nil {
		fmtHint = req.Audio.Format
	} else {
		fmtHint = req.Format
	}
	if fmtHint.Encoding == core.AudioEncodingPCM {
		if fmtHint.SampleRate != 0 && fmtHint.SampleRate != 16000 {
			return "", fmt.Errorf("%w: %w: elevenlabs scribe raw PCM requires 16000Hz sample rate, got %dHz",
				core.ErrInvalidTranscriptionRequest, core.ErrInvalidAudioFormat, fmtHint.SampleRate)
		}
		if fmtHint.Channels > 1 {
			return "", fmt.Errorf("%w: %w: elevenlabs scribe raw PCM requires mono (1 channel), got %d",
				core.ErrInvalidTranscriptionRequest, core.ErrInvalidAudioFormat, fmtHint.Channels)
		}
		return "pcm_s16le_16", nil
	}

	return "", nil
}

func resolveTranscriptionFilename(req *core.TranscriptionRequest) string {
	if req.Filename != "" {
		return req.Filename
	}
	if req.Audio != nil && req.Audio.Filename != "" {
		return req.Audio.Filename
	}
	var enc core.AudioEncoding
	if req.Audio != nil {
		enc = req.Audio.Format.Encoding
	} else {
		enc = req.Format.Encoding
	}
	if ext := enc.FileExtension(); ext != "" {
		return "audio." + ext
	}
	return "audio.bin"
}

func writeScribeMultipart(
	mw *multipart.Writer,
	req *core.TranscriptionRequest,
	model core.ModelID,
	fileFormat string,
	filename string,
	src io.Reader,
	maxBytes int64,
	written *atomic.Int64,
) error {
	if err := mw.WriteField("model_id", string(model)); err != nil {
		return err
	}
	if req.Language != "" {
		if err := mw.WriteField("language_code", req.Language); err != nil {
			return err
		}
	}
	if req.Diarize {
		if err := mw.WriteField("diarize", "true"); err != nil {
			return err
		}
	}
	if req.NumSpeakers != nil {
		if err := mw.WriteField("num_speakers", strconv.Itoa(*req.NumSpeakers)); err != nil {
			return err
		}
	}
	if req.Timestamps != "" {
		if err := mw.WriteField("timestamps_granularity", string(req.Timestamps)); err != nil {
			return err
		}
	}
	if req.TagAudioEvents {
		if err := mw.WriteField("tag_audio_events", "true"); err != nil {
			return err
		}
	}
	if req.Temperature != nil {
		if err := mw.WriteField("temperature", strconv.FormatFloat(*req.Temperature, 'f', -1, 64)); err != nil {
			return err
		}
	}
	if fileFormat != "" {
		if err := mw.WriteField("file_format", fileFormat); err != nil {
			return err
		}
	}
	for _, term := range req.Keyterms {
		if err := mw.WriteField("keyterms", term); err != nil {
			return err
		}
	}

	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return err
	}

	n, copyErr := io.Copy(part, io.LimitReader(src, maxBytes+1))
	written.Store(n)
	if copyErr != nil {
		return copyErr
	}
	if n > maxBytes {
		return fmt.Errorf("%w: audio size exceeds limit %d bytes", core.ErrInvalidTranscriptionRequest, maxBytes)
	}
	return nil
}

func mapScribeResponse(src *scribeResponse, model core.ModelID, headerReqID string, inputBytes int64) *core.TranscriptionResponse {
	id := src.TranscriptionID
	if id == "" {
		id = headerReqID
	}

	var maxEnd time.Duration

	var words []core.TranscriptionWord
	if len(src.Words) > 0 {
		words = make([]core.TranscriptionWord, len(src.Words))
		for i, w := range src.Words {
			start := secondsToDuration(w.Start)
			end := secondsToDuration(w.End)
			if end > maxEnd {
				maxEnd = end
			}
			words[i] = core.TranscriptionWord{
				Text:      w.Text,
				Start:     start,
				End:       end,
				Type:      w.Type,
				SpeakerID: w.SpeakerID,
				LogProb:   w.LogProb,
			}
		}
	}

	var events []core.AudioEvent
	if len(src.AudioEvents) > 0 {
		events = make([]core.AudioEvent, len(src.AudioEvents))
		for i, e := range src.AudioEvents {
			start := secondsToDuration(e.Start)
			end := secondsToDuration(e.End)
			if end > maxEnd {
				maxEnd = end
			}
			events[i] = core.AudioEvent{
				Type:       e.Type,
				Start:      start,
				End:        end,
				SpeakerID:  e.SpeakerID,
				Confidence: e.Confidence,
			}
		}
	}

	return &core.TranscriptionResponse{
		ID:                  id,
		Model:               model,
		Text:                src.Text,
		Language:            src.LanguageCode,
		LanguageProbability: src.LanguageProbability,
		Duration:            maxEnd,
		Words:               words,
		AudioEvents:         events,
		IsFinal:             true,
		Usage: core.TranscriptionUsage{
			AudioDuration: maxEnd,
			InputBytes:    inputBytes,
		},
	}
}

func secondsToDuration(sec float64) time.Duration {
	if math.IsNaN(sec) || math.IsInf(sec, 0) || sec <= 0 {
		return 0
	}
	return time.Duration(math.Round(sec * float64(time.Second)))
}

// Compile-time check that ElevenLabs implements core.SpeechRecognizer.
var _ core.SpeechRecognizer = (*ElevenLabs)(nil)
