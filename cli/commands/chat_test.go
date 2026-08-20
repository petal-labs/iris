package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

func TestExitError(t *testing.T) {
	err := exitWithCode(ExitValidation, errors.New("test error"))

	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want 'test error'", err.Error())
	}

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitValidation {
		t.Errorf("ExitCode() = %d, want %d", exitErr.ExitCode(), ExitValidation)
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"success", ExitSuccess, 0},
		{"validation", ExitValidation, 1},
		{"provider", ExitProvider, 2},
		{"network", ExitNetwork, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("Exit%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

func TestCreateProviderAllProviders(t *testing.T) {
	app := NewApp()

	tests := []struct {
		providerID string
		apiKey     string
		wantID     string
	}{
		{"openai", "test-key", "openai"},
		{"anthropic", "test-key", "anthropic"},
		{"gemini", "test-key", "gemini"},
		{"xai", "test-key", "xai"},
		{"zai", "test-key", "zai"},
		{"ollama", "", "ollama"},         // ollama works without API key
		{"ollama", "test-key", "ollama"}, // ollama also works with API key
		{"huggingface", "test-key", "huggingface"},
		{"perplexity", "test-key", "perplexity"},
		{"voyageai", "test-key", "voyageai"},
	}

	for _, tt := range tests {
		t.Run(tt.providerID, func(t *testing.T) {
			provider, err := app.createProvider(tt.providerID, tt.apiKey, nil)
			if err != nil {
				t.Fatalf("createProvider(%q, %q) error = %v", tt.providerID, tt.apiKey, err)
			}

			if provider.ID() != tt.wantID {
				t.Errorf("provider.ID() = %q, want %q", provider.ID(), tt.wantID)
			}
		})
	}
}

func TestCreateProviderUsesPerplexityBaseURL(t *testing.T) {
	var gotRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = true
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp-1","model":"sonar","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	app := NewApp()
	provider, err := app.createProvider("perplexity", "test-key", &config.Config{
		Providers: map[string]config.ProviderConfig{
			"perplexity": {BaseURL: server.URL},
		},
	})
	if err != nil {
		t.Fatalf("createProvider(perplexity) error = %v", err)
	}

	_, err = provider.Chat(context.Background(), &core.ChatRequest{
		Model:    "sonar",
		Messages: []core.Message{{Role: core.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("perplexity.Chat() error = %v", err)
	}
	if !gotRequest {
		t.Error("perplexity request did not use the configured base URL")
	}
}

func TestCreateProviderUsesVoyageAIBaseURL(t *testing.T) {
	var gotRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = true
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %q, want /embeddings", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"voyage-4-large","usage":{"total_tokens":1}}`)
	}))
	defer server.Close()

	app := NewApp()
	provider, err := app.createProvider("voyageai", "test-key", &config.Config{
		Providers: map[string]config.ProviderConfig{
			"voyageai": {BaseURL: server.URL},
		},
	})
	if err != nil {
		t.Fatalf("createProvider(voyageai) error = %v", err)
	}

	embedder, ok := provider.(core.EmbeddingProvider)
	if !ok {
		t.Fatal("voyageai provider does not implement core.EmbeddingProvider")
	}
	_, err = embedder.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "voyage-4-large",
		Input: []core.EmbeddingInput{{Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("voyageai.CreateEmbeddings() error = %v", err)
	}
	if !gotRequest {
		t.Error("voyageai request did not use the configured base URL")
	}
}

func TestCreateAzureFoundryProviderFromConfig(t *testing.T) {
	server := newAzureChatServer(t, "/openai/deployments/chat-prod/chat/completions", "2025-01-01-preview")
	defer server.Close()

	app := NewApp()
	provider, err := app.createProvider("azurefoundry", "test-key", &config.Config{
		Providers: map[string]config.ProviderConfig{
			"azurefoundry": {
				Endpoint:          server.URL,
				DeploymentID:      "chat-prod",
				APIVersion:        "2025-01-01-preview",
				UseOpenAIEndpoint: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("createProvider(azurefoundry) error = %v", err)
	}

	assertAzureChatSucceeds(t, provider)
}

func TestCreateAzureFoundryProviderFromEnvironment(t *testing.T) {
	server := newAzureChatServer(t, "/openai/deployments/env-deployment/chat/completions", "2024-10-21")
	defer server.Close()
	t.Setenv("AZURE_AI_ENDPOINT", server.URL)
	t.Setenv("AZURE_AI_DEPLOYMENT_ID", "env-deployment")

	app := NewApp()
	provider, err := app.createProvider("azurefoundry", "test-key", &config.Config{
		Providers: map[string]config.ProviderConfig{
			"azurefoundry": {UseOpenAIEndpoint: true},
		},
	})
	if err != nil {
		t.Fatalf("createProvider(azurefoundry) error = %v", err)
	}

	assertAzureChatSucceeds(t, provider)
}

func TestCreateAzureFoundryProviderRequiresEndpoint(t *testing.T) {
	t.Setenv("AZURE_AI_ENDPOINT", "")
	app := NewApp()
	_, err := app.createProvider("azurefoundry", "test-key", nil)
	if err == nil {
		t.Fatal("createProvider(azurefoundry) should require an endpoint")
	}
	if !strings.Contains(err.Error(), "endpoint") || !strings.Contains(err.Error(), "AZURE_AI_ENDPOINT") {
		t.Errorf("error = %q, want endpoint configuration guidance", err)
	}
}

func TestCreateAzureFoundryProviderRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ProviderConfig
	}{
		{
			name: "non HTTP endpoint",
			cfg:  config.ProviderConfig{Endpoint: "file:///tmp/azure"},
		},
		{
			name: "endpoint credentials",
			cfg:  config.ProviderConfig{Endpoint: "https://user:secret@example.com"},
		},
		{
			name: "unsafe deployment ID",
			cfg: config.ProviderConfig{
				Endpoint:     "https://example.openai.azure.com",
				DeploymentID: "chat/prod",
			},
		},
		{
			name: "unsafe API version",
			cfg: config.ProviderConfig{
				Endpoint:   "https://example.openai.azure.com",
				APIVersion: "2025-01-01&mode=unsafe",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			_, err := app.createProvider("azurefoundry", "test-key", &config.Config{
				Providers: map[string]config.ProviderConfig{"azurefoundry": tt.cfg},
			})
			if err == nil {
				t.Fatal("createProvider(azurefoundry) should reject invalid config")
			}
		})
	}
}

func newAzureChatServer(t *testing.T, wantPath, wantAPIVersion string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if got := r.URL.Query().Get("api-version"); got != wantAPIVersion {
			t.Errorf("api-version = %q, want %q", got, wantAPIVersion)
		}
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Errorf("api-key = %q, want test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp-1","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
}

func assertAzureChatSucceeds(t *testing.T, provider core.Provider) {
	t.Helper()
	_, err := provider.Chat(context.Background(), &core.ChatRequest{
		Model:    "gpt-4o",
		Messages: []core.Message{{Role: core.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("azurefoundry.Chat() error = %v", err)
	}
}

func TestCreateProviderUnsupported(t *testing.T) {
	app := NewApp()
	_, err := app.createProvider("unsupported", "test-key", nil)
	if err == nil {
		t.Fatal("createProvider() should return error for unsupported provider")
	}
}

func TestCreateProviderErrorMessage(t *testing.T) {
	app := NewApp()
	_, err := app.createProvider("nonexistent", "test-key", nil)
	if err == nil {
		t.Fatal("createProvider() should return error")
	}

	// Error should mention unsupported provider
	errMsg := err.Error()
	if !contains(errMsg, "unsupported provider") {
		t.Errorf("Error message should contain 'unsupported provider', got: %q", errMsg)
	}
}

// contains checks if s contains substr (simple helper for tests)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHandleChatErrorValidation(t *testing.T) {
	app := NewApp()
	// Test with validation error
	err := app.handleChatError(core.ErrModelRequired)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitValidation {
		t.Errorf("ExitCode() = %d, want %d (ExitValidation)", exitErr.ExitCode(), ExitValidation)
	}
}

func TestHandleChatErrorNetwork(t *testing.T) {
	app := NewApp()
	err := app.handleChatError(core.ErrNetwork)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitNetwork {
		t.Errorf("ExitCode() = %d, want %d (ExitNetwork)", exitErr.ExitCode(), ExitNetwork)
	}
}

func TestHandleChatErrorProvider(t *testing.T) {
	app := NewApp()
	provErr := &core.ProviderError{
		Provider:  "openai",
		Status:    429,
		RequestID: "req_123",
		Code:      "rate_limited",
		Message:   "Too many requests",
		Err:       core.ErrRateLimited,
	}

	err := app.handleChatError(provErr)

	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatal("expected *exitError type")
	}

	if exitErr.ExitCode() != ExitProvider {
		t.Errorf("ExitCode() = %d, want %d (ExitProvider)", exitErr.ExitCode(), ExitProvider)
	}
}

func TestProviderAllowsEmptyAPIKey(t *testing.T) {
	if !providerAllowsEmptyAPIKey("ollama") {
		t.Error("providerAllowsEmptyAPIKey(ollama) = false, want true")
	}
	for _, id := range []string{"openai", "anthropic", "gemini", "xai", "zai", "huggingface", ""} {
		if providerAllowsEmptyAPIKey(id) {
			t.Errorf("providerAllowsEmptyAPIKey(%q) = true, want false", id)
		}
	}
}

// TestRunChatKeylessOllama verifies that a missing keystore entry does not
// abort the request for providers that run without an API key (local
// Ollama). The fake Ollama server proves the request actually went out.
func TestRunChatKeylessOllama(t *testing.T) {
	var gotRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		gotRequest = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"llama3.2","created_at":"2026-08-17T00:00:00Z","message":{"role":"assistant","content":"hello"},"done":true}`)
	}))
	defer server.Close()

	// Keystore at an empty path: Get returns ErrKeyNotFound for everything.
	app := NewApp(
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}),
	)
	// runChat is invoked directly, so config is assigned here instead of
	// via the root command's PersistentPreRunE.
	app.cfg = &config.Config{Providers: map[string]config.ProviderConfig{
		"ollama": {BaseURL: server.URL},
	}}
	app.provider = "ollama"
	app.model = "llama3.2"
	app.chatPrompt = "hi"

	err := app.runChat(nil, nil)
	if err != nil {
		t.Fatalf("runChat() error = %v; want success with no stored key (local ollama)", err)
	}
	if !gotRequest {
		t.Error("no request reached the ollama server; keyless path did not proceed")
	}
}

func TestRunChatPerplexityWithStoredKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want stored key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp-1","model":"sonar","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	app, stdout := appWithStoredKey(t, "perplexity")
	app.cfg = &config.Config{Providers: map[string]config.ProviderConfig{
		"perplexity": {BaseURL: server.URL},
	}}
	app.provider = "perplexity"
	app.model = "sonar"
	app.chatPrompt = "hi"

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "hello") {
		t.Errorf("stdout = %q, want provider response", got)
	}
}

func TestRunChatAzureFoundryWithStoredKeyAndEnvironment(t *testing.T) {
	server := newAzureChatServer(t, "/models/chat/completions", "2024-05-01-preview")
	defer server.Close()
	t.Setenv("AZURE_AI_ENDPOINT", server.URL)

	app, stdout := appWithStoredKey(t, "azurefoundry")
	app.cfg = &config.Config{Providers: map[string]config.ProviderConfig{}}
	app.provider = "azurefoundry"
	app.model = "gpt-4o"
	app.chatPrompt = "hi"

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "hello") {
		t.Errorf("stdout = %q, want provider response", got)
	}
}

func TestRunChatVoyageAIReportsUnsupportedCapability(t *testing.T) {
	app, _ := appWithStoredKey(t, "voyageai")
	app.cfg = &config.Config{Providers: map[string]config.ProviderConfig{}}
	app.provider = "voyageai"
	app.model = "voyage-4-large"
	app.chatPrompt = "hi"

	err := app.runChat(nil, nil)
	if err == nil {
		t.Fatal("runChat() should report Voyage AI's unsupported chat capability")
	}
	if !strings.Contains(err.Error(), "chat is not supported by Voyage AI") {
		t.Errorf("error = %q, want Voyage AI capability error", err)
	}
	if strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("error = %q, provider should be constructed before capability validation", err)
	}
}

func appWithStoredKey(t *testing.T, providerID string) (*App, *bytes.Buffer) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "keys.enc")
	store, err := keystore.NewKeystoreAtPath(keyPath)
	if err != nil {
		t.Fatalf("create keystore: %v", err)
	}
	if err := store.Set(providerID, "test-key"); err != nil {
		t.Fatalf("store %s key: %v", providerID, err)
	}

	stdout := &bytes.Buffer{}
	app := NewApp(
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(keyPath)
		}),
		WithIO(strings.NewReader(""), stdout, &bytes.Buffer{}),
	)
	return app, stdout
}

// TestRunChatMissingKeyFailsForKeyedProviders verifies the actionable
// keystore hint still fires for providers that require a key.
func TestRunChatMissingKeyFailsForKeyedProviders(t *testing.T) {
	app := NewApp(
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}),
	)
	app.provider = "openai"
	app.model = "gpt-4o"
	app.chatPrompt = "hi"

	err := app.runChat(nil, nil)
	if err == nil {
		t.Fatal("runChat() should fail for openai with no stored key")
	}
	if !strings.Contains(err.Error(), "iris keys set openai") {
		t.Errorf("error = %q, want actionable 'iris keys set openai' hint", err.Error())
	}
}

// writeSchemaFile writes data to a temporary file and returns its path.
func writeSchemaFile(t *testing.T, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(p, []byte(data), 0644); err != nil {
		t.Fatalf("write schema file: %v", err)
	}
	return p
}

// TestRunChatSchemaStrict verifies --schema sets ResponseFormatJSONSchema with
// Strict=true and forwards the schema bytes to the provider.
func TestRunChatSchemaStrict(t *testing.T) {
	var gotReq *core.ChatRequest
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			gotReq = req
			return &core.ChatResponse{ID: "r", Model: req.Model, Output: `{"name":"John"}`}, nil
		},
	}
	app, _, _ := appWithMockProvider(t, mp)
	app.chatPrompt = "Extract: John is 30"
	app.chatSchema = writeSchemaFile(t, `{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string"}},"required":["name"]}`)

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if gotReq == nil {
		t.Fatal("provider.Chat was not called")
	}
	if gotReq.ResponseFormat != core.ResponseFormatJSONSchema {
		t.Errorf("ResponseFormat = %q, want %q", gotReq.ResponseFormat, core.ResponseFormatJSONSchema)
	}
	if gotReq.JSONSchema == nil {
		t.Fatal("JSONSchema is nil")
	}
	if !gotReq.JSONSchema.Strict {
		t.Error("JSONSchema.Strict = false, want true (strict mode)")
	}
	if gotReq.JSONSchema.Name != "schema" {
		t.Errorf("JSONSchema.Name = %q, want %q", gotReq.JSONSchema.Name, "schema")
	}
}

// TestRunChatSchemaNonStrict verifies --schema-non-strict relaxes strict mode.
func TestRunChatSchemaNonStrict(t *testing.T) {
	var gotReq *core.ChatRequest
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			gotReq = req
			return &core.ChatResponse{Output: `{}`}, nil
		},
	}
	app, _, _ := appWithMockProvider(t, mp)
	app.chatPrompt = "Extract"
	// Schema is intentionally not strict-compliant (missing
	// additionalProperties:false and required); non-strict mode must accept it.
	app.chatSchema = writeSchemaFile(t, `{"type":"object","properties":{"name":{"type":"string"}}}`)
	app.chatSchemaNonStrict = true

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if gotReq.JSONSchema == nil || gotReq.JSONSchema.Strict {
		t.Errorf("got Strict = true, want false in non-strict mode")
	}
}

// TestRunChatSchemaStrictRejected verifies a non-strict-compliant schema under
// strict mode fails with ErrInvalidSchema (ExitValidation) before any HTTP call.
func TestRunChatSchemaStrictRejected(t *testing.T) {
	called := false
	mp := &mockChatProvider{
		chat: func(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
			called = true
			return &core.ChatResponse{}, nil
		},
	}
	app, _, _ := appWithMockProvider(t, mp)
	app.chatPrompt = "Extract"
	app.chatSchema = writeSchemaFile(t, `{"type":"object","properties":{"name":{"type":"string"}}}`) // not strict

	err := app.runChat(nil, nil)
	if err == nil {
		t.Fatal("runChat() should fail for non-strict-compliant schema in strict mode")
	}
	if called {
		t.Error("provider.Chat should not be called when schema validation fails")
	}
	exitErr, ok := err.(*exitError)
	if !ok || exitErr.ExitCode() != ExitValidation {
		t.Errorf("exit code = %v, want ExitValidation", err)
	}
}

// TestLoadSchemaErrors covers file-not-found and invalid-JSON cases.
func TestLoadSchemaErrors(t *testing.T) {
	if _, err := loadSchema(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("loadSchema(missing) should error")
	}
	bad := writeSchemaFile(t, `{not valid json`)
	if _, err := loadSchema(bad); err == nil {
		t.Fatal("loadSchema(invalid JSON) should error")
	}
}

// TestRunChatPositionalArg verifies a positional argument is used as the prompt.
func TestRunChatPositionalArg(t *testing.T) {
	var gotReq *core.ChatRequest
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			gotReq = req
			return &core.ChatResponse{Output: "ok"}, nil
		},
	}
	app, _, _ := appWithMockProvider(t, mp)
	app.chatPrompt = "" // no --prompt

	if err := app.runChat(nil, []string{"Hello from arg"}); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if gotReq == nil {
		t.Fatal("provider.Chat was not called")
	}
	last := gotReq.Messages[len(gotReq.Messages)-1]
	if last.Role != core.RoleUser || last.Content != "Hello from arg" {
		t.Errorf("last message = {%s, %q}, want user/Hello from arg", last.Role, last.Content)
	}
}

// TestRunChatStdinPipe verifies piped stdin is treated as a one-shot prompt.
func TestRunChatStdinPipe(t *testing.T) {
	var gotReq *core.ChatRequest
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			gotReq = req
			return &core.ChatResponse{Output: "ok"}, nil
		},
	}
	var stdout bytes.Buffer
	app := NewApp(
		WithProviderFactory(func(string, string, *config.Config) (core.Provider, error) { return mp, nil }),
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader("piped prompt\n"), &stdout, &bytes.Buffer{}),
	)
	app.cfg = &config.Config{}
	app.provider = "ollama"
	app.model = "llama3.2"

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if gotReq == nil {
		t.Fatal("provider.Chat was not called")
	}
	last := gotReq.Messages[len(gotReq.Messages)-1]
	if last.Content != "piped prompt" {
		t.Errorf("last user message = %q, want %q", last.Content, "piped prompt")
	}
}

