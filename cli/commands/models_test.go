package commands

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

func newModelsApp(t *testing.T, mp *mockChatProvider) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
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
	return app, &stdout, &stderr
}

func TestRunModelsTable(t *testing.T) {
	mp := &mockChatProvider{
		models: []core.ModelInfo{
			{ID: "llama3.2", DisplayName: "Llama 3.2", Capabilities: []core.Feature{core.FeatureChat, core.FeatureChatStreaming}},
			{ID: "mistral", DisplayName: "Mistral 7B", Capabilities: []core.Feature{core.FeatureChat}},
		},
	}
	app, stdout, _ := newModelsApp(t, mp)

	if err := app.runModels(nil, nil); err != nil {
		t.Fatalf("runModels() error = %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"llama3.2", "Llama 3.2", "mistral", "Mistral 7B", "chat", "chat_streaming"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
}

func TestRunModelsJSON(t *testing.T) {
	mp := &mockChatProvider{
		models: []core.ModelInfo{
			{ID: "gpt-4o", DisplayName: "GPT-4o", Capabilities: []core.Feature{core.FeatureChat, core.FeatureStructuredOutput}},
		},
	}
	app, stdout, _ := newModelsApp(t, mp)
	app.jsonOutput = true

	if err := app.runModels(nil, nil); err != nil {
		t.Fatalf("runModels() error = %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not valid JSON array: %v (got %q)", err, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0]["id"] != "gpt-4o" {
		t.Errorf("id = %v, want gpt-4o", got[0]["id"])
	}
	caps, _ := got[0]["capabilities"].([]any)
	if len(caps) != 2 {
		t.Errorf("capabilities = %v, want 2 entries", caps)
	}
}

func TestRunModelsEmptyCatalog(t *testing.T) {
	mp := &mockChatProvider{models: nil}
	app, stdout, _ := newModelsApp(t, mp)

	if err := app.runModels(nil, nil); err != nil {
		t.Fatalf("runModels() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "No models") {
		t.Errorf("stdout = %q, want 'No models' notice", stdout.String())
	}
}

func TestRunModelsNoProvider(t *testing.T) {
	mp := &mockChatProvider{}
	app, _, _ := newModelsApp(t, mp)
	app.provider = ""

	err := app.runModels(nil, nil)
	if err == nil {
		t.Fatal("runModels() should error without a provider")
	}
	exitErr, ok := err.(*exitError)
	if !ok || exitErr.ExitCode() != ExitValidation {
		t.Errorf("exit code = %v, want ExitValidation", err)
	}
}

// TestRunModelsDoesNotRequireAPIKey verifies the static catalog is reachable
// even when no key is stored for a normally-keyed provider, via the injected
// factory.
func TestRunModelsDoesNotRequireAPIKey(t *testing.T) {
	mp := &mockChatProvider{
		id:     "openai",
		models: []core.ModelInfo{{ID: "gpt-4o", DisplayName: "GPT-4o"}},
	}
	var stdout bytes.Buffer
	app := NewApp(
		WithProviderFactory(func(string, string, *config.Config) (core.Provider, error) { return mp, nil }),
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(filepath.Join(t.TempDir(), "keys.enc"))
		}),
		WithIO(strings.NewReader(""), &stdout, &bytes.Buffer{}),
	)
	app.cfg = &config.Config{}
	app.provider = "openai" // normally keyed, but no key stored

	if err := app.runModels(nil, nil); err != nil {
		t.Fatalf("runModels() should not require an API key for the static catalog; got %v", err)
	}
	if !strings.Contains(stdout.String(), "gpt-4o") {
		t.Errorf("stdout = %q, want the model id", stdout.String())
	}
}
