package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"

	"github.com/petal-labs/iris/core"
)

// DefaultAgentMaxChunkBytes caps each SendAudio payload to an ElevenAgents session (256 KiB).
const DefaultAgentMaxChunkBytes = 256 * 1024

// AgentSignedURLRequest configures a request to mint a temporary signed
// WebSocket URL for a private ElevenAgents conversational agent via
// GET /v1/convai/conversation/get-signed-url.
type AgentSignedURLRequest struct {
	// AgentID is the hosted ElevenAgents agent identifier. Required.
	AgentID string `json:"agent_id"`

	// IncludeConversationID requests a single-use signed URL bound to a
	// server-generated conversation_id.
	IncludeConversationID bool `json:"include_conversation_id,omitempty"`

	// BranchID optionally selects a specific agent branch.
	BranchID string `json:"branch_id,omitempty"`

	// VersionID optionally selects a specific agent version.
	VersionID string `json:"version_id,omitempty"`

	// Environment optionally selects an agent environment (defaults to "production").
	Environment string `json:"environment,omitempty"`
}

// Validate checks that AgentSignedURLRequest contains a non-empty AgentID.
func (r *AgentSignedURLRequest) Validate() error {
	if r == nil || strings.TrimSpace(r.AgentID) == "" {
		return fmt.Errorf("%w: agent_id is required", core.ErrInvalidVoiceSessionConfig)
	}
	return nil
}

// AgentSignedURLResponse holds the temporary signed WebSocket URL returned by
// ElevenLabs. SignedURL is stored as [core.Secret] because it embeds a
// short-lived authentication token.
type AgentSignedURLResponse struct {
	// SignedURL is the authenticated wss:// URL (typically valid for 15 minutes).
	SignedURL core.Secret `json:"signed_url"`
}

// String returns a log-safe summary with the signed URL redacted.
func (r AgentSignedURLResponse) String() string {
	return fmt.Sprintf("AgentSignedURLResponse{signed_url=%s}", r.SignedURL.String())
}

// GoString returns a log-safe representation for %#v formatting.
func (r AgentSignedURLResponse) GoString() string {
	return "elevenlabs." + r.String()
}

