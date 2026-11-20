package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Speech and voice capability Feature constants for provider and model discovery.
const (
	// FeatureSpeechSynthesis indicates support for synchronous text-to-speech synthesis.
	FeatureSpeechSynthesis Feature = "speech_synthesis"

	// FeatureSpeechSynthesisStreaming indicates support for streaming text-to-speech synthesis.
	FeatureSpeechSynthesisStreaming Feature = "speech_synthesis_streaming"

	// FeatureSpeechSynthesisRealtime indicates support for bidirectional incremental-input
	// real-time text-to-speech synthesis (e.g., streaming LLM text tokens over WebSocket).
	FeatureSpeechSynthesisRealtime Feature = "speech_synthesis_realtime"

	// FeatureSpeechRecognition indicates support for file or buffered speech-to-text transcription.
	FeatureSpeechRecognition Feature = "speech_recognition"

	// FeatureSpeechRecognitionRealtime indicates support for real-time streaming speech-to-text.
	FeatureSpeechRecognitionRealtime Feature = "speech_recognition_realtime"

	// FeatureVoiceSession indicates support for full-duplex real-time voice sessions.
	FeatureVoiceSession Feature = "voice_session"
)

// SpeechSynthesizer is an optional interface for providers that support
// text-to-speech synthesis.
//
// Providers implementing only synchronous synthesis may advertise
// [FeatureSpeechSynthesis] without [FeatureSpeechSynthesisStreaming] and return
// [ErrNotSupported] from StreamSynthesize.
//
// Providers SHOULD be safe for concurrent calls.
type SpeechSynthesizer interface {
	// Synthesize converts text into a complete audio payload.
	Synthesize(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error)

	// StreamSynthesize converts text into a stream of incremental audio chunks.
	// Providers that do not support streaming synthesis return [ErrNotSupported].
	StreamSynthesize(ctx context.Context, req *SpeechRequest) (*SpeechStream, error)
}

// AsSpeechSynthesizer discovers text-to-speech synthesis on a Provider or its
// [ProviderUnwrapper] chain. It returns nil and false when no provider
// implements [SpeechSynthesizer].
func AsSpeechSynthesizer(p Provider) (SpeechSynthesizer, bool) {
	return asProviderCapability[SpeechSynthesizer](p)
}

// SpeechRecognizer is an optional interface for providers that support
// speech-to-text transcription of buffered or reader-backed audio inputs.
//
// Providers SHOULD be safe for concurrent calls.
type SpeechRecognizer interface {
	// Transcribe converts input audio into a structured transcript response.
	Transcribe(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResponse, error)
}

// AsSpeechRecognizer discovers speech-to-text transcription on a Provider or
// its [ProviderUnwrapper] chain. It returns nil and false when no provider
// implements [SpeechRecognizer].
func AsSpeechRecognizer(p Provider) (SpeechRecognizer, bool) {
	return asProviderCapability[SpeechRecognizer](p)
}

// ModelsWithCapability returns all models advertised by p whose Capabilities
// include feature. Returns nil when p is nil or no models match.
func ModelsWithCapability(p Provider, feature Feature) []ModelInfo {
	if p == nil {
		return nil
	}
	var matched []ModelInfo
	for _, m := range p.Models() {
		if m.HasCapability(feature) {
			matched = append(matched, m)
		}
	}
	return matched
}

// ProviderSupportsModelFeature reports whether p advertises model with feature
// in its model catalog. Returns false when p is nil, model is unknown, or the
// model does not declare feature. Unverified model/feature combinations are
// never optimistically treated as supported.
func ProviderSupportsModelFeature(p Provider, model ModelID, feature Feature) bool {
	if p == nil || model == "" {
		return false
	}
	for _, m := range p.Models() {
		if m.ID == model {
			return m.HasCapability(feature)
		}
	}
	return false
}

