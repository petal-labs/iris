package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/petal-labs/iris/v2/core"
)

// DefaultTTSModel is the default ElevenLabs text-to-speech model used when
// SpeechRequest.Model is empty.
const DefaultTTSModel = ModelElevenMultilingualV2

// maxTTSResponseBytes caps synchronous TTS audio reads to prevent unbounded
// memory growth on malformed responses.
const maxTTSResponseBytes = 100 * 1024 * 1024 // 100 MiB

type ttsRequestBody struct {
	Text          string            `json:"text"`
	ModelID       core.ModelID      `json:"model_id,omitempty"`
	LanguageCode  string            `json:"language_code,omitempty"`
	VoiceSettings *ttsVoiceSettings `json:"voice_settings,omitempty"`
	Seed          *int64            `json:"seed,omitempty"`
	PreviousText  string            `json:"previous_text,omitempty"`
	NextText      string            `json:"next_text,omitempty"`
}

type ttsVoiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	UseSpeakerBoost *bool    `json:"use_speaker_boost,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
}

func buildTTSRequestBody(req *core.SpeechRequest, model core.ModelID) ttsRequestBody {
	body := ttsRequestBody{
		Text:         req.Text,
		ModelID:      model,
		LanguageCode: req.Language,
		Seed:         req.Seed,
		PreviousText: req.PreviousText,
		NextText:     req.NextText,
	}
	if req.VoiceSettings != nil {
		body.VoiceSettings = &ttsVoiceSettings{
			Stability:       req.VoiceSettings.Stability,
			SimilarityBoost: req.VoiceSettings.SimilarityBoost,
			Style:           req.VoiceSettings.Style,
			UseSpeakerBoost: req.VoiceSettings.UseSpeakerBoost,
			Speed:           req.VoiceSettings.Speed,
		}
	}
	return body
}

// Synthesize converts text to audio using ElevenLabs' synchronous
// POST /v1/text-to-speech/{voice_id} endpoint.
//
// If req.Model is empty, [DefaultTTSModel] is used. If req.Format is zero,
// [DefaultTTSFormat] ("mp3_44100_128") is requested.
func (p *ElevenLabs) Synthesize(ctx context.Context, req *core.SpeechRequest) (*core.SpeechResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	outputFormat, resolvedFormat, err := resolveOutputFormat(req.Format)
	if err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = DefaultTTSModel
	}

	payload, err := json.Marshal(buildTTSRequestBody(req, model))
	if err != nil {
		return nil, newDecodeError(fmt.Errorf("marshal tts request: %w", err), nil)
	}

	path := fmt.Sprintf("/v1/text-to-speech/%s?output_format=%s",
		url.PathEscape(req.VoiceID),
		url.QueryEscape(outputFormat),
	)

	resp, err := p.doRequest(ctx, http.MethodPost, path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxTTSResponseBytes+1))
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, newNetworkError(err)
	}
	if int64(len(audioBytes)) > maxTTSResponseBytes {
		return nil, newDecodeError(fmt.Errorf("tts audio response exceeds maximum size of %d bytes", maxTTSResponseBytes), nil)
	}
	if len(audioBytes) == 0 {
		return nil, newDecodeError(errors.New("empty audio response from ElevenLabs"), nil)
	}

	resolvedFormat.MIMEType = responseMIMEType(resp.Header.Get("Content-Type"), resolvedFormat.EffectiveMIMEType())

	usage := core.SpeechUsage{
		OutputBytes: int64(len(audioBytes)),
	}
	if cost := parseCharacterCost(resp.Header); cost > 0 {
		usage.InputCharacters = cost
	}

	return &core.SpeechResponse{
		ID:      extractRequestID(resp.Header),
		Model:   model,
		VoiceID: req.VoiceID,
		Audio: core.AudioData{
			Data:   audioBytes,
			Format: resolvedFormat,
		},
		Usage: usage,
	}, nil
}

func parseCharacterCost(headers http.Header) int {
	raw := strings.TrimSpace(headers.Get("character-cost"))
	if raw == "" {
		return 0
	}
	cost, err := strconv.Atoi(raw)
	if err != nil || cost < 0 {
		return 0
	}
	return cost
}

// Compile-time check that ElevenLabs implements core.SpeechSynthesizer.
var _ core.SpeechSynthesizer = (*ElevenLabs)(nil)