// GetAgentSignedURL requests a temporary signed WebSocket URL for a private
// ElevenAgents agent via GET /v1/convai/conversation/get-signed-url.
//
// Backend services can either pass the resulting SignedURL into
// [ElevenLabs.StartAgentSession] or deliver it to trusted client apps so they
// never need access to the ElevenLabs API key.
func (p *ElevenLabs) GetAgentSignedURL(ctx context.Context, req *AgentSignedURLRequest) (*AgentSignedURLResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("agent_id", strings.TrimSpace(req.AgentID))
	if req.IncludeConversationID {
		q.Set("include_conversation_id", "true")
	}
	if strings.TrimSpace(req.BranchID) != "" {
		q.Set("branch_id", strings.TrimSpace(req.BranchID))
	}
	if strings.TrimSpace(req.VersionID) != "" {
		q.Set("version_id", strings.TrimSpace(req.VersionID))
	}
	if strings.TrimSpace(req.Environment) != "" {
		q.Set("environment", strings.TrimSpace(req.Environment))
	}

	path := "/v1/convai/conversation/get-signed-url?" + q.Encode()
	resp, err := p.doRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, redactSecretError(newNetworkError(err), p.config.APIKey)
	}

	var parsed struct {
		SignedURL string `json:"signed_url"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, redactSecretError(newDecodeError(err, body), p.config.APIKey)
	}
	if strings.TrimSpace(parsed.SignedURL) == "" {
		return nil, newDecodeError(errors.New("elevenlabs: get-signed-url response missing signed_url"), nil)
	}

	return &AgentSignedURLResponse{
		SignedURL: core.NewSecret(parsed.SignedURL),
	}, nil
}

// AgentSessionConfig configures a hosted ElevenAgents Conversational AI
// WebSocket session (wss://api.elevenlabs.io/v1/convai/conversation).
//
// Important architectural distinction: ElevenAgents executes its hosted LLM,
// prompt, and turn-taking logic inside ElevenLabs' platform. Use package voice
// (github.com/petal-labs/iris/voice) or [SpeechEngineHandler] when you want
// Iris to control external LLM reasoning directly.
//
// Privacy: String and GoString redact SignedURL, SystemPrompt, FirstMessage,
// and DynamicVariables values.
type AgentSessionConfig struct {
	// AgentID identifies the hosted ElevenAgents agent. Required unless
	// SignedURL is supplied directly.
	AgentID string `json:"agent_id,omitempty"`

	// SignedURL optionally supplies a pre-minted authenticated WebSocket URL
	// returned by [ElevenLabs.GetAgentSignedURL]. When provided, the session
	// dials SignedURL directly without requiring an API key on the client.
	SignedURL core.Secret `json:"signed_url,omitempty"`

	// UseDirectAuth skips automatic signed-URL minting and dials
	// /v1/convai/conversation?agent_id=... directly (useful for public agents
	// or direct xi-api-key header auth). When false and APIKey is configured
	// with no SignedURL, StartAgentSession mints a temporary signed URL first
	// for private agent authentication.
	UseDirectAuth bool `json:"use_direct_auth,omitempty"`

	// BranchID, VersionID, and Environment are forwarded to GetAgentSignedURL
	// when minting a signed URL.
	BranchID    string `json:"branch_id,omitempty"`
	VersionID   string `json:"version_id,omitempty"`
	Environment string `json:"environment,omitempty"`

	// UserID optionally identifies the end user in conversation_initiation_client_data.
	UserID string `json:"user_id,omitempty"`

	// DynamicVariables supplies template variables for the hosted agent prompt.
	DynamicVariables map[string]any `json:"-"`

	// SystemPrompt optionally overrides the hosted agent's prompt (requires
	// prompt override to be enabled on the agent in ElevenLabs).
	SystemPrompt string `json:"system_prompt,omitempty"`

	// FirstMessage optionally overrides the agent's opening utterance.
	FirstMessage string `json:"first_message,omitempty"`

	// Language optionally overrides the agent's language code.
	Language string `json:"language,omitempty"`

	// VoiceID optionally overrides the hosted agent's TTS voice.
	VoiceID string `json:"voice_id,omitempty"`

	// InputFormat describes expected user audio chunks (default PCM 16kHz mono).
	InputFormat core.AudioFormat `json:"input_format,omitempty"`

	// OutputFormat sets the fallback audio format if the server metadata omits it.
	OutputFormat core.AudioFormat `json:"output_format,omitempty"`

	// EventBufferSize bounds the capacity of Events(). Zero defaults to
	// [core.DefaultVoiceEventBufferSize].
	EventBufferSize int `json:"event_buffer_size,omitempty"`

	// MaxAudioChunkBytes bounds each SendAudio chunk. Zero defaults to
	// [DefaultAgentMaxChunkBytes] (256 KiB).
	MaxAudioChunkBytes int `json:"max_audio_chunk_bytes,omitempty"`
}

// Validate checks that AgentSessionConfig specifies either AgentID or SignedURL
// and carries valid format and buffer parameters.
func (c AgentSessionConfig) Validate() error {
	if strings.TrimSpace(c.AgentID) == "" && c.SignedURL.IsEmpty() {
		return fmt.Errorf("%w: either agent_id or signed_url is required", core.ErrInvalidVoiceSessionConfig)
	}
	if !c.SignedURL.IsEmpty() {
		u, err := url.Parse(c.SignedURL.Expose())
		if err != nil || (u.Scheme != "wss" && u.Scheme != "ws") || u.Host == "" {
			return fmt.Errorf("%w: signed_url must be a valid ws:// or wss:// URL", core.ErrInvalidVoiceSessionConfig)
		}
	}
	if !c.InputFormat.IsZero() {
		if err := c.InputFormat.Validate(); err != nil {
			return fmt.Errorf("%w: input_format: %w", core.ErrInvalidVoiceSessionConfig, err)
		}
	}
	if !c.OutputFormat.IsZero() {
		if err := c.OutputFormat.Validate(); err != nil {
			return fmt.Errorf("%w: output_format: %w", core.ErrInvalidVoiceSessionConfig, err)
		}
	}
	if c.EventBufferSize < 0 {
		return fmt.Errorf("%w: event_buffer_size must be non-negative, got %d", core.ErrInvalidVoiceSessionConfig, c.EventBufferSize)
	}
	if c.MaxAudioChunkBytes < 0 {
		return fmt.Errorf("%w: max_audio_chunk_bytes must be non-negative, got %d", core.ErrInvalidVoiceSessionConfig, c.MaxAudioChunkBytes)
	}
	return nil
}

// String returns a log-safe summary omitting SignedURL, SystemPrompt,
// FirstMessage, and DynamicVariables values.
func (c AgentSessionConfig) String() string {
	return fmt.Sprintf(
		"AgentSessionConfig{agent_id=%q, has_signed_url=%t, direct_auth=%t, user_id=%q, prompt_len=%d, dynamic_vars=%d}",
		c.AgentID, !c.SignedURL.IsEmpty(), c.UseDirectAuth, c.UserID, len(c.SystemPrompt), len(c.DynamicVariables),
	)
}

// GoString returns a log-safe representation for %#v formatting.
func (c AgentSessionConfig) GoString() string {
	return "elevenlabs." + c.String()
}

// StartVoiceSession opens a hosted ElevenAgents Conversational AI session
// implementing [core.VoiceSessionProvider].
//
// cfg.AgentID is required (or pass a pre-minted signed URL via
// cfg.Metadata["signed_url"]). When an API key is configured and no signed URL
// is supplied, StartVoiceSession automatically mints a short-lived signed URL
// via [ElevenLabs.GetAgentSignedURL] so private agents authenticate securely.
func (p *ElevenLabs) StartVoiceSession(ctx context.Context, cfg core.VoiceSessionConfig) (core.VoiceSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("elevenlabs: voice session context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	agentCfg := AgentSessionConfig{
		AgentID:            cfg.AgentID,
		SystemPrompt:       cfg.SystemPrompt,
		Language:           cfg.Language,
		VoiceID:            cfg.VoiceID,
		InputFormat:        cfg.InputFormat,
		OutputFormat:       cfg.OutputFormat,
		EventBufferSize:    cfg.EventBufferSize,
		MaxAudioChunkBytes: cfg.MaxAudioChunkBytes,
	}
	if cfg.Metadata != nil {
		if rawSigned := strings.TrimSpace(cfg.Metadata["signed_url"]); rawSigned != "" {
			agentCfg.SignedURL = core.NewSecret(rawSigned)
		}
		if uid := strings.TrimSpace(cfg.Metadata["user_id"]); uid != "" {
			agentCfg.UserID = uid
		}
		if branch := strings.TrimSpace(cfg.Metadata["branch_id"]); branch != "" {
			agentCfg.BranchID = branch
		}
		if version := strings.TrimSpace(cfg.Metadata["version_id"]); version != "" {
			agentCfg.VersionID = version
		}
		if env := strings.TrimSpace(cfg.Metadata["environment"]); env != "" {
			agentCfg.Environment = env
		}
	}

	return p.StartAgentSession(ctx, agentCfg)
}

// StartAgentSession opens a hosted ElevenAgents WebSocket session with full
// control over signed URL authentication, dynamic variables, and overrides.
func (p *ElevenLabs) StartAgentSession(ctx context.Context, cfg AgentSessionConfig) (core.VoiceSession, error) {
	if ctx == nil {
		return nil, fmt.Errorf("elevenlabs: agent session context cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var (
		wsURL       string
		headers     http.Header
		extraSecret core.Secret
	)

	switch {
	case !cfg.SignedURL.IsEmpty():
		wsURL = cfg.SignedURL.Expose()
		extraSecret = cfg.SignedURL
		headers = p.buildPublicWSHeaders()

	case !cfg.UseDirectAuth && !p.config.APIKey.IsEmpty():
		signedResp, err := p.GetAgentSignedURL(ctx, &AgentSignedURLRequest{
			AgentID:     cfg.AgentID,
			BranchID:    cfg.BranchID,
			VersionID:   cfg.VersionID,
			Environment: cfg.Environment,
		})
		if err != nil {
			return nil, err
		}
		wsURL = signedResp.SignedURL.Expose()
		extraSecret = signedResp.SignedURL
		headers = p.buildPublicWSHeaders()

	default:
		q := url.Values{}
		q.Set("agent_id", strings.TrimSpace(cfg.AgentID))
		var err error
		wsURL, err = p.wsEndpointURL("/v1/convai/conversation?" + q.Encode())
		if err != nil {
			return nil, redactSecretError(newNetworkError(err), p.config.APIKey)
		}
		if !p.config.APIKey.IsEmpty() {
			headers = p.buildHeaders("")
		} else {
			headers = p.buildPublicWSHeaders()
		}
	}

	conn, err := p.dialWebSocketURL(ctx, wsURL, headers, extraSecret)
	if err != nil {
		return nil, err
	}

	if initMsg, hasInit := buildAgentInitiationData(cfg); hasInit {
		payload, err := json.Marshal(initMsg)
		if err != nil {
			_ = conn.Close(websocket.StatusInternalError, "marshal init failed")
			return nil, newDecodeError(fmt.Errorf("marshal elevenagents initiation data: %w", err), nil)
		}
		if err := conn.WriteText(ctx, payload); err != nil {
			_ = conn.Close(websocket.StatusInternalError, "write init failed")
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, scrubAgentError(newNetworkError(err), p.config.APIKey, extraSecret)
		}
	}

	inputFormat := cfg.InputFormat
	if inputFormat.IsZero() {
		inputFormat = core.AudioFormatPCM16000
	}
	outputFormat := cfg.OutputFormat
	if outputFormat.IsZero() {
		outputFormat = core.AudioFormatPCM16000
	}
	maxChunkBytes := cfg.MaxAudioChunkBytes
	if maxChunkBytes <= 0 {
		maxChunkBytes = DefaultAgentMaxChunkBytes
	}
	bufSize := cfg.EventBufferSize
	if bufSize <= 0 {
		bufSize = core.DefaultVoiceEventBufferSize
	}

	sessCtx, cancel := context.WithCancel(ctx)
	s := &agentVoiceSession{
		provider:      p,
		extraSecret:   extraSecret,
		conn:          conn,
		ctx:           sessCtx,
		cancel:        cancel,
		maxChunkBytes: maxChunkBytes,
		inputFormat:   inputFormat,
		outputFormat:  outputFormat,
		tracker:       core.NewVoiceTurnTracker("agent-turn"),
		bus:           core.NewVoiceEventEmitter("", bufSize),
		done:          make(chan struct{}),
	}
	s.setState(core.VoiceSessionStateConnecting)

	go s.readLoop()
	return s, nil
}

func (p *ElevenLabs) buildPublicWSHeaders() http.Header {
	headers := make(http.Header)
	for k, vals := range p.config.Headers {
		if strings.EqualFold(k, "xi-api-key") {
			continue
		}
		for _, v := range vals {
			headers.Add(k, v)
		}
	}
	return headers
}

func scrubAgentError(err error, apiKey, extra core.Secret) error {
	err = redactSecretError(err, apiKey)
	if !extra.IsEmpty() {
		err = redactSecretError(err, extra)
	}
	return err
}

// --- ElevenAgents Wire Schemas ---

type agentClientInitData struct {
	Type                       string                   `json:"type"`
	UserID                     string                   `json:"user_id,omitempty"`
	DynamicVariables           map[string]any           `json:"dynamic_variables,omitempty"`
	ConversationConfigOverride *agentConfigOverrideData `json:"conversation_config_override,omitempty"`
}

type agentConfigOverrideData struct {
	Agent *agentOverrideSection `json:"agent,omitempty"`
	TTS   *agentTTSOverride     `json:"tts,omitempty"`
}

type agentOverrideSection struct {
	Prompt       *agentPromptOverride `json:"prompt,omitempty"`
	FirstMessage string               `json:"first_message,omitempty"`
	Language     string               `json:"language,omitempty"`
}

type agentPromptOverride struct {
	Prompt string `json:"prompt,omitempty"`
}

type agentTTSOverride struct {
	VoiceID string `json:"voice_id,omitempty"`
}

func buildAgentInitiationData(cfg AgentSessionConfig) (*agentClientInitData, bool) {
	var override *agentConfigOverrideData
	if cfg.SystemPrompt != "" || cfg.FirstMessage != "" || cfg.Language != "" {
		sec := &agentOverrideSection{
			FirstMessage: cfg.FirstMessage,
			Language:     cfg.Language,
		}
		if cfg.SystemPrompt != "" {
			sec.Prompt = &agentPromptOverride{Prompt: cfg.SystemPrompt}
		}
		override = &agentConfigOverrideData{Agent: sec}
	}
	if cfg.VoiceID != "" {
		if override == nil {
			override = &agentConfigOverrideData{}
		}
		override.TTS = &agentTTSOverride{VoiceID: cfg.VoiceID}
	}

	if cfg.UserID == "" && len(cfg.DynamicVariables) == 0 && override == nil {
		return nil, false
	}

	return &agentClientInitData{
		Type:                       "conversation_initiation_client_data",
		UserID:                     cfg.UserID,
		DynamicVariables:           cfg.DynamicVariables,
		ConversationConfigOverride: override,
	}, true
}

type agentServerEvent struct {
	Type string `json:"type"`

	ConversationInitiationMetadataEvent *struct {
		ConversationID         string `json:"conversation_id"`
		AgentOutputAudioFormat string `json:"agent_output_audio_format"`
		UserInputAudioFormat   string `json:"user_input_audio_format"`
	} `json:"conversation_initiation_metadata_event,omitempty"`

	UserTranscriptionEvent *struct {
		UserTranscript string `json:"user_transcript"`
		EventID        int64  `json:"event_id,omitempty"`
	} `json:"user_transcription_event,omitempty"`

	TentativeUserTranscriptionEvent *struct {
		UserTranscript string `json:"user_transcript"`
		EventID        int64  `json:"event_id,omitempty"`
	} `json:"tentative_user_transcription_event,omitempty"`

	AgentResponseEvent *struct {
		AgentResponse string `json:"agent_response"`
		EventID       int64  `json:"event_id,omitempty"`
	} `json:"agent_response_event,omitempty"`

	AgentResponseCorrectionEvent *struct {
		OriginalAgentResponse  string `json:"original_agent_response"`
		CorrectedAgentResponse string `json:"corrected_agent_response"`
	} `json:"agent_response_correction_event,omitempty"`

	AudioEvent *struct {
		AudioBase64 string `json:"audio_base_64"`
		EventID     int64  `json:"event_id,omitempty"`
	} `json:"audio_event,omitempty"`

	InterruptionEvent *struct {
		EventID int64 `json:"event_id,omitempty"`
	} `json:"interruption_event,omitempty"`

	PingEvent *struct {
		EventID int64 `json:"event_id"`
		PingMS  int   `json:"ping_ms,omitempty"`
	} `json:"ping_event,omitempty"`

	ClientToolCall *struct {
		ToolName   string          `json:"tool_name"`
		ToolCallID string          `json:"tool_call_id"`
		Parameters json.RawMessage `json:"parameters"`
	} `json:"client_tool_call,omitempty"`

	Error   string          `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
	Code    json.RawMessage `json:"code,omitempty"`
}