// TestRunChatNoPromptError verifies an empty prompt (no flag, no arg, empty
// stdin, non-interactive) yields a validation error rather than entering REPL.
func TestRunChatNoPromptError(t *testing.T) {
	mp := &mockChatProvider{}
	app, _, _ := appWithMockProvider(t, mp)
	app.chatPrompt = ""

	err := app.runChat(nil, nil)
	if err == nil {
		t.Fatal("runChat() should error when no prompt is available")
	}
	if !strings.Contains(err.Error(), "no prompt") {
		t.Errorf("error = %q, want 'no prompt' guidance", err.Error())
	}
}

// TestRunChatREPL verifies the interactive REPL: multiple turns are sent
// through a Conversation, so history accumulates across turns. Ctrl-D (EOF on
// the piped reader) exits cleanly.
func TestRunChatREPL(t *testing.T) {
	type capturedReq struct {
		msgs []core.Message
	}
	var captured []capturedReq
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			cp := make([]core.Message, len(req.Messages))
			copy(cp, req.Messages)
			captured = append(captured, capturedReq{msgs: cp})
			return &core.ChatResponse{Output: fmt.Sprintf("answer%d", len(captured))}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	app := NewApp(
		WithProviderFactory(func(string, string, *config.Config) (core.Provider, error) { return mp, nil }),
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader("hello\nhow are you\n"), &stdout, &stderr),
	)
	app.cfg = &config.Config{}
	app.provider = "ollama"
	app.model = "llama3.2"
	app.chatInteractive = true // force REPL despite non-terminal stdin

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if len(captured) != 2 {
		t.Fatalf("provider.Chat called %d times, want 2", len(captured))
	}
	// Turn 1: only the user message.
	if len(captured[0].msgs) != 1 || captured[0].msgs[0].Content != "hello" {
		t.Errorf("turn 1 messages = %v, want [user:hello]", captured[0].msgs)
	}
	// Turn 2: history includes turn 1's user + assistant + turn 2's user.
	if len(captured[1].msgs) != 3 {
		t.Fatalf("turn 2 messages = %v, want 3 (history + new user)", captured[1].msgs)
	}
	if captured[1].msgs[0].Content != "hello" || captured[1].msgs[0].Role != core.RoleUser {
		t.Errorf("turn 2 msg[0] = %v, want user:hello", captured[1].msgs[0])
	}
	if captured[1].msgs[1].Content != "answer1" || captured[1].msgs[1].Role != core.RoleAssistant {
		t.Errorf("turn 2 msg[1] = %v, want assistant:answer1", captured[1].msgs[1])
	}
	if captured[1].msgs[2].Content != "how are you" || captured[1].msgs[2].Role != core.RoleUser {
		t.Errorf("turn 2 msg[2] = %v, want user:how are you", captured[1].msgs[2])
	}
	if !strings.Contains(stdout.String(), "answer1") || !strings.Contains(stdout.String(), "answer2") {
		t.Errorf("stdout = %q, want both answers", stdout.String())
	}
}

