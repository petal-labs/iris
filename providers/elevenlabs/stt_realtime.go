package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/petal-labs/iris/core"
)

// DefaultRealtimeSTTModel is the default ElevenLabs real-time Scribe model used
// when RealtimeTranscriptionConfig.Model is empty.
const DefaultRealtimeSTTModel = ModelScribeV2Realtime

// Bounded defaults for real-time Scribe STT sessions.
const (
	// DefaultRealtimeMaxChunkBytes caps each SendAudio payload (256 KiB).
	DefaultRealtimeMaxChunkBytes = 256 * 1024

	// DefaultRealtimeEventBufferSize is the default bounded capacity of Events().
	DefaultRealtimeEventBufferSize = 32
)

type realtimeAudioChunkMessage struct {
	MessageType string `json:"message_type"`
	AudioBase64 string `json:"audio_base_64"`
	SampleRate  int    `json:"sample_rate,omitempty"`
	Commit      bool   `json:"commit"`
}

type realtimeServerMessage struct {
	MessageType         string       `json:"message_type"`
	SessionID           string       `json:"session_id,omitempty"`
	Text                string       `json:"text,omitempty"`
	LanguageCode        string       `json:"language_code,omitempty"`
	LanguageProbability *float64     `json:"language_probability,omitempty"`
	Words               []scribeWord `json:"words,omitempty"`
	Error               string       `json:"error,omitempty"`
	Message             string       `json:"message,omitempty"`
}

// StartRealtimeTranscription opens a bidirectional WebSocket session to
// ElevenLabs' wss://api.elevenlabs.io/v1/speech-to-text/realtime endpoint.
//
// Supported real-time audio formats are mono raw PCM (8000, 16000, 22050,
// 24000, 44100, or 48000Hz) and G.711 mu-law (8000Hz).
func (p *ElevenLabs) StartRealtimeTranscription(ctx context.Context, cfg core.RealtimeTranscriptionConfig) (core.RealtimeTranscriptionSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("elevenlabs: realtime transcription context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	audioFormatStr, resolvedFormat, err := resolveRealtimeSTTFormat(cfg.Format)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrInvalidTranscriptionRequest, err)
	}

	model := cfg.Model
	if model == "" {
		model = DefaultRealtimeSTTModel
	}

	q := url.Values{}
	q.Set("model_id", string(model))
	q.Set("audio_format", audioFormatStr)
	if cfg.Language != "" {
		q.Set("language_code", cfg.Language)
	}
	if cfg.CommitStrategy != "" {
		q.Set("commit_strategy", string(cfg.CommitStrategy))
	}
	if cfg.IncludeTimestamps {
		q.Set("include_timestamps", "true")
	}

	path := "/v1/speech-to-text/realtime?" + q.Encode()
	conn, err := p.dialWebSocket(ctx, path)
	if err != nil {
		return nil, err
	}

	maxChunkBytes := cfg.MaxChunkBytes
	if maxChunkBytes <= 0 {
		maxChunkBytes = DefaultRealtimeMaxChunkBytes
	}
	eventBuf := cfg.EventBufferSize
	if eventBuf <= 0 {
		eventBuf = DefaultRealtimeEventBufferSize
	}

	sessCtx, cancel := context.WithCancel(ctx)
	s := &realtimeScribeSession{
		provider:      p,
		model:         model,
		format:        resolvedFormat,
		maxChunkBytes: maxChunkBytes,
		conn:          conn,
		ctx:           sessCtx,
		cancel:        cancel,
		events:        make(chan core.RealtimeTranscriptEvent, eventBuf),
		errCh:         make(chan error, 1),
		done:          make(chan struct{}),
	}

	go s.readLoop()
	return s, nil
}