// --- Active ElevenAgents Session ---

type agentVoiceSession struct {
	provider    *ElevenLabs
	extraSecret core.Secret

	conn    wsConn
	writeMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	maxChunkBytes int

	metaMu       sync.RWMutex
	sessionID    string
	state        core.VoiceSessionState
	inputFormat  core.AudioFormat
	outputFormat core.AudioFormat

	tracker *core.VoiceTurnTracker
	bus     *core.VoiceEventEmitter
	done    chan struct{}

	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func (s *agentVoiceSession) ID() string {
	s.metaMu.RLock()
	defer s.metaMu.RUnlock()
	return s.sessionID
}

func (s *agentVoiceSession) State() core.VoiceSessionState {
	s.metaMu.RLock()
	defer s.metaMu.RUnlock()
	return s.state
}

func (s *agentVoiceSession) setState(st core.VoiceSessionState) {
	s.metaMu.Lock()
	s.state = st
	s.metaMu.Unlock()
}

func (s *agentVoiceSession) currentOutputFormat() core.AudioFormat {
	s.metaMu.RLock()
	defer s.metaMu.RUnlock()
	return s.outputFormat
}

func (s *agentVoiceSession) currentInputFormat() core.AudioFormat {
	s.metaMu.RLock()
	defer s.metaMu.RUnlock()
	return s.inputFormat
}

func (s *agentVoiceSession) SendAudio(ctx context.Context, chunk core.AudioChunk) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if err := chunk.Validate(); err != nil {
		return err
	}
	if len(chunk.Data) > s.maxChunkBytes {
		return fmt.Errorf("%w: chunk size %d exceeds max_audio_chunk_bytes %d",
			core.ErrInvalidAudioChunk, len(chunk.Data), s.maxChunkBytes)
	}
	inFmt := s.currentInputFormat()
	if inFmt.Encoding == core.AudioEncodingPCM && len(chunk.Data)%2 != 0 {
		return fmt.Errorf("%w: 16-bit PCM chunk byte length must be even, got %d",
			core.ErrInvalidAudioChunk, len(chunk.Data))
	}
	if len(chunk.Data) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.writeJSON(ctx, map[string]string{
		"user_audio_chunk": base64.StdEncoding.EncodeToString(chunk.Data),
	})
}

