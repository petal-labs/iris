package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/petal-labs/iris/core"
)

// DefaultRealtimeTTSModel is the default ElevenLabs model used for WebSocket
// incremental-input TTS when RealtimeSynthesisConfig.Model is empty.
const DefaultRealtimeTTSModel = ModelElevenFlashV25

// Bounded defaults and API limits for ElevenLabs incremental-input TTS WebSockets.
const (
	// DefaultRealtimeTTSBufferSize is the default bounded capacity of Audio().
	DefaultRealtimeTTSBufferSize = 32

	minRealtimeChunkLength       = 50
	maxRealtimeChunkLength       = 500
	minRealtimeInactivityTimeout = 1 * time.Second
	maxRealtimeInactivityTimeout = 180 * time.Second
)

type wsTTSBOSMessage struct {
	Text             string               `json:"text"`
	VoiceSettings    *ttsVoiceSettings    `json:"voice_settings,omitempty"`
	GenerationConfig *wsTTSGenerationConf `json:"generation_config,omitempty"`
}

type wsTTSGenerationConf struct {
	ChunkLengthSchedule []int `json:"chunk_length_schedule,omitempty"`
}

type wsTTSTextMessage struct {
	Text                 string `json:"text"`
	TryTriggerGeneration bool   `json:"try_trigger_generation,omitempty"`
	Flush                bool   `json:"flush,omitempty"`
}

type wsTTSServerMessage struct {
	Audio        *string         `json:"audio"`
	IsFinal      bool            `json:"isFinal"`
	IsFinalSnake bool            `json:"is_final"`
	Error        string          `json:"error,omitempty"`
	Message      string          `json:"message,omitempty"`
	Code         json.RawMessage `json:"code,omitempty"`
}

// StartRealtimeSynthesis opens a bidirectional incremental-input TTS WebSocket
// session to ElevenLabs' wss://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream-input
// endpoint.
//
// Model compatibility is enforced before dialing: models in the ElevenLabs
// catalog that lack [core.FeatureSpeechSynthesisRealtime] (such as
// [ModelElevenV3] and Scribe STT models) as well as v3/v4/scribe model IDs are
// rejected with a [*core.ProviderError] wrapping [core.ErrNotSupported].
//
// When cfg.Model is empty, [DefaultRealtimeTTSModel] ([ModelElevenFlashV25]) is
// used. When cfg.Format is zero, [DefaultTTSFormat] ("mp3_44100_128") is used.
// Callers should always defer Close() on the returned session to guarantee
// WebSocket and goroutine cleanup.
func (p *ElevenLabs) StartRealtimeSynthesis(ctx context.Context, cfg core.RealtimeSynthesisConfig) (core.RealtimeSynthesisSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("elevenlabs: realtime synthesis context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	model := cfg.Model
	if model == "" {
		model = DefaultRealtimeTTSModel
	}
	if err := validateRealtimeTTSModel(model); err != nil {
		return nil, err
	}

	for i, n := range cfg.ChunkLengthSchedule {
		if n < minRealtimeChunkLength || n > maxRealtimeChunkLength {
			return nil, fmt.Errorf("%w: elevenlabs chunk_length_schedule[%d] must be between %d and %d, got %d",
				core.ErrInvalidSpeechRequest, i, minRealtimeChunkLength, maxRealtimeChunkLength, n)
		}
	}
	if cfg.InactivityTimeout > 0 {
		if cfg.InactivityTimeout < minRealtimeInactivityTimeout || cfg.InactivityTimeout > maxRealtimeInactivityTimeout {
			return nil, fmt.Errorf("%w: elevenlabs inactivity_timeout must be between %s and %s, got %s",
				core.ErrInvalidSpeechRequest, minRealtimeInactivityTimeout, maxRealtimeInactivityTimeout, cfg.InactivityTimeout)
		}
	}

	outputFormat, resolvedFormat, err := resolveOutputFormat(cfg.Format)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrInvalidSpeechRequest, err)
	}

	q := url.Values{}
	q.Set("model_id", string(model))
	q.Set("output_format", outputFormat)
	if cfg.Language != "" {
		q.Set("language_code", cfg.Language)
	}
	if cfg.InactivityTimeout > 0 {
		q.Set("inactivity_timeout", strconv.Itoa(int(cfg.InactivityTimeout/time.Second)))
	}
	if cfg.SyncAlignment {
		q.Set("sync_alignment", "true")
	}
	if cfg.AutoMode {
		q.Set("auto_mode", "true")
	}

	path := fmt.Sprintf("/v1/text-to-speech/%s/stream-input?%s",
		url.PathEscape(cfg.VoiceID),
		q.Encode(),
	)

	conn, err := p.dialWebSocket(ctx, path)
	if err != nil {
		return nil, err
	}

	bos := buildWSTTSBOSMessage(cfg)
	bosPayload, err := json.Marshal(bos)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "marshal bos failed")
		return nil, newDecodeError(fmt.Errorf("marshal realtime tts bos: %w", err), nil)
	}
	if err := conn.WriteText(ctx, bosPayload); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "bos write failed")
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, redactSecretError(newNetworkError(err), p.config.APIKey)
	}

	bufCap := cfg.BufferSize
	if bufCap <= 0 {
		bufCap = DefaultRealtimeTTSBufferSize
	}

	sessCtx, cancel := context.WithCancel(ctx)
	s := &realtimeTTSSession{
		provider: p,
		model:    model,
		voiceID:  cfg.VoiceID,
		format:   resolvedFormat,
		conn:     conn,
		ctx:      sessCtx,
		cancel:   cancel,
		audio:    make(chan core.AudioChunk, bufCap),
		errCh:    make(chan error, 1),
		done:     make(chan struct{}),
	}

	go s.readLoop()
	return s, nil
}

