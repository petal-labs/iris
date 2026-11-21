package elevenlabs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/petal-labs/iris/core"
)

// ElevenLabs Speech Engine Upstream protocol constants.
//
// Reference: https://elevenlabs.io/docs/api-reference/speech-engine/speech-engine-upstream
const (
	// SpeechEngineAuthHeader is the HTTP header carrying ElevenLabs' short-lived
	// HS256 JWT on inbound Speech Engine upstream WebSocket connections.
	SpeechEngineAuthHeader = "X-Elevenlabs-Speech-Engine-Authorization"

	// SpeechEngineJWTIssuer is the expected JWT "iss" claim for Speech Engine upstream.
	SpeechEngineJWTIssuer = "https://api.elevenlabs.io/convai/speech-engine"

	// SpeechEngineJWTSubject is the expected JWT "sub" claim for Speech Engine upstream.
	SpeechEngineJWTSubject = "convai_speech_engine_upstream"

	// DefaultSpeechEngineClockSkew is the clock-skew leeway (60s) specified by ElevenLabs.
	DefaultSpeechEngineClockSkew = 60 * time.Second

	// DefaultSpeechEngineMaxToolRounds caps sequential tool-calling iterations per turn.
	DefaultSpeechEngineMaxToolRounds = 5
)

// Speech Engine sentinel errors.
var (
	// ErrInvalidSpeechEngineConfig indicates SpeechEngineConfig failed validation.
	ErrInvalidSpeechEngineConfig = errors.New("elevenlabs: invalid speech engine config")

	// ErrInvalidSpeechEngineToken indicates inbound Speech Engine JWT verification failed.
	ErrInvalidSpeechEngineToken = errors.New("elevenlabs: invalid speech engine authorization token")
)

// SpeechEngineToolExecutor executes model-requested tool calls during a Speech Engine turn.
type SpeechEngineToolExecutor interface {
	Execute(ctx context.Context, call core.ToolCall) (any, error)
}

// SpeechEngineToolExecutorFunc adapts a function to [SpeechEngineToolExecutor].
type SpeechEngineToolExecutorFunc func(ctx context.Context, call core.ToolCall) (any, error)

// Execute invokes f(ctx, call).
func (f SpeechEngineToolExecutorFunc) Execute(ctx context.Context, call core.ToolCall) (any, error) {
	return f(ctx, call)
}

// callableTool matches tools.Tool structurally without adding a package import.
type callableTool interface {
	Call(ctx context.Context, args json.RawMessage) (any, error)
}

// SpeechEngineConfig configures an ElevenLabs Speech Engine upstream WebSocket
// [SpeechEngineHandler].
//
// Unlike client-initiated TTS/STT endpoints or hosted ElevenAgents, ElevenLabs
// Speech Engine uses an inverted WebSocket protocol where ElevenLabs connects
// outbound to your server's publicly reachable wsUrl, sends transcribed
// conversation history, and synthesizes the streamed text responses produced by
// your chosen Iris LLM provider.
//
// Privacy: String and GoString omit SystemPrompt content and credentials.
type SpeechEngineConfig struct {
	// LLM is any Iris chat provider (OpenAI, Anthropic, Gemini, Ollama, etc.)
	// used to generate assistant replies. Required.
	LLM core.Provider

	// Model is the chat model ID passed to LLM. Required.
	Model core.ModelID

	// SystemPrompt optionally prepends system instructions to each turn's messages.
	// Privacy: omitted from String() and GoString().
	SystemPrompt string

	// Temperature optionally overrides the LLM sampling temperature.
	Temperature *float32

	// MaxTokens optionally caps LLM output tokens per round.
	MaxTokens *int

	// Tools registers optional Iris tools exposed to the LLM. Entries that also
	// implement Call(context.Context, json.RawMessage) (any, error) (such as
	// tools.Tool) are executed automatically when ToolExecutor is nil.
	Tools []core.Tool

	// ToolExecutor optionally executes model-requested tool calls during a turn.
	ToolExecutor SpeechEngineToolExecutor

	// MaxToolRounds caps sequential tool-calling iterations per turn.
	// Zero defaults to DefaultSpeechEngineMaxToolRounds (5).
	MaxToolRounds int

	// DisableAuth disables X-Elevenlabs-Speech-Engine-Authorization JWT
	// verification on inbound HTTP upgrades.
	//
	// WARNING: Only enable this when inbound traffic is strictly restricted at
	// the network layer (e.g., an ElevenLabs egress IP allowlist).
	DisableAuth bool

	// ClockSkew overrides the JWT expiration clock-skew leeway.
	// Zero defaults to DefaultSpeechEngineClockSkew (60s).
	ClockSkew time.Duration

	// OnEvent optionally observes typed core.VoiceEvent lifecycle, transcript,
	// interruption, tool, and error events for metrics or session tracking.
	OnEvent func(core.VoiceEvent)
}