// TestRunChatREPLEmptyInput verifies an immediate EOF in REPL mode exits cleanly
// without invoking the provider.
func TestRunChatREPLEmptyInput(t *testing.T) {
	called := false
	mp := &mockChatProvider{
		chat: func(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
			called = true
			return &core.ChatResponse{Output: "x"}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	app := NewApp(
		WithProviderFactory(func(string, string, *config.Config) (core.Provider, error) { return mp, nil }),
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &stdout, &stderr),
	)
	app.cfg = &config.Config{}
	app.provider = "ollama"
	app.model = "llama3.2"
	app.chatInteractive = true

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if called {
		t.Error("provider.Chat should not be called on immediate EOF")
	}
}

// TestRunChatREPLSystemMessage verifies a --system message seeds the
// conversation history in REPL mode.
func TestRunChatREPLSystemMessage(t *testing.T) {
	var firstReq *core.ChatRequest
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			if firstReq == nil {
				firstReq = req
			}
			return &core.ChatResponse{Output: "ok"}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	app := NewApp(
		WithProviderFactory(func(string, string, *config.Config) (core.Provider, error) { return mp, nil }),
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader("hi\n"), &stdout, &stderr),
	)
	app.cfg = &config.Config{}
	app.provider = "ollama"
	app.model = "llama3.2"
	app.chatInteractive = true
	app.chatSystem = "Be concise"

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if firstReq == nil || len(firstReq.Messages) < 2 {
		t.Fatalf("expected system + user messages, got %v", firstReq)
	}
	if firstReq.Messages[0].Role != core.RoleSystem || firstReq.Messages[0].Content != "Be concise" {
		t.Errorf("msg[0] = %v, want system/Be concise", firstReq.Messages[0])
	}
}