func (s *agentVoiceSession) SendText(ctx context.Context, text string) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: text cannot be empty", core.ErrInvalidSpeechRequest)
	}
	if ctx == nil {
		ctx = s.ctx
	}
	return s.writeJSON(ctx, map[string]string{
		"type": "user_message",
		"text": text,
	})
}

func (s *agentVoiceSession) SubmitToolResult(ctx context.Context, result core.ToolResult) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if strings.TrimSpace(result.CallID) == "" {
		return fmt.Errorf("%w: tool result call_id is required", core.ErrInvalidVoiceSessionConfig)
	}
	if ctx == nil {
		ctx = s.ctx
	}

	var resultStr string
	switch v := result.Content.(type) {
	case nil:
		resultStr = ""
	case string:
		resultStr = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("elevenlabs: marshal client_tool_result content: %w", err)
		}
		resultStr = string(b)
	}

	return s.writeJSON(ctx, map[string]any{
		"type":         "client_tool_result",
		"tool_call_id": result.CallID,
		"result":       resultStr,
		"is_error":     result.IsError,
	})
}

func (s *agentVoiceSession) Interrupt(ctx context.Context) error {
	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
	}
	if ctx == nil {
		ctx = s.ctx
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if intr, ok := s.tracker.Interrupt(core.VoiceInterruptionClientCancel); ok {
		s.setState(core.VoiceSessionStateInterrupted)
		_ = s.bus.Emit(s.ctx, core.VoiceEvent{
			Type:         core.VoiceEventTurnInterrupted,
			SessionID:    s.ID(),
			TurnID:       intr.CancelledTurnID,
			Interruption: &intr,
		})
	}

	return s.writeJSON(ctx, map[string]string{
		"type": "user_activity",
	})
}

