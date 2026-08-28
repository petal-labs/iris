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

func (a *App) newInitCommand() *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "Initialize a new Iris project",
		Long: `Initialize a new Iris project with a standard directory structure.

Creates a project directory with:
  - main.go: A starter Go file using the Iris SDK
  - go.mod: A standalone Go module with the Iris dependency
  - tools/: Directory for custom tools

Example:
  iris init myapp
  iris init myapp --provider openai`,
		Args: cobra.ExactArgs(1),
		RunE: a.runInit,
	}

	initCmd.Flags().StringVar(&a.initProvider, "provider", "openai", "Default provider for generated code")
	return initCmd
}

func (a *App) runInit(cmd *cobra.Command, args []string) error {
	projectPath := args[0]
	projectName := filepath.Base(projectPath)

	if err := validateProjectName(projectName); err != nil {
		return err
	}
	if err := validateInitProvider(a.initProvider); err != nil {
		return err
	}
	if err := ensureProjectPathAvailable(projectPath); err != nil {
		return err
	}
	if err := createProject(projectPath, projectName, a.initProvider); err != nil {
		return err
	}

	printInitSuccess(a, projectName, projectPath)
	return nil
}

func ensureProjectPathAvailable(projectPath string) error {
	_, err := os.Stat(projectPath)
	if err == nil {
		return fmt.Errorf("directory %q already exists", projectPath)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect project path %q: %w", projectPath, err)
	}
	return nil
}

func createProject(projectPath, projectName, provider string) error {
	toolsPath := filepath.Join(projectPath, "tools")
	if err := os.MkdirAll(toolsPath, 0755); err != nil {
		return fmt.Errorf("failed to create project directories: %w", err)
	}
	if err := os.WriteFile(filepath.Join(toolsPath, ".gitkeep"), []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create tools/.gitkeep: %w", err)
	}

	data := templateData{
		ProjectName: projectName,
		Provider:    provider,
		GoVersion:   scaffoldGoVersion,
		SDKVersion:  scaffoldSDKVersion,
	}
	if err := generateFile(filepath.Join(projectPath, "main.go"), mainTemplateForProvider(provider), data); err != nil {
		return fmt.Errorf("failed to create main.go: %w", err)
	}
	if err := generateFile(filepath.Join(projectPath, "go.mod"), goModTemplate, data); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}
	return nil
}

func printInitSuccess(a *App, projectName, projectPath string) {
	fmt.Fprintf(a.stdout, "Created Iris project: %s\n\n", projectName)
	fmt.Fprintln(a.stdout, "Next steps:")
	fmt.Fprintf(a.stdout, "  cd %s\n", projectPath)
	if a.initProvider == "azurefoundry" {
		fmt.Fprintln(a.stdout, "  export AZURE_AI_ENDPOINT=<your-endpoint>")
	}
	if a.initProvider != "ollama" {
		fmt.Fprintf(a.stdout, "  export %s=<your-key>\n", envVarForProvider(a.initProvider))
	}
	if a.initProvider == "ollama" {
		fmt.Fprintln(a.stdout, "  # Ensure Ollama is running and llama3.2 is available")
	}
	fmt.Fprintln(a.stdout, "  go run .")
}

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Check for invalid characters.
	validName := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid project name %q: must start with a letter and contain only letters, numbers, underscores, and hyphens", name)
	}

	// Check for reserved names.
	reserved := []string{".", "..", "iris"}
	for _, r := range reserved {
		if name == r {
			return fmt.Errorf("invalid project name %q: reserved name", name)
		}
	}

	return nil
}

type templateData struct {
	ProjectName string
	Provider    string
	GoVersion   string
	SDKVersion  string
}

const (
	scaffoldGoVersion  = "1.25.0"
	scaffoldSDKVersion = "v0.17.0"
)

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
	if err := tmpl.Execute(f, data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func envVarForProvider(provider string) string {
	switch provider {
	case "huggingface":
		return "HF_TOKEN"
	case "voyageai":
		return "VOYAGE_API_KEY"
	case "azurefoundry":
		return "AZURE_AI_API_KEY"
	default:
		return strings.ToUpper(provider) + "_API_KEY"
	}
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
	case "perplexity":
		return "sonar"
	case "huggingface":
		return "meta-llama/Llama-3-8B-Instruct"
	case "azurefoundry":
		return "gpt-4o"
	case "voyageai":
		return "voyage-4-large"
	default:
		return ""
	}
}

func validateInitProvider(provider string) error {
	switch provider {
	case "openai", "anthropic", "gemini", "xai", "zai", "ollama",
		"huggingface", "perplexity", "voyageai", "azurefoundry":
		return nil
	default:
		return fmt.Errorf("unsupported provider %q for project scaffold", provider)
	}
}

func mainTemplateForProvider(provider string) string {
	switch provider {
	case "ollama":
		return ollamaMainGoTemplate
	case "voyageai":
		return voyageAIMainGoTemplate
	case "azurefoundry":
		return azureFoundryMainGoTemplate
	default:
		return mainGoTemplate
	}
}

// Templates.

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

var ollamaMainGoTemplate = `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/ollama"
)

func main() {
	p := ollama.NewLocal()
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

var voyageAIMainGoTemplate = `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/voyageai"
)

func main() {
	apiKey := os.Getenv("{{.Provider | envVar}}")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "{{.Provider | envVar}} not set")
		os.Exit(1)
	}

	p := voyageai.New(apiKey)
	resp, err := p.CreateEmbeddings(context.Background(), &core.EmbeddingRequest{
		Model: "{{.Provider | defaultModel}}",
		Input: []core.EmbeddingInput{
			{Text: "Hello, world!"},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d embedding(s)\n", len(resp.Vectors))
}
`

var azureFoundryMainGoTemplate = `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/azurefoundry"
)

func main() {
	p, err := azurefoundry.NewFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
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

var goModTemplate = `module {{.ProjectName}}

go {{.GoVersion}}

require github.com/petal-labs/iris {{.SDKVersion}}
`
