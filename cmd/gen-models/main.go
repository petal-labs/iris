// Command gen-models generates model constants from models.dev or local TOML files.
//
// Usage:
//
//	go run ./cmd/gen-models [flags]
//
// Flags:
//
//	-provider string
//	    Generate models for specific provider (default: all supported)
//	-local string
//	    Load models from local TOML directory instead of GitHub API
//	-output string
//	    Output directory (default: providers/<provider>)
//	-token string
//	    GitHub API token for higher rate limits
//	-dry-run
//	    Print generated code without writing files
//
// Examples:
//
//	# Generate from local TOML files (recommended)
//	go run ./cmd/gen-models -provider=openai -local=./internal/gen/models/data/openai
//
//	# Generate from models.dev GitHub (requires token for rate limits)
//	go run ./cmd/gen-models -provider=openai -token=$GITHUB_TOKEN
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petal-labs/iris/internal/gen/models"
)

var (
	provider = flag.String("provider", "", "Generate for specific provider (openai, anthropic, google)")
	local    = flag.String("local", "", "Load models from local TOML directory")
	output   = flag.String("output", "", "Output directory (default: providers/<provider>)")
	token    = flag.String("token", "", "GitHub API token for higher rate limits")
	dryRun   = flag.Bool("dry-run", false, "Print generated code without writing")
)

// Supported providers and their package names
var providers = map[string]string{
	"openai":    "openai",
	"anthropic": "anthropic",
	"google":    "gemini",
}

func main() {
	flag.Parse()

	// Determine which providers to generate
	targets := providers
	if *provider != "" {
		pkg, ok := providers[*provider]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", *provider)
			fmt.Fprintf(os.Stderr, "Supported providers: openai, anthropic, google\n")
			os.Exit(1)
		}
		targets = map[string]string{*provider: pkg}
	}

	// Generate for each provider
	for providerName, packageName := range targets {
		if err := generateProvider(providerName, packageName); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", providerName, err)
			os.Exit(1)
		}
	}
}

func generateProvider(providerName, packageName string) error {
	var modelData []models.ModelData
	var err error

	if *local != "" {
		// Load from local TOML files
		fmt.Printf("Loading models for %s from %s...\n", providerName, *local)
		modelData, err = models.LoadLocalModels(*local)
	} else {
		// Fetch from GitHub
		fmt.Printf("Fetching models for %s from models.dev...\n", providerName)
		client := models.NewClient()
		if *token != "" {
			client = models.NewClient(models.WithToken(*token))
		}
		modelData, err = client.FetchProviderModels(providerName)
	}

	if err != nil {
		return fmt.Errorf("load models: %w", err)
	}

	fmt.Printf("Found %d models for %s\n", len(modelData), providerName)

	if len(modelData) == 0 {
		fmt.Printf("No models found for %s, skipping\n", providerName)
		return nil
	}

	// Generate code
	gen := models.NewGenerator(providerName, packageName)
	code, err := gen.Generate(modelData)
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	if *dryRun {
		fmt.Printf("\n--- Generated code for %s ---\n", providerName)
		fmt.Println(string(code))
		return nil
	}

	// Determine output path
	outDir := *output
	if outDir == "" {
		outDir = filepath.Join("providers", packageName)
	}

	outPath := filepath.Join(outDir, "models_gen.go")

	// Write file
	if err := os.WriteFile(outPath, code, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("Wrote %s\n", outPath)
	return nil
}