func (s *agentVoiceSession) Events() <-chan core.VoiceEvent {
	return s.bus.Events()
}

func (s *agentVoiceSession) Err() <-chan error {
	return s.bus.Err()
}

func (s *agentVoiceSession) Close() error {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		s.tracker.Close()
		s.cancel()
		s.closeErr = s.conn.Close(websocket.StatusNormalClosure, "client closed session")
	})
	<-s.done
	if isNormalWSClose(s.closeErr) {
		return nil
	}
	return s.closeErr
}

func (s *agentVoiceSession) writeJSON(ctx context.Context, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return newDecodeError(fmt.Errorf("marshal elevenagents message: %w", err), nil)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.closed.Load() {
		return core.ErrVoiceSessionClosed
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
			return core.ErrVoiceSessionClosed
		}
		return scrubAgentError(newNetworkError(err), s.provider.config.APIKey, s.extraSecret)
	}
	return nil
}

func (s *agentVoiceSession) ensureTurn() string {
	if id := s.tracker.ActiveTurnID(); id != "" {
		return id
	}
	id, _ := s.tracker.StartTurnWithContext(s.ctx)
	_ = s.bus.Emit(s.ctx, core.VoiceEvent{
		Type:      core.VoiceEventTurnStarted,
		SessionID: s.ID(),
		TurnID:    id,
	})
	return id
}