func validateRealtimeTTSModel(model core.ModelID) error {
	if info := GetModelInfo(model); info != nil {
		if !info.HasCapability(core.FeatureSpeechSynthesisRealtime) {
			return unsupportedRealtimeTTSModelError(model)
		}
		return nil
	}

	lower := strings.ToLower(strings.TrimSpace(string(model)))
	if strings.HasPrefix(lower, "eleven_v3") ||
		strings.HasPrefix(lower, "eleven_v4") ||
		strings.HasPrefix(lower, "scribe") {
		return unsupportedRealtimeTTSModelError(model)
	}
	return nil
}

func unsupportedRealtimeTTSModelError(model core.ModelID) error {
	return &core.ProviderError{
		Provider: providerID,
		Status:   http.StatusBadRequest,
		Code:     "unsupported_model",
		Message: fmt.Sprintf(
			"model %q does not support ElevenLabs incremental-input TTS WebSocket; use %s or %s",
			model, ModelElevenFlashV25, ModelElevenMultilingualV2,
		),
		Err: core.ErrNotSupported,
	}
}

func buildWSTTSBOSMessage(cfg core.RealtimeSynthesisConfig) wsTTSBOSMessage {
	bos := wsTTSBOSMessage{
		Text: " ",
	}
	if cfg.VoiceSettings != nil {
		bos.VoiceSettings = &ttsVoiceSettings{
			Stability:       cfg.VoiceSettings.Stability,
			SimilarityBoost: cfg.VoiceSettings.SimilarityBoost,
			Style:           cfg.VoiceSettings.Style,
			UseSpeakerBoost: cfg.VoiceSettings.UseSpeakerBoost,
			Speed:           cfg.VoiceSettings.Speed,
		}
	}
	if len(cfg.ChunkLengthSchedule) > 0 {
		sched := make([]int, len(cfg.ChunkLengthSchedule))
		copy(sched, cfg.ChunkLengthSchedule)
		bos.GenerationConfig = &wsTTSGenerationConf{
			ChunkLengthSchedule: sched,
		}
	}
	return bos
}

type realtimeTTSSession struct {
	provider *ElevenLabs
	model    core.ModelID
	voiceID  string
	format   core.AudioFormat

	conn    wsConn
	writeMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	audio chan core.AudioChunk
	errCh chan error
	done  chan struct{}

	finished  atomic.Bool
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func (s *realtimeTTSSession) SendText(ctx context.Context, text string) error {
	if s.closed.Load() || s.finished.Load() {
		return core.ErrRealtimeSessionClosed
	}
	if text == "" {
		return fmt.Errorf("%w: text fragment cannot be empty (use Finish to end the stream)", core.ErrInvalidSpeechRequest)
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.writeMessage(ctx, wsTTSTextMessage{Text: text}, false)
}

func (s *realtimeTTSSession) Flush(ctx context.Context) error {
	if s.closed.Load() || s.finished.Load() {
		return core.ErrRealtimeSessionClosed
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.writeMessage(ctx, wsTTSTextMessage{Text: " ", Flush: true}, false)
}

func (s *realtimeTTSSession) Finish(ctx context.Context) error {
	if s.closed.Load() || s.finished.Load() {
		return core.ErrRealtimeSessionClosed
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.writeMessage(ctx, wsTTSTextMessage{Text: ""}, true)
}

func (s *realtimeTTSSession) writeMessage(ctx context.Context, msg wsTTSTextMessage, markFinished bool) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return newDecodeError(fmt.Errorf("marshal realtime tts message: %w", err), nil)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.closed.Load() || s.finished.Load() {
		return core.ErrRealtimeSessionClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}

	if err := s.conn.WriteText(ctx, payload); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if s.closed.Load() {
			return core.ErrRealtimeSessionClosed
		}
		return redactSecretError(newNetworkError(err), s.provider.config.APIKey)
	}
	if markFinished {
		s.finished.Store(true)
	}
	return nil
}

func (s *realtimeTTSSession) Audio() <-chan core.AudioChunk {
	return s.audio
}

func (s *realtimeTTSSession) Err() <-chan error {
	return s.errCh
}

func (s *realtimeTTSSession) Close() error {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		s.cancel()
		s.closeErr = s.conn.Close(websocket.StatusNormalClosure, "client closed session")
	})
	<-s.done
	if isNormalWSClose(s.closeErr) {
		return nil
	}
	return s.closeErr
}