type realtimeScribeSession struct {
	provider      *ElevenLabs
	model         core.ModelID
	format        core.AudioFormat
	maxChunkBytes int

	conn    wsConn
	writeMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	events chan core.RealtimeTranscriptEvent
	errCh  chan error
	done   chan struct{}

	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func (s *realtimeScribeSession) SendAudio(ctx context.Context, chunk core.AudioChunk) error {
	if s.closed.Load() {
		return core.ErrRealtimeSessionClosed
	}
	if ctx == nil {
		ctx = s.ctx
	}
	if err := chunk.Validate(); err != nil {
		return err
	}
	if len(chunk.Data) > s.maxChunkBytes {
		return fmt.Errorf("%w: chunk size %d exceeds max_chunk_bytes %d",
			core.ErrInvalidAudioChunk, len(chunk.Data), s.maxChunkBytes)
	}
	if !chunk.Format.IsZero() {
		if chunk.Format.Encoding != s.format.Encoding ||
			(chunk.Format.SampleRate != 0 && chunk.Format.SampleRate != s.format.SampleRate) {
			return fmt.Errorf("%w: chunk format %s does not match session format %s",
				core.ErrInvalidAudioChunk, chunk.Format.String(), s.format.String())
		}
	}
	if s.format.Encoding == core.AudioEncodingPCM && len(chunk.Data)%2 != 0 {
		return fmt.Errorf("%w: 16-bit PCM chunk byte length must be even, got %d",
			core.ErrInvalidAudioChunk, len(chunk.Data))
	}

	msg := realtimeAudioChunkMessage{
		MessageType: "input_audio_chunk",
		AudioBase64: base64.StdEncoding.EncodeToString(chunk.Data),
		SampleRate:  s.format.SampleRate,
		Commit:      chunk.IsFinal,
	}
	return s.writeMessage(ctx, msg)
}

func (s *realtimeScribeSession) Commit(ctx context.Context) error {
	if s.closed.Load() {
		return core.ErrRealtimeSessionClosed
	}
	if ctx == nil {
		ctx = s.ctx
	}
	msg := realtimeAudioChunkMessage{
		MessageType: "input_audio_chunk",
		AudioBase64: "",
		SampleRate:  s.format.SampleRate,
		Commit:      true,
	}
	return s.writeMessage(ctx, msg)
}

func (s *realtimeScribeSession) writeMessage(ctx context.Context, msg realtimeAudioChunkMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return newDecodeError(fmt.Errorf("marshal realtime audio chunk: %w", err), nil)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.closed.Load() {
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
	return nil
}

func (s *realtimeScribeSession) Events() <-chan core.RealtimeTranscriptEvent {
	return s.events
}

func (s *realtimeScribeSession) Err() <-chan error {
	return s.errCh
}

func (s *realtimeScribeSession) Close() error {
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

func (s *realtimeScribeSession) readLoop() {
	defer close(s.done)
	defer close(s.events)
	defer close(s.errCh)
	defer func() {
		s.closed.Store(true)
		s.cancel()
		_ = s.conn.Close(websocket.StatusNormalClosure, "")
	}()

	var sessionID string

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

		var msg realtimeServerMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.errCh <- redactSecretError(newDecodeError(err, data), s.provider.config.APIKey)
			return
		}

		evt, termErr, emit := s.mapServerMessage(&msg, &sessionID)
		if termErr != nil {
			s.errCh <- redactSecretError(termErr, s.provider.config.APIKey)
			return
		}
		if !emit {
			continue
		}

		select {
		case s.events <- evt:
		case <-s.ctx.Done():
			if !s.closed.Load() {
				s.errCh <- s.ctx.Err()
			}
			return
		}
	}
}

func (s *realtimeScribeSession) mapServerMessage(msg *realtimeServerMessage, sessionID *string) (core.RealtimeTranscriptEvent, error, bool) {
	now := time.Now()
	switch msg.MessageType {
	case "session_started":
		*sessionID = msg.SessionID
		return core.RealtimeTranscriptEvent{
			Type:      core.RealtimeTranscriptEventSessionStarted,
			SessionID: msg.SessionID,
			Timestamp: now,
		}, nil, true

	case "partial_transcript":
		return core.RealtimeTranscriptEvent{
			Type:      core.RealtimeTranscriptEventPartial,
			SessionID: *sessionID,
			IsFinal:   false,
			Timestamp: now,
			Transcript: &core.TranscriptionResponse{
				ID:       *sessionID,
				Model:    s.model,
				Text:     msg.Text,
				Language: msg.LanguageCode,
				IsFinal:  false,
			},
		}, nil, true

	case "committed_transcript":
		return core.RealtimeTranscriptEvent{
			Type:      core.RealtimeTranscriptEventCommitted,
			SessionID: *sessionID,
			IsFinal:   true,
			Timestamp: now,
			Transcript: &core.TranscriptionResponse{
				ID:       *sessionID,
				Model:    s.model,
				Text:     msg.Text,
				Language: msg.LanguageCode,
				IsFinal:  true,
			},
		}, nil, true

	case "committed_transcript_with_timestamps":
		mapped := mapScribeResponse(&scribeResponse{
			TranscriptionID:     *sessionID,
			LanguageCode:        msg.LanguageCode,
			LanguageProbability: msg.LanguageProbability,
			Text:                msg.Text,
			Words:               msg.Words,
		}, s.model, *sessionID, 0)
		return core.RealtimeTranscriptEvent{
			Type:       core.RealtimeTranscriptEventCommitted,
			SessionID:  *sessionID,
			IsFinal:    true,
			Timestamp:  now,
			Transcript: mapped,
		}, nil, true

	default:
		if isRealtimeErrorType(msg.MessageType) {
			return core.RealtimeTranscriptEvent{}, mapRealtimeServerError(msg), false
		}
		// Ignore unknown future informational message types safely.
		return core.RealtimeTranscriptEvent{}, nil, false
	}
}

func isRealtimeErrorType(msgType string) bool {
	switch msgType {
	case "error",
		"auth_error",
		"quota_exceeded",
		"rate_limited",
		"transcriber_error",
		"input_error",
		"chunk_size_exceeded",
		"insufficient_audio_activity",
		"commit_throttled",
		"unaccepted_terms",
		"queue_overflow",
		"resource_exhausted",
		"session_time_limit_exceeded":
		return true
	default:
		return false
	}
}

func mapRealtimeServerError(msg *realtimeServerMessage) error {
	text := msg.Error
	if text == "" {
		text = msg.Message
	}
	if text == "" {
		text = msg.MessageType
	}

	var (
		status   int
		sentinel error
	)
	switch msg.MessageType {
	case "auth_error", "unaccepted_terms":
		status = http.StatusUnauthorized
		sentinel = core.ErrUnauthorized
	case "quota_exceeded", "rate_limited", "commit_throttled", "resource_exhausted":
		status = http.StatusTooManyRequests
		sentinel = core.ErrRateLimited
	case "input_error", "chunk_size_exceeded", "insufficient_audio_activity":
		status = http.StatusBadRequest
		sentinel = core.ErrBadRequest
	default:
		status = http.StatusInternalServerError
		sentinel = core.ErrServer
	}

	return &core.ProviderError{
		Provider: providerID,
		Status:   status,
		Code:     msg.MessageType,
		Message:  text,
		Err:      sentinel,
	}
}

func resolveRealtimeSTTFormat(f core.AudioFormat) (string, core.AudioFormat, error) {
	if err := f.Validate(); err != nil {
		return "", core.AudioFormat{}, err
	}
	if f.Channels > 1 {
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs realtime STT only supports mono (1 channel), got %d",
			core.ErrInvalidAudioFormat, f.Channels)
	}
	switch f.Encoding {
	case core.AudioEncodingPCM:
		return resolvePCMFormat(f)
	case core.AudioEncodingULaw:
		if f.SampleRate != 0 && f.SampleRate != 8000 {
			return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs realtime ulaw requires 8000Hz sample rate, got %dHz",
				core.ErrInvalidAudioFormat, f.SampleRate)
		}
		return "ulaw_8000", core.AudioFormatULaw8000, nil
	default:
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs realtime STT supports pcm or ulaw, got %q",
			core.ErrInvalidAudioFormat, f.Encoding)
	}
}

// Compile-time check that ElevenLabs implements core.RealtimeSpeechRecognizer.
var _ core.RealtimeSpeechRecognizer = (*ElevenLabs)(nil)