// TestRunChatTimeout verifies --timeout cancels an unresponsive provider and
// surfaces ExitTimeout.
func TestRunChatTimeout(t *testing.T) {
	mp := &mockChatProvider{
		chat: func(ctx context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	app, _, _ := appWithMockProvider(t, mp)
	app.chatPrompt = "hi"
	app.chatTimeout = 50 * time.Millisecond

	err := app.runChat(nil, nil)
	if err == nil {
		t.Fatal("runChat() should time out")
	}
	exitErr, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error type = %T, want *exitError", err)
	}
	if exitErr.ExitCode() != ExitTimeout {
		t.Errorf("exit code = %d, want ExitTimeout (%d)", exitErr.ExitCode(), ExitTimeout)
	}
}

// TestRunChatVerboseNonStreaming verifies --verbose prints a token usage
// summary for non-streaming responses (previously only streaming did).
func TestRunChatVerboseNonStreaming(t *testing.T) {
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			return &core.ChatResponse{
				Output: "ok",
				Usage:  core.TokenUsage{PromptTokens: 5, CompletionTokens: 7, TotalTokens: 12},
			}, nil
		},
	}
	app, _, stderr := appWithMockProvider(t, mp)
	app.chatPrompt = "hi"
	app.verbose = true

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr = %q, want a usage summary", stderr.String())
	}
	if !strings.Contains(stderr.String(), "12 total tokens") {
		t.Errorf("stderr = %q, want '12 total tokens'", stderr.String())
	}
}

// TestRunChatJSONEnvelopeWithSchema verifies --json still emits the response
// envelope when --schema is set (the schema constrains the model; --json wraps
// the envelope).
func TestRunChatJSONEnvelopeWithSchema(t *testing.T) {
	mp := &mockChatProvider{
		chat: func(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
			return &core.ChatResponse{ID: "r1", Model: req.Model, Output: `{"name":"John"}`}, nil
		},
	}
	app, stdout, _ := appWithMockProvider(t, mp)
	app.chatPrompt = "Extract: John"
	app.chatSchema = writeSchemaFile(t, `{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string"}},"required":["name"]}`)
	app.jsonOutput = true

	if err := app.runChat(nil, nil); err != nil {
		t.Fatalf("runChat() error = %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (got %q)", err, stdout.String())
	}
	if env["output"] != `{"name":"John"}` {
		t.Errorf("envelope output = %v, want the schema-constrained JSON string", env["output"])
	}
}
