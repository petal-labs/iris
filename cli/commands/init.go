package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

var initProvider string

var initCmd = &cobra.Command{
	Use:   "init <project-name>",
	Short: "Initialize a new Iris project",
	Long: `Initialize a new Iris project with a standard directory structure.

Creates a project directory with:
  - main.go: A starter Go file using the Iris SDK
  - iris.yaml: Project configuration
  - tools/: Directory for custom tools

Example:
  iris init myagent
  iris init myagent --provider openai`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&initProvider, "provider", "openai", "Default provider for generated code")
}

func runInit(cmd *cobra.Command, args []string) error {
	projectPath := args[0]
	projectName := filepath.Base(projectPath)

	// Validate project name (just the base name, not full path)
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	// Check if directory already exists
	if _, err := os.Stat(projectPath); err == nil {
		return fmt.Errorf("directory %q already exists", projectPath)
	}

	// Create directory structure
	dirs := []string{
		projectPath,
		filepath.Join(projectPath, "tools"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create .gitkeep files in empty directories
	gitkeepDirs := []string{
		filepath.Join(projectPath, "tools"),
	}

	for _, dir := range gitkeepDirs {
		path := filepath.Join(dir, ".gitkeep")
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Generate main.go
	mainPath := filepath.Join(projectPath, "main.go")
	if err := generateFile(mainPath, mainGoTemplate, templateData{
		Provider: initProvider,
	}); err != nil {
		return fmt.Errorf("failed to create main.go: %w", err)
	}

	// Generate iris.yaml
	configPath := filepath.Join(projectPath, "iris.yaml")
	if err := generateFile(configPath, irisYamlTemplate, templateData{
		Provider: initProvider,
	}); err != nil {
		return fmt.Errorf("failed to create iris.yaml: %w", err)
	}

	// Print success message
	fmt.Printf("Created Iris project: %s\n\n", projectName)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectPath)
	fmt.Printf("  export %s=<your-key>\n", envVarForProvider(initProvider))
	fmt.Println("  go run main.go")

	return nil
}

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Check for invalid characters
	validName := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid project name %q: must start with a letter and contain only letters, numbers, underscores, and hyphens", name)
	}

	// Check for reserved names
	reserved := []string{".", "..", "iris"}
	for _, r := range reserved {
		if name == r {
			return fmt.Errorf("invalid project name %q: reserved name", name)
		}
	}

	return nil
}

type templateData struct {
	Provider string
}

var templateFuncs = template.FuncMap{
	"envVar":       envVarForProvider,
	"defaultModel": defaultModel,
}

func generateFile(path string, tmplContent string, data templateData) error {
	tmpl, err := template.New("file").Funcs(templateFuncs).Parse(tmplContent)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func envVarForProvider(provider string) string {
	return strings.ToUpper(provider) + "_API_KEY"
}

func defaultModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-sonnet-4-5"
	case "gemini":
		return "gemini-2.5-flash"
	case "xai":
		return "grok-4-1-fast-non-reasoning"
	case "zai":
		return "glm-4.7-flash"
	case "ollama":
		return "llama3.2"
	default:
		return "default"
	}
}

// Templates

var mainGoTemplate = `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/{{.Provider}}"
)

func main() {
	apiKey := os.Getenv("{{.Provider | envVar}}")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "{{.Provider | envVar}} not set")
		os.Exit(1)
	}

	p := {{.Provider}}.New(apiKey)
	c := core.NewClient(p)

	resp, err := c.Chat("{{.Provider | defaultModel}}").
		User("Hello, world!").
		GetResponse(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println(resp.Output)
}
`

var irisYamlTemplate = `# Iris project configuration
default_provider: {{.Provider}}
default_model: {{.Provider | defaultModel}}

# Provider configurations
# API keys should be set via 'iris keys set <provider>' or environment variables
providers:
  {{.Provider}}:
    api_key_env: {{.Provider | envVar}}
`
