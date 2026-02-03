//go:build integration

// Package integration provides integration tests for the Iris SDK.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petal-labs/iris/tools"
)

// isCI returns true if running in a CI environment.
// It checks for common CI environment variables.
func isCI() bool {
	// GitHub Actions, GitLab CI, CircleCI, Travis, Jenkins, etc.
	ciVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL"}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// skipInCI skips the test when running in CI environments.
// Use this for tests that are flaky or unsupported in CI (e.g., HuggingFace).
func skipInCI(t *testing.T, reason string) {
	t.Helper()
	if isCI() {
		t.Skipf("skipping in CI: %s", reason)
	}
}

// skipOrFailOnMissingKey handles missing API keys.
// In CI environments, it fails loudly unless IRIS_SKIP_INTEGRATION is set.
// In local development, it skips the test gracefully.
func skipOrFailOnMissingKey(t *testing.T, keyName string) {
	t.Helper()
	if isCI() && os.Getenv("IRIS_SKIP_INTEGRATION") == "" {
		t.Fatalf("%s not set (CI environment detected; set IRIS_SKIP_INTEGRATION=1 to skip)", keyName)
	}
	t.Skipf("%s not set", keyName)
}

// skipIfNoAPIKey skips the test if OPENAI_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("OPENAI_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "OPENAI_API_KEY")
	}
}

// getAPIKey returns the OpenAI API key from environment.
func getAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Fatal("OPENAI_API_KEY not set")
	}
	return key
}

// skipIfNoAnthropicKey skips the test if ANTHROPIC_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoAnthropicKey(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "ANTHROPIC_API_KEY")
	}
}

// getAnthropicKey returns the Anthropic API key from environment.
func getAnthropicKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Fatal("ANTHROPIC_API_KEY not set")
	}
	return key
}

// skipIfNoGeminiKey skips the test if GEMINI_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoGeminiKey(t *testing.T) {
	t.Helper()
	if os.Getenv("GEMINI_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "GEMINI_API_KEY")
	}
}

// getGeminiKey returns the Gemini API key from environment.
func getGeminiKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Fatal("GEMINI_API_KEY not set")
	}
	return key
}

// skipIfNoZaiKey skips the test if ZAI_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoZaiKey(t *testing.T) {
	t.Helper()
	if os.Getenv("ZAI_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "ZAI_API_KEY")
	}
}

// getZaiKey returns the Z.ai API key from environment.
func getZaiKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ZAI_API_KEY")
	if key == "" {
		t.Fatal("ZAI_API_KEY not set")
	}
	return key
}

// skipIfNoXaiKey skips the test if XAI_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoXaiKey(t *testing.T) {
	t.Helper()
	if os.Getenv("XAI_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "XAI_API_KEY")
	}
}

// getXaiKey returns the xAI API key from environment.
func getXaiKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("XAI_API_KEY")
	if key == "" {
		t.Fatal("XAI_API_KEY not set")
	}
	return key
}

// skipIfNoVoyageAPIKey skips the test if VOYAGEAI_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoVoyageAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("VOYAGEAI_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "VOYAGEAI_API_KEY")
	}
}

// getVoyageAPIKey returns the Voyage AI API key from environment.
func getVoyageAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("VOYAGEAI_API_KEY")
	if key == "" {
		t.Fatal("VOYAGEAI_API_KEY not set")
	}
	return key
}

// skipIfNoPerplexityKey skips the test if PERPLEXITY_API_KEY is not set.
// In CI, it fails unless IRIS_SKIP_INTEGRATION is set.
func skipIfNoPerplexityKey(t *testing.T) {
	t.Helper()
	if os.Getenv("PERPLEXITY_API_KEY") == "" {
		skipOrFailOnMissingKey(t, "PERPLEXITY_API_KEY")
	}
}

// getPerplexityKey returns the Perplexity API key from environment.
func getPerplexityKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("PERPLEXITY_API_KEY")
	if key == "" {
		t.Fatal("PERPLEXITY_API_KEY not set")
	}
	return key
}

// cliResult holds the result of running a CLI command.
type cliResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runCLI executes the iris CLI with the given arguments.
// It uses the pre-built binary from TestMain for efficiency.
func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()

	binaryPath := getCliBinary()
	if binaryPath == "" {
		t.Fatal("CLI binary not built - TestMain may not have run")
	}

	// Run the CLI
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run CLI: %v", err)
		}
	}

	return cliResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// runCLIWithStdin executes the iris CLI with stdin input.
// It uses the pre-built binary from TestMain for efficiency.
func runCLIWithStdin(t *testing.T, stdin string, args ...string) cliResult {
	t.Helper()

	binaryPath := getCliBinary()
	if binaryPath == "" {
		t.Fatal("CLI binary not built - TestMain may not have run")
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run CLI: %v", err)
		}
	}

	return cliResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// findTestDataDir locates the testdata directory.
func findTestDataDir(t *testing.T) string {
	t.Helper()

	candidates := []string{
		"../testdata",
		"testdata",
		"tests/testdata",
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(candidate)
			return abs
		}
	}

	t.Fatal("Could not find testdata directory")
	return ""
}

// testTool implements tools.Tool for testing.
type testTool struct {
	name        string
	description string
	schema      json.RawMessage
}

func (t *testTool) Name() string        { return t.name }
func (t *testTool) Description() string { return t.description }
func (t *testTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{JSONSchema: t.schema}
}
func (t *testTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return map[string]string{"result": "test result"}, nil
}

// createTestTool creates a simple tool for testing.
func createTestTool() tools.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "The city and state, e.g. San Francisco, CA",
			},
		},
		"required": []string{"location"},
	}

	schemaJSON, _ := json.Marshal(schema)

	return &testTool{
		name:        "get_weather",
		description: "Get the current weather in a given location",
		schema:      schemaJSON,
	}
}

// tempDir creates a temporary directory for testing.
func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// skipIfNoOllama skips the test if local Ollama is not available.
// This checks by making a simple request to the Ollama API.
func skipIfNoOllama(t *testing.T) {
	t.Helper()
	// Check if OLLAMA_HOST is set, otherwise use default
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", host+"/api/tags", nil)
	if err != nil {
		t.Skipf("Ollama not available: %v", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("Ollama not available at %s: %v", host, err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Ollama returned status %d", resp.StatusCode)
	}
}

// getOllamaHost returns the Ollama host URL from environment or default.
func getOllamaHost(t *testing.T) string {
	t.Helper()
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	return host
}
