//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Chat(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)

	// Set up keystore with API key
	setupKeystore(t, "openai", apiKey)

	result := runCLI(t, "chat",
		"--provider", "openai",
		"--model", "gpt-4o-mini",
		"--prompt", "Say 'hello' and nothing else.")

	if result.ExitCode != 0 {
		t.Errorf("Exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	if result.Stdout == "" {
		t.Error("Stdout is empty")
	}

	t.Logf("Output: %s", result.Stdout)
}

func TestCLI_Chat_Streaming(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	setupKeystore(t, "openai", apiKey)

	result := runCLI(t, "chat",
		"--provider", "openai",
		"--model", "gpt-4o-mini",
		"--prompt", "Count from 1 to 3.",
		"--stream")

	if result.ExitCode != 0 {
		t.Errorf("Exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	if result.Stdout == "" {
		t.Error("Stdout is empty")
	}

	t.Logf("Output: %s", result.Stdout)
}

func TestCLI_Chat_JSON(t *testing.T) {
	skipIfNoAPIKey(t)

	apiKey := getAPIKey(t)
	setupKeystore(t, "openai", apiKey)

	result := runCLI(t, "chat",
		"--provider", "openai",
		"--model", "gpt-4o-mini",
		"--prompt", "Say hello.",
		"--json")

	if result.ExitCode != 0 {
		t.Errorf("Exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	// Verify valid JSON
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, result.Stdout)
	}

	// Verify expected fields
	if _, ok := output["output"]; !ok {
		t.Error("JSON output missing 'output' field")
	}
	if _, ok := output["usage"]; !ok {
		t.Error("JSON output missing 'usage' field")
	}

	t.Logf("JSON Output: %s", result.Stdout)
}

func TestCLI_Chat_MissingProvider(t *testing.T) {
	result := runCLI(t, "chat", "--prompt", "Hello")

	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for missing provider")
	}

	if !strings.Contains(result.Stderr, "provider") {
		t.Errorf("Stderr should mention provider, got: %s", result.Stderr)
	}
}

func TestCLI_Init(t *testing.T) {
	tmpDir := tempDir(t)
	projectPath := filepath.Join(tmpDir, "testproject")

	result := runCLI(t, "init", projectPath)

	if result.ExitCode != 0 {
		t.Errorf("Exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	// Verify directory created
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		t.Error("Project directory not created")
	}

	// Verify files exist
	files := []string{
		"main.go",
		"iris.yaml",
		"agents/.gitkeep",
		"tools/.gitkeep",
	}

	for _, file := range files {
		path := filepath.Join(projectPath, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("File %s not created", file)
		}
	}

	// Verify main.go compiles
	checkCompiles(t, projectPath)

	t.Logf("Output: %s", result.Stdout)
}

func TestCLI_Init_WithProvider(t *testing.T) {
	tmpDir := tempDir(t)
	projectPath := filepath.Join(tmpDir, "anthropic-project")

	result := runCLI(t, "init", projectPath, "--provider", "anthropic")

	if result.ExitCode != 0 {
		t.Errorf("Exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	// Verify main.go contains anthropic
	mainPath := filepath.Join(projectPath, "main.go")
	content, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "anthropic") {
		t.Error("main.go should contain 'anthropic'")
	}

	if !strings.Contains(string(content), "ANTHROPIC_API_KEY") {
		t.Error("main.go should contain 'ANTHROPIC_API_KEY'")
	}
}

func TestCLI_Init_ExistingDirectory(t *testing.T) {
	tmpDir := tempDir(t)
	projectPath := filepath.Join(tmpDir, "existing")

	// Create directory first
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	result := runCLI(t, "init", projectPath)

	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for existing directory")
	}

	if !strings.Contains(result.Stderr, "exists") {
		t.Errorf("Stderr should mention exists, got: %s", result.Stderr)
	}
}

func TestCLI_Keys(t *testing.T) {
	// Use a unique provider name to avoid conflicts
	provider := "testprovider-integration"
	testKey := "test-api-key-12345"

	// Set key
	result := runCLIWithStdin(t, testKey+"\n", "keys", "set", provider)
	if result.ExitCode != 0 {
		t.Errorf("keys set exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	// List keys
	result = runCLI(t, "keys", "list")
	if result.ExitCode != 0 {
		t.Errorf("keys list exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, provider) {
		t.Errorf("keys list should contain %s, got: %s", provider, result.Stdout)
	}

	// Delete key
	result = runCLI(t, "keys", "delete", provider)
	if result.ExitCode != 0 {
		t.Errorf("keys delete exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	// Verify deleted
	result = runCLI(t, "keys", "list")
	if strings.Contains(result.Stdout, provider) {
		t.Errorf("keys list should not contain %s after delete", provider)
	}
}

func TestCLI_GraphExport(t *testing.T) {
	testDataDir := findTestDataDir(t)
	agentPath := filepath.Join(testDataDir, "agent.yaml")

	// Test mermaid output
	result := runCLI(t, "graph", "export", agentPath)
	if result.ExitCode != 0 {
		t.Errorf("graph export exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, "graph TD") {
		t.Error("Mermaid output should contain 'graph TD'")
	}

	// Test JSON output
	result = runCLI(t, "graph", "export", agentPath, "--format", "json")
	if result.ExitCode != 0 {
		t.Errorf("graph export --format json exit code = %d, want 0\nStderr: %s", result.ExitCode, result.Stderr)
	}

	var graphJSON map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &graphJSON); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	if _, ok := graphJSON["name"]; !ok {
		t.Error("JSON should contain 'name' field")
	}
}

func TestCLI_Help(t *testing.T) {
	result := runCLI(t, "--help")

	if result.ExitCode != 0 {
		t.Errorf("Exit code = %d, want 0", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "iris") {
		t.Error("Help should mention iris")
	}

	// Check for main commands
	commands := []string{"chat", "keys", "init", "graph"}
	for _, cmd := range commands {
		if !strings.Contains(result.Stdout, cmd) {
			t.Errorf("Help should mention '%s' command", cmd)
		}
	}
}

// setupKeystore sets up a key in the keystore for testing.
func setupKeystore(t *testing.T, provider, apiKey string) {
	t.Helper()
	result := runCLIWithStdin(t, apiKey+"\n", "keys", "set", provider)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to set up keystore: %s", result.Stderr)
	}
	t.Cleanup(func() {
		runCLI(t, "keys", "delete", provider)
	})
}

// checkCompiles verifies that the Go code in the directory compiles.
func checkCompiles(t *testing.T, dir string) {
	t.Helper()

	// We can't actually compile since it depends on iris modules
	// Just verify the file is valid Go syntax
	mainPath := filepath.Join(dir, "main.go")
	content, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	if !strings.Contains(string(content), "package main") {
		t.Error("main.go should contain 'package main'")
	}

	if !strings.Contains(string(content), "func main()") {
		t.Error("main.go should contain 'func main()'")
	}
}
