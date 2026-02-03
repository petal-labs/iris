// Package commands implements the CLI command structure using Cobra.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/petal-labs/iris/cli/config"
)

var (
	// Global flags
	cfgFile    string
	provider   string
	model      string
	jsonOutput bool
	verbose    bool

	// Loaded configuration
	cfg *config.Config
)

// rootCmd is the base command for the CLI.
var rootCmd = &cobra.Command{
	Use:   "iris",
	Short: "Iris - AI agent development CLI",
	Long: `Iris is a command-line interface for AI agent development.

Use Iris to manage API keys, chat with models, and build agent workflows.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.iris/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "", "provider ID (openai, anthropic, ollama)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "model ID (e.g. gpt-4o)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "emit JSON output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable debug logging")
}

// initConfig reads in config file and sets defaults.
func initConfig() error {
	path := cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}

	var err error
	cfg, err = config.LoadConfig(path)
	if err != nil {
		return err
	}

	// Apply config defaults if flags not set
	if provider == "" && cfg.DefaultProvider != "" {
		provider = cfg.DefaultProvider
	}
	if model == "" && cfg.DefaultModel != "" {
		model = cfg.DefaultModel
	}

	return nil
}

// GetConfig returns the loaded configuration.
func GetConfig() *config.Config {
	return cfg
}

// GetProvider returns the effective provider ID (flag or config default).
func GetProvider() string {
	return provider
}

// GetModel returns the effective model ID (flag or config default).
func GetModel() string {
	return model
}

// IsJSONOutput returns true if JSON output is enabled.
func IsJSONOutput() bool {
	return jsonOutput
}

// IsVerbose returns true if verbose output is enabled.
func IsVerbose() bool {
	return verbose
}
