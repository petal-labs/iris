package commands

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func TestGenerateFileRejectsInvalidTemplate(t *testing.T) {
	err := generateFile(filepath.Join(t.TempDir(), "invalid.txt"), "{{", templateData{})
	if err == nil {
		t.Fatal("generateFile() should reject an invalid template")
	}
}

func TestCreateProjectRejectsFilePath(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.WriteFile(projectPath, []byte("occupied"), 0600); err != nil {
		t.Fatalf("write project-path fixture: %v", err)
	}
	err := createProject(projectPath, "project", "openai")
	if err == nil || !strings.Contains(err.Error(), "failed to create project directories") {
		t.Fatalf("createProject() error = %v, want project-directory error", err)
	}
}

func TestCreateProjectReportsMainFileError(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(projectPath, "main.go"), 0755); err != nil {
		t.Fatalf("create main.go directory fixture: %v", err)
	}
	err := createProject(projectPath, "project", "openai")
	if err == nil || !strings.Contains(err.Error(), "failed to create main.go") {
		t.Fatalf("createProject() error = %v, want main.go creation error", err)
	}
}

func TestResolveScaffoldSDKVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkedVersion string
		moduleVersion string
		want          string
	}{
		{name: "release build", linkedVersion: "v0.18.0", moduleVersion: "(devel)", want: "v0.18.0"},
		{name: "tagged go install", linkedVersion: "dev", moduleVersion: "v0.18.0", want: "v0.18.0"},
		{name: "pseudo-version install", linkedVersion: "dev", moduleVersion: "v0.0.0-20260828120000-abcdef123456", want: "v0.0.0-20260828120000-abcdef123456"},
		{name: "module version after development linker version", linkedVersion: "v0.17.0-74-g243be7c", moduleVersion: "v0.18.0", want: "v0.18.0"},
		{name: "make development build", linkedVersion: "v0.17.0-74-g243be7c", moduleVersion: "(devel)", want: scaffoldSDKVersionFallback},
		{name: "dirty tagged build", linkedVersion: "v0.18.0-dirty", moduleVersion: "(devel)", want: scaffoldSDKVersionFallback},
		{name: "go development build", linkedVersion: "dev", moduleVersion: "(devel)", want: scaffoldSDKVersionFallback},
		{name: "missing build version", linkedVersion: "", moduleVersion: "", want: scaffoldSDKVersionFallback},
		{name: "malformed linked version", linkedVersion: "release", moduleVersion: "(devel)", want: scaffoldSDKVersionFallback},
		{name: "leading-zero prerelease", linkedVersion: "v1.2.3-01", moduleVersion: "(devel)", want: scaffoldSDKVersionFallback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveScaffoldSDKVersion(tt.linkedVersion, tt.moduleVersion)
			if got != tt.want {
				t.Errorf("resolveScaffoldSDKVersion(%q, %q) = %q, want %q", tt.linkedVersion, tt.moduleVersion, got, tt.want)
			}
		})
	}
}

func TestScaffoldSDKVersionFallbackMatchesLatestChangelogRelease(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("open CHANGELOG.md: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	releaseHeading := regexp.MustCompile(`^## \[([^]]+)\] - [0-9]{4}-[0-9]{2}-[0-9]{2}\r?$`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "## [") || line == "## [Unreleased]" {
			continue
		}
		match := releaseHeading.FindStringSubmatch(line)
		if len(match) != 2 {
			t.Fatalf("latest CHANGELOG release heading %q is malformed", line)
		}
		want := "v" + match[1]
		if scaffoldSDKVersionFallback != want {
			t.Errorf("scaffoldSDKVersionFallback = %q, want latest CHANGELOG release %q", scaffoldSDKVersionFallback, want)
		}
		return
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan CHANGELOG.md: %v", err)
	}
	t.Fatal("CHANGELOG.md contains no dated release heading")
}

func TestInitUsesBuildVersionForSDKDependency(t *testing.T) {
	originalVersion := Version
	Version = "v0.18.0"
	t.Cleanup(func() { Version = originalVersion })

	projectPath := filepath.Join(t.TempDir(), "scaffold")
	if err := runInitWithPath(projectPath, "openai"); err != nil {
		t.Fatalf("runInitWithPath() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(projectPath, "go.mod"))
	if err != nil {
		t.Fatalf("read generated go.mod: %v", err)
	}
	want := "require github.com/petal-labs/iris v0.18.0"
	if !strings.Contains(string(content), want) {
		t.Errorf("generated go.mod missing %q:\n%s", want, content)
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