// Validate checks that SpeechEngineConfig has required fields and valid bounds.
// Returned errors wrap [ErrInvalidSpeechEngineConfig].
func (c SpeechEngineConfig) Validate() error {
	if c.LLM == nil {
		return fmt.Errorf("%w: LLM provider is required", ErrInvalidSpeechEngineConfig)
	}
	if strings.TrimSpace(string(c.Model)) == "" {
		return fmt.Errorf("%w: model is required", ErrInvalidSpeechEngineConfig)
	}
	if c.MaxToolRounds < 0 {
		return fmt.Errorf("%w: max_tool_rounds must be non-negative, got %d", ErrInvalidSpeechEngineConfig, c.MaxToolRounds)
	}
	if c.ClockSkew < 0 {
		return fmt.Errorf("%w: clock_skew must be non-negative, got %s", ErrInvalidSpeechEngineConfig, c.ClockSkew)
	}
	for i, t := range c.Tools {
		if t == nil {
			return fmt.Errorf("%w: tools[%d] cannot be nil", ErrInvalidSpeechEngineConfig, i)
		}
		if strings.TrimSpace(t.Name()) == "" {
			return fmt.Errorf("%w: tools[%d] name cannot be empty", ErrInvalidSpeechEngineConfig, i)
		}
	}
	return nil
}

// String returns a log-safe summary omitting SystemPrompt content.
func (c SpeechEngineConfig) String() string {
	llmID := "<nil>"
	if c.LLM != nil {
		llmID = c.LLM.ID()
	}
	return fmt.Sprintf("SpeechEngineConfig{llm=%q, model=%q, prompt_len=%d, tools=%d, disable_auth=%t}",
		llmID, c.Model, len(c.SystemPrompt), len(c.Tools), c.DisableAuth)
}

// GoString returns a log-safe representation for %#v formatting.
func (c SpeechEngineConfig) GoString() string {
	return "elevenlabs." + c.String()
}