// Synthesize generates speech audio through the client's provider. It validates
// req, verifies [SpeechSynthesizer] support, and applies client rate limiting,
// timeout, telemetry, and retry policy.
//
// Telemetry events carry provider, model, timing, and error metadata only; raw
// text and audio payloads are never emitted.
func (c *Client) Synthesize(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: speech context cannot be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	synth, ok := AsSpeechSynthesizer(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support speech synthesis: %w", c.speechProviderID(), ErrNotSupported)
	}

	var timeout time.Duration
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
		timeout = c.timeout
	}

	start := time.Now()
	providerID := c.provider.ID()
	ctx = c.startSpeechTelemetry(ctx, providerID, req.Model, start)
	resp, err := c.synthesizeWithRetry(ctx, synth, req)
	if timeout > 0 && err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = newTimeoutError(timeout, providerID, req.Model)
	}
	c.endSpeechTelemetry(ctx, providerID, req.Model, start, err)
	return resp, err
}

func (c *Client) synthesizeWithRetry(ctx context.Context, synth SpeechSynthesizer, req *SpeechRequest) (*SpeechResponse, error) {
	for attempt := 0; ; attempt++ {
		if err := c.waitForRateLimit(ctx); err != nil {
			return nil, err
		}
		resp, err := synth.Synthesize(ctx, req)
		if err == nil {
			return resp, nil
		}
		delay, retry := c.retry.NextDelay(attempt, err)
		if !retry {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// StreamSynthesize initiates a streaming speech synthesis request through the
// client's provider. It validates req, verifies [SpeechSynthesizer] support,
// and applies client rate limiting, timeout, and stream-lifecycle telemetry.
//
// Unlike [Client.Synthesize], streaming setup is not automatically retried
// after the stream starts; the client timeout remains active until the returned
// [SpeechStream] completes or ctx is cancelled.
func (c *Client) StreamSynthesize(ctx context.Context, req *SpeechRequest) (*SpeechStream, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: speech context cannot be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	synth, ok := AsSpeechSynthesizer(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support speech synthesis: %w", c.speechProviderID(), ErrNotSupported)
	}

	var timeout time.Duration
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		timeout = c.timeout
	}

	start := time.Now()
	providerID := c.provider.ID()
	ctx = c.startSpeechTelemetry(ctx, providerID, req.Model, start)

	if err := c.waitForRateLimit(ctx); err != nil {
		if cancel != nil {
			cancel()
		}
		if timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			err = newTimeoutError(timeout, providerID, req.Model)
		}
		c.endSpeechTelemetry(ctx, providerID, req.Model, start, err)
		return nil, err
	}

	stream, err := synth.StreamSynthesize(ctx, req)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		if timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			err = newTimeoutError(timeout, providerID, req.Model)
		}
		c.endSpeechTelemetry(ctx, providerID, req.Model, start, err)
		return nil, err
	}
	if stream == nil {
		if cancel != nil {
			cancel()
		}
		err = ErrNilSpeechStream
		c.endSpeechTelemetry(ctx, providerID, req.Model, start, err)
		return nil, err
	}

	mapErr := func(e error) error {
		if errors.Is(e, ErrTimeout) {
			return e
		}
		if timeout > 0 && errors.Is(e, context.DeadlineExceeded) {
			return newTimeoutError(timeout, providerID, req.Model)
		}
		return e
	}

	return wrapSpeechStreamWithTelemetry(ctx, stream, c.telemetry, providerID, req.Model, start, cancel, mapErr), nil
}