func (s *realtimeTTSSession) readLoop() {
	defer close(s.done)
	defer close(s.audio)
	defer close(s.errCh)
	defer func() {
		s.closed.Store(true)
		s.cancel()
		_ = s.conn.Close(websocket.StatusNormalClosure, "")
	}()

	var seq int64

	for {
		data, err := s.conn.ReadText(s.ctx)
		if err != nil {
			if s.closed.Load() {
				// Clean caller-initiated Close() produces no error.
				return
			}
			if ctxErr := s.ctx.Err(); ctxErr != nil {
				s.errCh <- ctxErr
				return
			}
			if isNormalWSClose(err) {
				return
			}
			s.errCh <- redactSecretError(newNetworkError(err), s.provider.config.APIKey)
			return
		}

		var msg wsTTSServerMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.errCh <- redactSecretError(newDecodeError(err, data), s.provider.config.APIKey)
			return
		}

		if serverErr := mapWSTTSServerError(&msg); serverErr != nil {
			s.errCh <- redactSecretError(serverErr, s.provider.config.APIKey)
			return
		}

		isFinal := msg.IsFinal || msg.IsFinalSnake

		var audioBytes []byte
		if msg.Audio != nil && *msg.Audio != "" {
			decoded, err := base64.StdEncoding.DecodeString(*msg.Audio)
			if err != nil {
				s.errCh <- redactSecretError(
					newDecodeError(fmt.Errorf("decode realtime tts audio base64: %w", err), nil),
					s.provider.config.APIKey,
				)
				return
			}
			audioBytes = decoded
		}

		if len(audioBytes) == 0 && !isFinal {
			// Skip keepalive or alignment-only frames without audio data.
			continue
		}

		chunk := core.AudioChunk{
			Data:     audioBytes,
			Format:   s.format,
			Sequence: seq,
			IsFinal:  isFinal,
		}
		seq++

		select {
		case s.audio <- chunk:
		case <-s.ctx.Done():
			if !s.closed.Load() {
				s.errCh <- s.ctx.Err()
			}
			return
		}
	}
}

func mapWSTTSServerError(msg *wsTTSServerMessage) error {
	hasAudio := msg.Audio != nil && *msg.Audio != ""
	isFinal := msg.IsFinal || msg.IsFinalSnake

	codeStr, codeNum := parseWSTTSCode(msg.Code)
	if msg.Error == "" && (msg.Message == "" || hasAudio || isFinal) {
		return nil
	}

	text := msg.Message
	if text == "" {
		text = msg.Error
	}
	if text == "" {
		text = "elevenlabs realtime tts error"
	}

	errCode := msg.Error
	if errCode == "" {
		errCode = codeStr
	}

	status, sentinel := classifyWSTTSServerError(errCode, codeNum)
	return &core.ProviderError{
		Provider: providerID,
		Status:   status,
		Code:     errCode,
		Message:  text,
		Err:      sentinel,
	}
}

func parseWSTTSCode(raw json.RawMessage) (string, int) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", 0
	}
	var num int
	if err := json.Unmarshal(raw, &num); err == nil {
		return strconv.Itoa(num), num
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if parsed, err := strconv.Atoi(str); err == nil {
			return str, parsed
		}
		return str, 0
	}
	return "", 0
}

func classifyWSTTSServerError(code string, num int) (int, error) {
	switch num {
	case http.StatusUnauthorized, http.StatusForbidden, 1008:
		return http.StatusUnauthorized, core.ErrUnauthorized
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests, core.ErrRateLimited
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return http.StatusBadRequest, core.ErrBadRequest
	}

	lower := strings.ToLower(code)
	switch {
	case strings.Contains(lower, "auth"), strings.Contains(lower, "unauthorized"), strings.Contains(lower, "invalid_api_key"):
		return http.StatusUnauthorized, core.ErrUnauthorized
	case strings.Contains(lower, "rate"), strings.Contains(lower, "quota"), strings.Contains(lower, "limit"):
		return http.StatusTooManyRequests, core.ErrRateLimited
	case strings.Contains(lower, "invalid"), strings.Contains(lower, "validation"), strings.Contains(lower, "bad_request"), strings.Contains(lower, "input"):
		return http.StatusBadRequest, core.ErrBadRequest
	default:
		return http.StatusInternalServerError, core.ErrServer
	}
}

// Compile-time check that ElevenLabs implements core.RealtimeSpeechSynthesizer.
var _ core.RealtimeSpeechSynthesizer = (*ElevenLabs)(nil)
