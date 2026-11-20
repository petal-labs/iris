package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
	"github.com/petal-labs/iris/providers/internal/timeoutx"
)

func TestNewDefaultsAndOptions(t *testing.T) {
	p := New("  sk_test_123  ")
	if p.ID() != "elevenlabs" {
		t.Fatalf("ID() = %q, want elevenlabs", p.ID())
	}
	if p.config.APIKey.Expose() != "sk_test_123" {
		t.Errorf("APIKey = %q, want trimmed sk_test_123", p.config.APIKey.Expose())
	}
	if p.config.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", p.config.BaseURL, DefaultBaseURL)
	}
	if p.config.HTTPClient != http.DefaultClient {
		t.Error("expected default http.DefaultClient")
	}
	if p.config.Timeout != timeoutx.Default {
		t.Errorf("Timeout = %s, want %s", p.config.Timeout, timeoutx.Default)
	}

	customClient := &http.Client{Timeout: 5 * time.Second}
	p2 := New(
		"initial-key",
		nil,
		WithAPIKey("override-key"),
		WithBaseURL("https://proxy.example.com/elevenlabs///"),
		WithBaseURL("   "), // ignored
		WithHTTPClient(customClient),
		WithHTTPClient(nil), // ignored
		WithHeader("X-Custom", "custom-value"),
		WithHeader("   ", "ignored"),
		WithHeader("xi-api-key", "spoofed"),
		WithTimeout(15*time.Second),
	)
	if p2.config.APIKey.Expose() != "override-key" {
		t.Errorf("APIKey = %q, want override-key", p2.config.APIKey.Expose())
	}
	if p2.config.BaseURL != "https://proxy.example.com/elevenlabs" {
		t.Errorf("BaseURL = %q, want trimmed URL", p2.config.BaseURL)
	}
	if p2.config.HTTPClient != customClient {
		t.Error("expected custom HTTP client to be preserved")
	}
	if p2.config.Timeout != 15*time.Second {
		t.Errorf("Timeout = %s, want 15s", p2.config.Timeout)
	}

	hdrs := p2.buildHeaders("application/json")
	if got := hdrs.Get("xi-api-key"); got != "override-key" {
		t.Errorf("xi-api-key header = %q, want override-key", got)
	}
	if got := hdrs.Get("X-Custom"); got != "custom-value" {
		t.Errorf("X-Custom header = %q, want custom-value", got)
	}
	if got := hdrs.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type header = %q, want application/json", got)
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Run("missing env var", func(t *testing.T) {
		t.Setenv(DefaultAPIKeyEnvVar, "")
		if _, err := NewFromEnv(); !errors.Is(err, ErrAPIKeyNotFound) {
			t.Fatalf("NewFromEnv() err = %v, want ErrAPIKeyNotFound", err)
		}
	})

	t.Run("whitespace env var", func(t *testing.T) {
		t.Setenv(DefaultAPIKeyEnvVar, "   \t\n  ")
		if _, err := NewFromEnv(); !errors.Is(err, ErrAPIKeyNotFound) {
			t.Fatalf("NewFromEnv() err = %v, want ErrAPIKeyNotFound", err)
		}
	})

	t.Run("valid env var with options", func(t *testing.T) {
		t.Setenv(DefaultAPIKeyEnvVar, "  el_live_secret_key  ")
		p, err := NewFromEnv(WithBaseURL("https://api.example.test/"), WithTimeout(10*time.Second))
		if err != nil {
			t.Fatalf("NewFromEnv() err = %v", err)
		}
		if p.config.APIKey.Expose() != "el_live_secret_key" {
			t.Errorf("APIKey = %q, want el_live_secret_key", p.config.APIKey.Expose())
		}
		if p.config.BaseURL != "https://api.example.test" {
			t.Errorf("BaseURL = %q, want https://api.example.test", p.config.BaseURL)
		}
		if p.config.Timeout != 10*time.Second {
			t.Errorf("Timeout = %s, want 10s", p.config.Timeout)
		}
	})
}

func TestAPIKeyAndHeaderRedaction(t *testing.T) {
	secretKey := "xi_super_secret_api_key_999"
	secretHeaderVal := "Bearer_super_secret_proxy_token"

	p := New(secretKey, WithHeader("X-Proxy-Auth", secretHeaderVal))

	outputs := []string{
		p.String(),
		fmt.Sprintf("%#v", p),
		p.config.String(),
		fmt.Sprintf("%#v", p.config),
	}

	rawJSON, err := json.Marshal(p.config)
	if err != nil {
		t.Fatalf("json.Marshal(Config) err = %v", err)
	}
	outputs = append(outputs, string(rawJSON))

	for i, out := range outputs {
		if strings.Contains(out, secretKey) {
			t.Errorf("output[%d] leaked API key: %s", i, out)
		}
		if strings.Contains(out, secretHeaderVal) {
			t.Errorf("output[%d] leaked header value: %s", i, out)
		}
		if !strings.Contains(out, "[REDACTED]") {
			t.Errorf("output[%d] missing [REDACTED] marker: %s", i, out)
		}
	}

	var nilProvider *ElevenLabs
	if got := nilProvider.String(); got != "ElevenLabs{<nil>}" {
		t.Errorf("nil String() = %q", got)
	}
	if got := fmt.Sprintf("%#v", nilProvider); got != "(*elevenlabs.ElevenLabs)(nil)" {
		t.Errorf("nil GoString() = %q", got)
	}
}