func (s *agentVoiceSession) readLoop() {
	var termErr error
	defer func() {
		s.closed.Store(true)
		s.tracker.Close()
		s.cancel()
		_ = s.conn.Close(websocket.StatusNormalClosure, "")
		s.setState(core.VoiceSessionStateClosed)
		if termErr == nil && len(s.bus.Events()) < cap(s.bus.Events()) {
			_ = s.bus.Emit(context.Background(), core.VoiceEvent{
				Type:      core.VoiceEventSessionEnded,
				SessionID: s.ID(),
				State:     core.VoiceSessionStateClosed,
			})
		}
		s.bus.Fail(termErr)
		close(s.done)
	}()

	emit := func(evt core.VoiceEvent) bool {
		if err := s.bus.Emit(s.ctx, evt); err != nil {
			if !s.closed.Load() {
				if ctxErr := s.ctx.Err(); ctxErr != nil {
					termErr = ctxErr
				}
			}
			return false
		}
		return true
	}

	var audioSeq int64

	for {
		data, err := s.conn.ReadText(s.ctx)
		if err != nil {
			if s.closed.Load() {
				return
			}
			if ctxErr := s.ctx.Err(); ctxErr != nil {
				termErr = ctxErr
				return
			}
			if isNormalWSClose(err) {
				return
			}
			termErr = scrubAgentError(newNetworkError(err), s.provider.config.APIKey, s.extraSecret)
			return
		}

		var msg agentServerEvent
		if err := json.Unmarshal(data, &msg); err != nil {
			termErr = scrubAgentError(newDecodeError(err, data), s.provider.config.APIKey, s.extraSecret)
			return
		}

		switch msg.Type {
		case "conversation_initiation_metadata":
			if meta := msg.ConversationInitiationMetadataEvent; meta != nil {
				s.metaMu.Lock()
				s.sessionID = meta.ConversationID
				if parsed, ok := parseElevenLabsFormatName(meta.AgentOutputAudioFormat); ok {
					s.outputFormat = parsed
				}
				if parsed, ok := parseElevenLabsFormatName(meta.UserInputAudioFormat); ok {
					s.inputFormat = parsed
				}
				s.state = core.VoiceSessionStateActive
				s.metaMu.Unlock()
			} else {
				s.setState(core.VoiceSessionStateActive)
			}
			if !emit(core.VoiceEvent{
				Type:      core.VoiceEventSessionStarted,
				SessionID: s.ID(),
				State:     core.VoiceSessionStateActive,
			}) {
				return
			}

		case "ping":
			if msg.PingEvent != nil {
				if err := s.writeJSON(s.ctx, map[string]any{
					"type":     "pong",
					"event_id": msg.PingEvent.EventID,
				}); err != nil && !s.closed.Load() {
					if ctxErr := s.ctx.Err(); ctxErr != nil {
						termErr = ctxErr
					} else {
						termErr = err
					}
					return
				}
			}

		case "tentative_user_transcript":
			if ev := msg.TentativeUserTranscriptionEvent; ev != nil {
				if !emit(core.VoiceEvent{
					Type:       core.VoiceEventUserTranscriptPartial,
					SessionID:  s.ID(),
					TurnID:     s.tracker.ActiveTurnID(),
					Role:       core.RoleUser,
					Transcript: &core.TranscriptionResponse{Text: ev.UserTranscript, IsFinal: false},
					IsFinal:    false,
				}) {
					return
				}
			}

		case "user_transcript":
			if ev := msg.UserTranscriptionEvent; ev != nil {
				turnID, _ := s.tracker.StartTurnWithContext(s.ctx)
				s.setState(core.VoiceSessionStateListening)
				if !emit(core.VoiceEvent{
					Type:      core.VoiceEventTurnStarted,
					SessionID: s.ID(),
					TurnID:    turnID,
				}) {
					return
				}
				if !emit(core.VoiceEvent{
					Type:       core.VoiceEventUserTranscriptCommitted,
					SessionID:  s.ID(),
					TurnID:     turnID,
					Role:       core.RoleUser,
					Transcript: &core.TranscriptionResponse{Text: ev.UserTranscript, IsFinal: true},
					IsFinal:    true,
				}) {
					return
				}
			}

		case "agent_response":
			if ev := msg.AgentResponseEvent; ev != nil {
				turnID := s.ensureTurn()
				if !emit(core.VoiceEvent{
					Type:      core.VoiceEventAgentTextCommitted,
					SessionID: s.ID(),
					TurnID:    turnID,
					Role:      core.RoleAssistant,
					Text:      ev.AgentResponse,
					IsFinal:   true,
				}) {
					return
				}
			}

		case "agent_response_correction":
			if ev := msg.AgentResponseCorrectionEvent; ev != nil {
				turnID := s.ensureTurn()
				if !emit(core.VoiceEvent{
					Type:      core.VoiceEventAgentTextCommitted,
					SessionID: s.ID(),
					TurnID:    turnID,
					Role:      core.RoleAssistant,
					Text:      ev.CorrectedAgentResponse,
					IsFinal:   true,
					Metadata:  map[string]string{"corrected": "true"},
				}) {
					return
				}
			}

		case "audio":
			if ev := msg.AudioEvent; ev != nil && ev.AudioBase64 != "" {
				decoded, err := base64.StdEncoding.DecodeString(ev.AudioBase64)
				if err != nil {
					termErr = scrubAgentError(
						newDecodeError(fmt.Errorf("decode elevenagents audio base64: %w", err), nil),
						s.provider.config.APIKey,
						s.extraSecret,
					)
					return
				}
				if len(decoded) == 0 {
					continue
				}
				turnID := s.ensureTurn()
				s.setState(core.VoiceSessionStateSpeaking)
				chunk := core.AudioChunk{
					Data:     decoded,
					Format:   s.currentOutputFormat(),
					Sequence: audioSeq,
				}
				audioSeq++
				if !emit(core.VoiceEvent{
					Type:      core.VoiceEventAudioOutput,
					SessionID: s.ID(),
					TurnID:    turnID,
					Role:      core.RoleAssistant,
					Audio:     &chunk,
				}) {
					return
				}
			}

		case "interruption":
			intr, ok := s.tracker.Interrupt(core.VoiceInterruptionBargeIn)
			if !ok {
				intr = core.VoiceInterruption{
					CancelledTurnID: "active",
					Reason:          core.VoiceInterruptionBargeIn,
				}
			}
			s.setState(core.VoiceSessionStateInterrupted)
			if !emit(core.VoiceEvent{
				Type:         core.VoiceEventTurnInterrupted,
				SessionID:    s.ID(),
				TurnID:       intr.CancelledTurnID,
				Interruption: &intr,
			}) {
				return
			}

		case "client_tool_call":
			if tc := msg.ClientToolCall; tc != nil {
				turnID := s.ensureTurn()
				args := tc.Parameters
				if len(args) == 0 {
					args = json.RawMessage(`{}`)
				}
				if !emit(core.VoiceEvent{
					Type:      core.VoiceEventToolCall,
					SessionID: s.ID(),
					TurnID:    turnID,
					Role:      core.RoleAssistant,
					ToolCall: &core.ToolCall{
						ID:        tc.ToolCallID,
						Name:      tc.ToolName,
						Arguments: args,
					},
				}) {
					return
				}
			}

		case "error":
			codeStr, codeNum := parseWSTTSCode(msg.Code)
			errCode := msg.Error
			if errCode == "" {
				errCode = codeStr
			}
			errMsg := msg.Message
			if errMsg == "" {
				errMsg = msg.Error
			}
			if errMsg == "" {
				errMsg = "elevenagents session error"
			}
			status, sentinel := classifyWSTTSServerError(errCode, codeNum)
			termErr = scrubAgentError(&core.ProviderError{
				Provider: providerID,
				Status:   status,
				Code:     errCode,
				Message:  errMsg,
				Err:      sentinel,
			}, s.provider.config.APIKey, s.extraSecret)
			return
		}
	}
}