// Transcribe converts audio to text through the client's provider. It validates
// req, verifies [SpeechRecognizer] support, and applies client rate limiting,
// timeout, telemetry, and retry policy.
//
// Automatic retries are only performed for in-memory [AudioData] inputs;
// requests backed by [TranscriptionRequest.Reader] are attempted once because
// an io.Reader cannot be safely rewound after partial consumption.
//
// Telemetry events carry provider, model, timing, and error metadata only; raw
// audio and transcript payloads are never emitted.
func (c *Client) Transcribe(ctx context.Context, req *TranscriptionRequest) (*TranscriptionResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("iris: transcription context cannot be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	recognizer, ok := AsSpeechRecognizer(c.provider)
	if !ok {
		return nil, fmt.Errorf("iris: provider %s does not support speech recognition: %w", c.speechProviderID(), ErrNotSupported)
	}

	var timeout time.Duration
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
		timeout = c.timeout
	}

	start := time.Now()
	providerID := c.provider.ID()
	ctx = c.startSpeechTelemetry(ctx, providerID, req.Model, start)
	resp, err := c.transcribeWithRetry(ctx, recognizer, req)
	if timeout > 0 && err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = newTimeoutError(timeout, providerID, req.Model)
	}
	c.endSpeechTelemetry(ctx, providerID, req.Model, start, err)
	return resp, err
}

func (c *Client) transcribeWithRetry(ctx context.Context, recognizer SpeechRecognizer, req *TranscriptionRequest) (*TranscriptionResponse, error) {
	for attempt := 0; ; attempt++ {
		if err := c.waitForRateLimit(ctx); err != nil {
			return nil, err
		}
		resp, err := recognizer.Transcribe(ctx, req)
		if err == nil {
			return resp, nil
		}
		// Never retry reader-backed requests: the reader may already be partially consumed.
		if req.Reader != nil {
			return resp, err
		}
		delay, retry := c.retry.NextDelay(attempt, err)
		if !retry {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (c *Client) speechProviderID() string {
	if c.provider == nil {
		return "<nil>"
	}
	return c.provider.ID()
}

func (c *Client) startSpeechTelemetry(ctx context.Context, provider string, model ModelID, start time.Time) context.Context {
	event := RequestStartEvent{Provider: provider, Model: model, Start: start}
	if hook, ok := c.telemetry.(ContextualTelemetryHook); ok {
		return hook.OnRequestStartWithContext(ctx, event)
	}
	c.telemetry.OnRequestStart(event)
	return ctx
}

func (c *Client) endSpeechTelemetry(ctx context.Context, provider string, model ModelID, start time.Time, err error) {
	event := RequestEndEvent{
		Provider: provider,
		Model:    model,
		Start:    start,
		End:      time.Now(),
		Err:      err,
	}
	if hook, ok := c.telemetry.(ContextualTelemetryHook); ok {
		hook.OnRequestEndWithContext(ctx, event)
		return
	}
	c.telemetry.OnRequestEnd(event)
}

func wrapSpeechStreamWithTelemetry(
	ctx context.Context,
	stream *SpeechStream,
	hook TelemetryHook,
	provider string,
	model ModelID,
	start time.Time,
	onDone func(),
	mapErr func(error) error,
) *SpeechStream {
	finalCh := make(chan *SpeechResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(finalCh)
		defer close(errCh)
		if onDone != nil {
			defer onDone()
		}

		var finalErr error
		errRecv := stream.Err
		finalRecv := stream.Final

		for errRecv != nil || finalRecv != nil {
			select {
			case resp, ok := <-finalRecv:
				finalRecv = nil
				if ok && resp != nil {
					finalCh <- resp
				}
			case err, ok := <-errRecv:
				errRecv = nil
				if ok && err != nil {
					out := err
					if mapErr != nil {
						out = mapErr(err)
					}
					finalErr = out
					errCh <- out
				}
			}
		}

		endEvent := RequestEndEvent{
			Provider: provider,
			Model:    model,
			Start:    start,
			End:      time.Now(),
			Err:      finalErr,
		}
		if ctxHook, ok := hook.(ContextualTelemetryHook); ok {
			ctxHook.OnRequestEndWithContext(ctx, endEvent)
		} else {
			hook.OnRequestEnd(endEvent)
		}
	}()

	return &SpeechStream{
		Ch:    stream.Ch,
		Err:   errCh,
		Final: finalCh,
	}
}