func TestProviderInterfaceAndUnsupportedChat(t *testing.T) {
	p := New("test-key")

	if !p.Supports(core.FeatureSpeechSynthesis) || !p.Supports(core.FeatureSpeechSynthesisStreaming) {
		t.Error("expected Supports(FeatureSpeechSynthesis) and Supports(FeatureSpeechSynthesisStreaming) == true")
	}
	if p.Supports(core.FeatureChat) || p.Supports(core.FeatureSpeechRecognition) || p.Supports(core.FeatureVoiceSession) {
		t.Error("Supports() must not advertise unimplemented capabilities")
	}
	if synth, ok := core.AsSpeechSynthesizer(p); !ok || synth != p {
		t.Fatalf("AsSpeechSynthesizer() = (%v, %v), want provider, true", synth, ok)
	}

	if _, err := p.Chat(context.Background(), &core.ChatRequest{}); !errors.Is(err, core.ErrNotSupported) {
		t.Fatalf("Chat() err = %v, want ErrNotSupported", err)
	}
	if _, err := p.StreamChat(context.Background(), &core.ChatRequest{}); !errors.Is(err, core.ErrNotSupported) {
		t.Fatalf("StreamChat() err = %v, want ErrNotSupported", err)
	}
}

func TestModelsCatalogAndDefensiveCopy(t *testing.T) {
	p := New("test-key")
	got := p.Models()
	if len(got) != 5 {
		t.Fatalf("len(Models()) = %d, want 5", len(got))
	}
	if !core.ProviderSupportsModelFeature(p, ModelElevenMultilingualV2, core.FeatureSpeechSynthesis) ||
		!core.ProviderSupportsModelFeature(p, ModelElevenMultilingualV2, core.FeatureSpeechSynthesisStreaming) {
		t.Error("expected ModelElevenMultilingualV2 to support sync and streaming TTS")
	}
	if !core.ProviderSupportsModelFeature(p, ModelElevenFlashV25, core.FeatureSpeechSynthesisStreaming) {
		t.Error("expected ModelElevenFlashV25 to support FeatureSpeechSynthesisStreaming")
	}
	if core.ProviderSupportsModelFeature(p, ModelScribeV2, core.FeatureSpeechSynthesis) {
		t.Error("expected ModelScribeV2 NOT to support FeatureSpeechSynthesis")
	}

	// Mutating returned slice must not mutate catalog.
	got[0].ID = "mutated"
	got[0].Capabilities[0] = core.FeatureChat

	again := p.Models()
	if again[0].ID != ModelElevenMultilingualV2 {
		t.Errorf("catalog mutated: got %q, want %q", again[0].ID, ModelElevenMultilingualV2)
	}
	if len(again[0].Capabilities) != 2 || again[0].Capabilities[0] != core.FeatureSpeechSynthesis || again[0].Capabilities[1] != core.FeatureSpeechSynthesisStreaming {
		t.Errorf("catalog capabilities mutated: got %v", again[0].Capabilities)
	}

	info := GetModelInfo(ModelScribeV2)
	if info == nil || info.ID != ModelScribeV2 {
		t.Fatalf("GetModelInfo(ModelScribeV2) = %+v", info)
	}
	info.DisplayName = "mutated"
	if GetModelInfo(ModelScribeV2).DisplayName == "mutated" {
		t.Error("GetModelInfo did not return a defensive copy")
	}

	if GetModelInfo("nonexistent_model") != nil {
		t.Error("expected nil for unknown model ID")
	}
}

func TestProviderRegistryRegistration(t *testing.T) {
	if !providers.IsRegistered("elevenlabs") {
		t.Fatal("expected elevenlabs to be registered in providers registry")
	}
	p, err := providers.Create("elevenlabs", "reg-key")
	if err != nil {
		t.Fatalf("providers.Create() err = %v", err)
	}
	if p.ID() != "elevenlabs" {
		t.Errorf("ID() = %q, want elevenlabs", p.ID())
	}
}

func TestConcurrentSafety(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("xi-api-key") != "concurrent-key" {
				return nil, errors.New("missing api key")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
	}
	p := New("concurrent-key", WithHTTPClient(client), WithHeader("X-Env", "test"), WithTimeout(time.Second))

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	errCh := make(chan error, workers)

	for range workers {
		go func() {
			defer wg.Done()
			_ = p.ID()
			_ = p.Models()
			_ = p.Supports(core.FeatureSpeechSynthesis)
			_ = p.String()
			resp, err := p.doRequest(context.Background(), http.MethodGet, "/v1/models", "", nil)
			if err != nil {
				errCh <- err
				return
			}
			data, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				errCh <- readErr
				return
			}
			if string(data) != "ok" {
				errCh <- fmt.Errorf("unexpected body %q", data)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent worker error: %v", err)
	}
}