func parseElevenLabsFormatName(name string) (core.AudioFormat, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return core.AudioFormat{}, false
	}
	switch name {
	case "ulaw_8000":
		return core.AudioFormatULaw8000, true
	case "alaw_8000":
		return core.AudioFormatALaw8000, true
	}
	if strings.HasPrefix(name, "pcm_") {
		sr, err := strconv.Atoi(strings.TrimPrefix(name, "pcm_"))
		if err == nil && sr > 0 {
			return core.AudioFormat{
				Encoding:   core.AudioEncodingPCM,
				SampleRate: sr,
				Channels:   1,
				MIMEType:   core.AudioEncodingPCM.MIMEType(),
			}, true
		}
	}
	if strings.HasPrefix(name, "mp3_") {
		parts := strings.Split(strings.TrimPrefix(name, "mp3_"), "_")
		if len(parts) == 2 {
			sr, err1 := strconv.Atoi(parts[0])
			br, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && sr > 0 && br > 0 {
				return core.AudioFormat{
					Encoding:   core.AudioEncodingMP3,
					SampleRate: sr,
					BitRate:    br,
					Channels:   1,
					MIMEType:   core.AudioEncodingMP3.MIMEType(),
				}, true
			}
		}
	}
	return core.AudioFormat{}, false
}

// Compile-time checks.
var (
	_ core.VoiceSessionProvider = (*ElevenLabs)(nil)
	_ core.VoiceSession         = (*agentVoiceSession)(nil)
)
