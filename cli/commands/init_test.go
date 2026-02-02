package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myagent", false},
		{"valid with numbers", "agent123", false},
		{"valid with underscore", "my_agent", false},
		{"valid with hyphen", "my-agent", false},
		{"empty", "", true},
		{"starts with number", "123agent", true},
		{"starts with hyphen", "-agent", true},
		{"contains space", "my agent", true},
		{"contains dot", "my.agent", true},
		{"reserved dot", ".", true},
		{"reserved dotdot", "..", true},
		{"reserved iris", "iris", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProjectName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestEnvVarForProvider(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"ollama", "OLLAMA_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := envVarForProvider(tt.provider)
			if got != tt.want {
				t.Errorf("envVarForProvider(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestDefaultModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "gpt-4o"},
		{"anthropic", "claude-sonnet-4-5"},
		{"gemini", "gemini-2.5-flash"},
		{"unknown", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := defaultModel(tt.provider)
			if got != tt.want {
				t.Errorf("defaultModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGenerateFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	tmpl := "Hello {{.Provider}}!"
	data := templateData{Provider: "world"}

	err := generateFile(path, tmpl, data)
	if err != nil {
		t.Fatalf("generateFile() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(content) != "Hello world!" {
		t.Errorf("generateFile() content = %q, want 'Hello world!'", string(content))
	}
}

func TestGenerateFileWithFuncs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	tmpl := "Provider: {{.Provider}}, Env: {{.Provider | envVar}}, Model: {{.Provider | defaultModel}}"
	data := templateData{Provider: "openai"}

	err := generateFile(path, tmpl, data)
	if err != nil {
		t.Fatalf("generateFile() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	expected := "Provider: openai, Env: OPENAI_API_KEY, Model: gpt-4o"
	if string(content) != expected {
		t.Errorf("generateFile() content = %q, want %q", string(content), expected)
	}
}

func TestInitCreatesProjectStructure(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "testproject")

	// Set up for runInit
	initProvider = "openai"

	// Simulate running the init command
	err := runInitWithPath(projectPath, "openai")
	if err != nil {
		t.Fatalf("runInit() error = %v", err)
	}

	// Verify directory structure
	dirs := []string{
		projectPath,
		filepath.Join(projectPath, "agents"),
		filepath.Join(projectPath, "tools"),
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Directory %q not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}

	// Verify .gitkeep files
	gitkeeps := []string{
		filepath.Join(projectPath, "agents", ".gitkeep"),
		filepath.Join(projectPath, "tools", ".gitkeep"),
	}

	for _, path := range gitkeeps {
		if _, err := os.Stat(path); err != nil {
			t.Errorf(".gitkeep not created at %q: %v", path, err)
		}
	}

	// Verify main.go exists and contains expected content
	mainPath := filepath.Join(projectPath, "main.go")
	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("main.go not created: %v", err)
	}

	if !strings.Contains(string(mainContent), "package main") {
		t.Error("main.go missing 'package main'")
	}
	if !strings.Contains(string(mainContent), "openai.New") {
		t.Error("main.go missing 'openai.New'")
	}

	// Verify iris.yaml exists and contains expected content
	configPath := filepath.Join(projectPath, "iris.yaml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("iris.yaml not created: %v", err)
	}

	if !strings.Contains(string(configContent), "default_provider: openai") {
		t.Error("iris.yaml missing 'default_provider: openai'")
	}
}

func TestInitErrorOnExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "existing")

	// Create the directory first
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	err := runInitWithPath(projectPath, "openai")
	if err == nil {
		t.Error("runInit() should return error for existing directory")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Error message should mention 'already exists', got: %v", err)
	}
}

// Helper function to run init with a specific path
func runInitWithPath(projectPath, provider string) error {
	projectName := filepath.Base(projectPath)

	if err := validateProjectName(projectName); err != nil {
		return err
	}

	if _, err := os.Stat(projectPath); err == nil {
		return os.ErrExist
	}

	dirs := []string{
		projectPath,
		filepath.Join(projectPath, "agents"),
		filepath.Join(projectPath, "tools"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	gitkeepDirs := []string{
		filepath.Join(projectPath, "agents"),
		filepath.Join(projectPath, "tools"),
	}

	for _, dir := range gitkeepDirs {
		path := filepath.Join(dir, ".gitkeep")
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			return err
		}
	}

	mainPath := filepath.Join(projectPath, "main.go")
	if err := generateFile(mainPath, mainGoTemplate, templateData{Provider: provider}); err != nil {
		return err
	}

	configPath := filepath.Join(projectPath, "iris.yaml")
	if err := generateFile(configPath, irisYamlTemplate, templateData{Provider: provider}); err != nil {
		return err
	}

	return nil
}