// SpeechEngineJWTClaims represents the validated claims of an inbound
// X-Elevenlabs-Speech-Engine-Authorization HS256 JWT.
type SpeechEngineJWTClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	ExpiresAt int64  `json:"exp"`
	NotBefore int64  `json:"nbf,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
}

// VerifySpeechEngineJWT validates an inbound ElevenLabs Speech Engine
// X-Elevenlabs-Speech-Engine-Authorization HS256 JWT using the SHA-256 hash of
// apiKey as the HMAC secret.
//
// It verifies the HS256 algorithm, HMAC-SHA256 signature (constant-time),
// issuer ([SpeechEngineJWTIssuer]), subject ([SpeechEngineJWTSubject]), and
// expiration / not-before timestamps with clockSkew leeway (defaults to
// [DefaultSpeechEngineClockSkew] when clockSkew <= 0).
//
// Returned errors wrap [ErrInvalidSpeechEngineToken] and never expose apiKey
// or raw token bytes.
func VerifySpeechEngineJWT(apiKey, token string, clockSkew time.Duration) (*SpeechEngineJWTClaims, error) {
	return verifySpeechEngineJWTAt(apiKey, token, clockSkew, time.Now())
}

func verifySpeechEngineJWTAt(apiKey, token string, clockSkew time.Duration, now time.Time) (*SpeechEngineJWTClaims, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required for JWT verification", ErrInvalidSpeechEngineToken)
	}
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return nil, fmt.Errorf("%w: authorization token is missing", ErrInvalidSpeechEngineToken)
	}
	if clockSkew <= 0 {
		clockSkew = DefaultSpeechEngineClockSkew
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: malformed JWT structure", ErrInvalidSpeechEngineToken)
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid JWT header encoding", ErrInvalidSpeechEngineToken)
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return nil, fmt.Errorf("%w: invalid JWT header JSON", ErrInvalidSpeechEngineToken)
	}
	if hdr.Alg != "HS256" {
		return nil, fmt.Errorf("%w: unsupported JWT alg %q, want HS256", ErrInvalidSpeechEngineToken, hdr.Alg)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid JWT signature encoding", ErrInvalidSpeechEngineToken)
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	sum := sha256.Sum256([]byte(apiKey))
	hexSecret := []byte(hex.EncodeToString(sum[:]))

	if !verifyHMACSHA256(signingInput, sigBytes, hexSecret) && !verifyHMACSHA256(signingInput, sigBytes, sum[:]) {
		return nil, fmt.Errorf("%w: signature mismatch", ErrInvalidSpeechEngineToken)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid JWT payload encoding", ErrInvalidSpeechEngineToken)
	}
	var claims SpeechEngineJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: invalid JWT payload JSON", ErrInvalidSpeechEngineToken)
	}

	if claims.Issuer != SpeechEngineJWTIssuer {
		return nil, fmt.Errorf("%w: unexpected iss claim %q", ErrInvalidSpeechEngineToken, claims.Issuer)
	}
	if claims.Subject != SpeechEngineJWTSubject {
		return nil, fmt.Errorf("%w: unexpected sub claim %q", ErrInvalidSpeechEngineToken, claims.Subject)
	}
	if claims.ExpiresAt <= 0 {
		return nil, fmt.Errorf("%w: missing exp claim", ErrInvalidSpeechEngineToken)
	}

	nowSec := now.Unix()
	skewSec := int64(clockSkew / time.Second)
	if nowSec > claims.ExpiresAt+skewSec {
		return nil, fmt.Errorf("%w: token expired", ErrInvalidSpeechEngineToken)
	}
	if claims.NotBefore > 0 && nowSec+skewSec < claims.NotBefore {
		return nil, fmt.Errorf("%w: token not yet valid", ErrInvalidSpeechEngineToken)
	}

	return &claims, nil
}

func verifyHMACSHA256(input, sig, secret []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(input)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}

// SpeechEngineHandler implements [http.Handler] for ElevenLabs' Speech Engine
// Upstream WebSocket protocol. Mount it on your HTTP server at the publicly
// reachable path configured in your ElevenLabs Speech Engine wsUrl.
type SpeechEngineHandler struct {
	provider  *ElevenLabs
	cfg       SpeechEngineConfig
	toolIndex map[string]core.Tool
	nowFn     func() time.Time
}

// NewSpeechEngineHandler creates a [SpeechEngineHandler] bound to p's API key
// for inbound JWT verification.
func (p *ElevenLabs) NewSpeechEngineHandler(cfg SpeechEngineConfig) (*SpeechEngineHandler, error) {
	if p == nil {
		return nil, fmt.Errorf("%w: elevenlabs provider cannot be nil", ErrInvalidSpeechEngineConfig)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.DisableAuth {
		if err := p.requireAPIKey(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidSpeechEngineConfig, err)
		}
	}

	if cfg.MaxToolRounds == 0 {
		cfg.MaxToolRounds = DefaultSpeechEngineMaxToolRounds
	}
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = DefaultSpeechEngineClockSkew
	}
	if len(cfg.Tools) > 0 {
		cp := make([]core.Tool, len(cfg.Tools))
		copy(cp, cfg.Tools)
		cfg.Tools = cp
	}

	toolIndex := make(map[string]core.Tool, len(cfg.Tools))
	for _, t := range cfg.Tools {
		toolIndex[t.Name()] = t
	}

	return &SpeechEngineHandler{
		provider:  p,
		cfg:       cfg,
		toolIndex: toolIndex,
		nowFn:     time.Now,
	}, nil
}

// ServeHTTP authenticates the inbound ElevenLabs Speech Engine upgrade request
// (unless DisableAuth is true), accepts the WebSocket connection, and handles
// the conversation lifecycle until close or cancellation.
func (h *SpeechEngineHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.DisableAuth {
		token := r.Header.Get(SpeechEngineAuthHeader)
		if _, err := verifySpeechEngineJWTAt(h.provider.config.APIKey.Expose(), token, h.cfg.ClockSkew, h.nowFn()); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Authenticated server-to-server WebSocket upgrade from ElevenLabs.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(maxWSMessageBytes)

	_ = h.serveConn(r.Context(), &coderWSConn{conn: conn})
}

type speechEngineInbound struct {
	Type           string          `json:"type"`
	ConversationID string          `json:"conversation_id,omitempty"`
	EventID        int64           `json:"event_id,omitempty"`
	UserTranscript json.RawMessage `json:"user_transcript,omitempty"`
	Message        string          `json:"message,omitempty"`
	Error          string          `json:"error,omitempty"`
}

type speechEngineMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type speechEngineAgentResponse struct {
	Type    string `json:"type"`
	EventID int64  `json:"event_id"`
	Content string `json:"content"`
	IsFinal bool   `json:"is_final"`
}

type speechEnginePong struct {
	Type string `json:"type"`
}

func (h *SpeechEngineHandler) serveConn(ctx context.Context, conn wsConn) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sessCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	var (
		writeMu        sync.Mutex
		turnWG         sync.WaitGroup
		conversationID string
		lastEventID    int64
		tracker        = core.NewVoiceTurnTracker("se-turn")
	)
	defer func() {
		tracker.Close()
		turnWG.Wait()
	}()

	writeJSON := func(writeCtx context.Context, v any) error {
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteText(writeCtx, payload)
	}

	emit := func(evt core.VoiceEvent) {
		if h.cfg.OnEvent == nil {
			return
		}
		if evt.SessionID == "" {
			evt.SessionID = conversationID
		}
		if evt.Timestamp.IsZero() {
			evt.Timestamp = h.nowFn()
		}
		h.cfg.OnEvent(evt)
	}

	for {
		data, err := conn.ReadText(sessCtx)
		if err != nil {
			if isNormalWSClose(err) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				emit(core.VoiceEvent{
					Type:  core.VoiceEventSessionEnded,
					State: core.VoiceSessionStateClosed,
				})
				return nil
			}
			netErr := redactSecretError(newNetworkError(err), h.provider.config.APIKey)
			emit(core.VoiceEvent{
				Type:         core.VoiceEventError,
				Err:          netErr,
				ErrorMessage: netErr.Error(),
			})
			return netErr
		}

		var msg speechEngineInbound
		if err := json.Unmarshal(data, &msg); err != nil {
			decErr := redactSecretError(newDecodeError(err, data), h.provider.config.APIKey)
			emit(core.VoiceEvent{
				Type:         core.VoiceEventError,
				Err:          decErr,
				ErrorMessage: decErr.Error(),
			})
			return decErr
		}

		switch msg.Type {
		case "init":
			conversationID = msg.ConversationID
			emit(core.VoiceEvent{
				Type:  core.VoiceEventSessionStarted,
				State: core.VoiceSessionStateActive,
			})

		case "ping":
			if err := writeJSON(sessCtx, speechEnginePong{Type: "pong"}); err != nil {
				return redactSecretError(newNetworkError(err), h.provider.config.APIKey)
			}

		case "close":
			tracker.Close()
			turnWG.Wait()
			emit(core.VoiceEvent{
				Type:  core.VoiceEventSessionEnded,
				State: core.VoiceSessionStateClosed,
			})
			return nil

		case "error":
			errText := msg.Message
			if errText == "" {
				errText = msg.Error
			}
			if errText == "" {
				errText = "speech engine upstream error"
			}
			pErr := redactSecretError(&core.ProviderError{
				Provider: providerID,
				Status:   http.StatusInternalServerError,
				Code:     "speech_engine_error",
				Message:  errText,
				Err:      core.ErrServer,
			}, h.provider.config.APIKey)
			emit(core.VoiceEvent{
				Type:         core.VoiceEventError,
				Err:          pErr,
				ErrorMessage: pErr.Error(),
			})

		case "user_transcript":
			if msg.EventID > 0 && msg.EventID <= lastEventID {
				// Ignore duplicate or out-of-order stale transcript events.
				continue
			}
			if msg.EventID > 0 {
				lastEventID = msg.EventID
			}

			history, latestUserText, err := parseSpeechEngineTranscript(msg.UserTranscript)
			if err != nil {
				decErr := newDecodeError(fmt.Errorf("parse user_transcript: %w", err), nil)
				emit(core.VoiceEvent{
					Type:         core.VoiceEventError,
					Err:          decErr,
					ErrorMessage: decErr.Error(),
				})
				continue
			}
			if len(history) == 0 {
				continue
			}

			// Higher event_id supersedes and cancels any in-flight LLM turn immediately.
			if intr, interrupted := tracker.Interrupt(core.VoiceInterruptionBargeIn); interrupted {
				emit(core.VoiceEvent{
					Type:         core.VoiceEventTurnInterrupted,
					TurnID:       intr.CancelledTurnID,
					Interruption: &intr,
				})
			}

			turnID, turnCtx := tracker.StartTurnWithContext(sessCtx)
			emit(core.VoiceEvent{
				Type:       core.VoiceEventUserTranscriptCommitted,
				TurnID:     turnID,
				Role:       core.RoleUser,
				Transcript: &core.TranscriptionResponse{Text: latestUserText, IsFinal: true},
				IsFinal:    true,
			})

			eventID := msg.EventID
			turnWG.Add(1)
			go func() {
				defer turnWG.Done()
				h.runTurn(turnCtx, tracker, turnID, eventID, history, writeJSON, emit)
			}()
		}
	}
}

func (h *SpeechEngineHandler) runTurn(
	turnCtx context.Context,
	tracker *core.VoiceTurnTracker,
	turnID string,
	eventID int64,
	transcriptHistory []core.Message,
	writeJSON func(context.Context, any) error,
	emit func(core.VoiceEvent),
) {
	if turnCtx.Err() != nil || !tracker.IsCurrentTurn(turnID) {
		return
	}

	emit(core.VoiceEvent{
		Type:   core.VoiceEventTurnStarted,
		TurnID: turnID,
		State:  core.VoiceSessionStateThinking,
	})

	messages := make([]core.Message, 0, len(transcriptHistory)+1)
	if h.cfg.SystemPrompt != "" {
		messages = append(messages, core.Message{
			Role:    core.RoleSystem,
			Content: h.cfg.SystemPrompt,
		})
	}
	messages = append(messages, transcriptHistory...)

	for round := 0; round <= h.cfg.MaxToolRounds; round++ {
		if turnCtx.Err() != nil || !tracker.IsCurrentTurn(turnID) {
			return
		}

		resp, streamed, err := h.runLLMRound(turnCtx, tracker, turnID, eventID, messages, writeJSON, emit)
		if err != nil {
			if turnCtx.Err() != nil || !tracker.IsCurrentTurn(turnID) {
				return
			}
			emit(core.VoiceEvent{
				Type:         core.VoiceEventError,
				TurnID:       turnID,
				Err:          err,
				ErrorMessage: err.Error(),
			})
			tracker.CompleteTurn(turnID)
			return
		}

		if resp.Output != "" || len(resp.ToolCalls) > 0 {
			messages = append(messages, core.Message{
				Role:      core.RoleAssistant,
				Content:   resp.Output,
				ToolCalls: resp.ToolCalls,
			})
		}

		if len(resp.ToolCalls) == 0 {
			if turnCtx.Err() != nil || !tracker.IsCurrentTurn(turnID) {
				return
			}
			if !streamed && resp.Output != "" {
				if err := writeJSON(turnCtx, speechEngineAgentResponse{
					Type:    "agent_response",
					EventID: eventID,
					Content: resp.Output,
					IsFinal: true,
				}); err != nil {
					if turnCtx.Err() == nil {
						emit(core.VoiceEvent{
							Type:         core.VoiceEventError,
							TurnID:       turnID,
							Err:          err,
							ErrorMessage: err.Error(),
						})
					}
					tracker.CompleteTurn(turnID)
					return
				}
			} else if streamed {
				if err := writeJSON(turnCtx, speechEngineAgentResponse{
					Type:    "agent_response",
					EventID: eventID,
					Content: "",
					IsFinal: true,
				}); err != nil {
					if turnCtx.Err() == nil {
						emit(core.VoiceEvent{
							Type:         core.VoiceEventError,
							TurnID:       turnID,
							Err:          err,
							ErrorMessage: err.Error(),
						})
					}
					tracker.CompleteTurn(turnID)
					return
				}
			}

			if resp.Output != "" {
				emit(core.VoiceEvent{
					Type:    core.VoiceEventAgentTextCommitted,
					TurnID:  turnID,
					Role:    core.RoleAssistant,
					Text:    resp.Output,
					IsFinal: true,
				})
			}
			if tracker.CompleteTurn(turnID) {
				emit(core.VoiceEvent{
					Type:   core.VoiceEventTurnCompleted,
					TurnID: turnID,
				})
			}
			return
		}

		if round == h.cfg.MaxToolRounds {
			err := fmt.Errorf("elevenlabs: speech engine turn exceeded max tool rounds (%d)", h.cfg.MaxToolRounds)
			emit(core.VoiceEvent{
				Type:         core.VoiceEventError,
				TurnID:       turnID,
				Err:          err,
				ErrorMessage: err.Error(),
			})
			tracker.CompleteTurn(turnID)
			return
		}

		toolResults, err := h.executeToolCalls(turnCtx, tracker, turnID, resp.ToolCalls, emit)
		if err != nil {
			if turnCtx.Err() != nil || !tracker.IsCurrentTurn(turnID) {
				return
			}
			emit(core.VoiceEvent{
				Type:         core.VoiceEventError,
				TurnID:       turnID,
				Err:          err,
				ErrorMessage: err.Error(),
			})
			tracker.CompleteTurn(turnID)
			return
		}
		messages = append(messages, core.Message{
			Role:        core.RoleTool,
			ToolResults: toolResults,
		})
	}
}

func (h *SpeechEngineHandler) runLLMRound(
	turnCtx context.Context,
	tracker *core.VoiceTurnTracker,
	turnID string,
	eventID int64,
	messages []core.Message,
	writeJSON func(context.Context, any) error,
	emit func(core.VoiceEvent),
) (*core.ChatResponse, bool, error) {
	req := &core.ChatRequest{
		Model:       h.cfg.Model,
		Messages:    messages,
		Temperature: h.cfg.Temperature,
		MaxTokens:   h.cfg.MaxTokens,
		Tools:       h.cfg.Tools,
	}

	if !h.cfg.LLM.Supports(core.FeatureChatStreaming) {
		resp, err := h.cfg.LLM.Chat(turnCtx, req)
		if err != nil {
			return nil, false, err
		}
		if resp == nil {
			return &core.ChatResponse{}, false, nil
		}
		return resp, false, nil
	}

	stream, err := h.cfg.LLM.StreamChat(turnCtx, req)
	if err != nil {
		return nil, false, err
	}
	if stream == nil {
		return nil, false, core.ErrNilStream
	}

	var (
		buf       strings.Builder
		finalResp *core.ChatResponse
		streamErr error
		streamed  bool
		chClosed  = stream.Ch == nil
		errDone   = stream.Err == nil
		finalDone = stream.Final == nil
	)

	for !chClosed || !errDone || !finalDone {
		var chRecv <-chan core.ChatChunk
		if !chClosed {
			chRecv = stream.Ch
		}
		var errRecv <-chan error
		if !errDone {
			errRecv = stream.Err
		}
		var finalRecv <-chan *core.ChatResponse
		if !finalDone {
			finalRecv = stream.Final
		}

		select {
		case <-turnCtx.Done():
			return nil, streamed, turnCtx.Err()

		case chunk, ok := <-chRecv:
			if !ok {
				chClosed = true
				continue
			}
			if chunk.Delta == "" {
				continue
			}
			if !tracker.IsCurrentTurn(turnID) {
				return nil, streamed, context.Canceled
			}
			buf.WriteString(chunk.Delta)
			streamed = true
			emit(core.VoiceEvent{
				Type:    core.VoiceEventAgentTextDelta,
				TurnID:  turnID,
				Role:    core.RoleAssistant,
				Text:    chunk.Delta,
				IsFinal: false,
			})
			if err := writeJSON(turnCtx, speechEngineAgentResponse{
				Type:    "agent_response",
				EventID: eventID,
				Content: chunk.Delta,
				IsFinal: false,
			}); err != nil {
				return nil, streamed, err
			}

		case err, ok := <-errRecv:
			errDone = true
			if ok && err != nil {
				streamErr = err
			}

		case resp, ok := <-finalRecv:
			finalDone = true
			if ok && resp != nil {
				finalResp = resp
			}
		}
	}

	if streamErr != nil {
		return nil, streamed, streamErr
	}
	if finalResp == nil {
		finalResp = &core.ChatResponse{Output: buf.String()}
	} else if finalResp.Output == "" {
		finalResp.Output = buf.String()
	}
	return finalResp, streamed, nil
}

func (h *SpeechEngineHandler) executeToolCalls(
	turnCtx context.Context,
	tracker *core.VoiceTurnTracker,
	turnID string,
	calls []core.ToolCall,
	emit func(core.VoiceEvent),
) ([]core.ToolResult, error) {
	results := make([]core.ToolResult, 0, len(calls))
	for _, call := range calls {
		if turnCtx.Err() != nil || !tracker.IsCurrentTurn(turnID) {
			return nil, turnCtx.Err()
		}

		callCopy := call
		emit(core.VoiceEvent{
			Type:     core.VoiceEventToolCall,
			TurnID:   turnID,
			Role:     core.RoleAssistant,
			ToolCall: &callCopy,
		})

		var (
			out any
			err error
		)
		switch {
		case h.cfg.ToolExecutor != nil:
			out, err = h.cfg.ToolExecutor.Execute(turnCtx, call)
		case h.toolIndex[call.Name] != nil:
			if ct, ok := h.toolIndex[call.Name].(callableTool); ok {
				out, err = ct.Call(turnCtx, call.Arguments)
			} else {
				err = fmt.Errorf("tool %q is not executable", call.Name)
			}
		default:
			err = fmt.Errorf("tool %q not registered", call.Name)
		}
		if turnCtx.Err() != nil {
			return nil, turnCtx.Err()
		}

		res := core.ToolResult{CallID: call.ID, Content: out, IsError: false}
		if err != nil {
			res.Content = err.Error()
			res.IsError = true
		}

		resCopy := res
		emit(core.VoiceEvent{
			Type:       core.VoiceEventToolResult,
			TurnID:     turnID,
			Role:       core.RoleTool,
			ToolResult: &resCopy,
		})
		results = append(results, res)
	}
	return results, nil
}

func parseSpeechEngineTranscript(raw json.RawMessage) ([]core.Message, string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, "", nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []speechEngineMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, "", err
		}
		msgs := make([]core.Message, 0, len(items))
		var lastUser string
		for _, item := range items {
			role := mapSpeechEngineRole(item.Role)
			msgs = append(msgs, core.Message{
				Role:    role,
				Content: item.Content,
			})
			if role == core.RoleUser {
				lastUser = item.Content
			}
		}
		return msgs, lastUser, nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(single) == "" {
		return nil, "", nil
	}
	return []core.Message{{Role: core.RoleUser, Content: single}}, single, nil
}

func mapSpeechEngineRole(r string) core.Role {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "assistant", "agent":
		return core.RoleAssistant
	case "system":
		return core.RoleSystem
	default:
		return core.RoleUser
	}
}

// Compile-time check that SpeechEngineHandler implements http.Handler.
var _ http.Handler = (*SpeechEngineHandler)(nil)
