package commands

import (
	"bytes"
	"os"
	"os/exec"
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
		{"valid simple", "myapp", false},
		{"valid with numbers", "app123", false},
		{"valid with underscore", "my_app", false},
		{"valid with hyphen", "my-app", false},
		{"empty", "", true},
		{"starts with number", "123app", true},
		{"starts with hyphen", "-app", true},
		{"contains space", "my app", true},
		{"contains dot", "my.app", true},
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
		{"huggingface", "HF_TOKEN"},
		{"voyageai", "VOYAGE_API_KEY"},
		{"azurefoundry", "AZURE_AI_API_KEY"},
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
		{"xai", "grok-4-1-fast-non-reasoning"},
		{"zai", "glm-4.7-flash"},
		{"ollama", "llama3.2"},
		{"perplexity", "sonar"},
		{"huggingface", "meta-llama/Llama-3-8B-Instruct"},
		{"azurefoundry", "gpt-4o"},
		{"voyageai", "voyage-4-large"},
		{"unknown", ""},
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
	data := templateData{ProjectName: "testproject", Provider: "world"}

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
	data := templateData{ProjectName: "testproject", Provider: "openai"}

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

	// Simulate running the init command
	err := runInitWithPath(projectPath, "openai")
	if err != nil {
		t.Fatalf("runInit() error = %v", err)
	}

	// Verify directory structure
	dirs := []string{
		projectPath,
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

	// Verify go.mod exists and declares a runnable module with Iris.
	goModPath := filepath.Join(projectPath, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("go.mod not created: %v", err)
	}
	if !strings.Contains(string(goModContent), "module testproject") {
		t.Error("go.mod missing generated module name")
	}
	if !strings.Contains(string(goModContent), "github.com/petal-labs/iris") {
		t.Error("go.mod missing Iris dependency")
	}

	if _, err := os.Stat(filepath.Join(projectPath, "iris.yaml")); !os.IsNotExist(err) {
		t.Errorf("unused iris.yaml should not be generated, stat error = %v", err)
	}
}

func TestInitRejectsUnsupportedProvider(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "badprovider")

	err := runInitWithPath(projectPath, "not-a-provider")
	if err == nil {
		t.Fatal("runInit() should reject an unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("error = %q, want unsupported provider guidance", err)
	}
	if _, statErr := os.Stat(projectPath); !os.IsNotExist(statErr) {
		t.Errorf("project directory should not be created, stat error = %v", statErr)
	}
}

func TestGeneratedProjectsCompile(t *testing.T) {
	providers := []string{
		"openai", "anthropic", "gemini", "xai", "zai",
		"ollama", "huggingface", "perplexity", "voyageai", "azurefoundry",
	}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			projectPath := filepath.Join(t.TempDir(), "scaffold")
			if err := runInitWithPath(projectPath, provider); err != nil {
				t.Fatalf("runInitWithPath(%q) error = %v", provider, err)
			}
			compileGeneratedProject(t, projectPath)
		})
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

func runInitWithPath(projectPath, provider string) error {
	app := NewApp(WithIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}))
	app.initProvider = provider
	return app.runInit(nil, []string{projectPath})
}

func compileGeneratedProject(t *testing.T, projectPath string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	runGoCommand(t, projectPath, "mod", "edit", "-replace=github.com/petal-labs/iris="+repoRoot)
	runGoCommand(t, projectPath, "test", "./...")
}

func runGoCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
