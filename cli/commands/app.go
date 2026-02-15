package commands

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/petal-labs/iris/cli/config"
	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

// ConfigLoader loads CLI config from a path.
type ConfigLoader func(path string) (*config.Config, error)

// ProviderFactory creates a provider using CLI config context.
type ProviderFactory func(providerID, apiKey string, cfg *config.Config) (core.Provider, error)

// KeystoreFactory creates a keystore instance.
type KeystoreFactory func() (keystore.Keystore, error)

// AppOption customizes App dependencies.
type AppOption func(*App)

// App holds CLI state and runtime dependencies.
type App struct {
	root *cobra.Command

	loadConfig      ConfigLoader
	createProvider  ProviderFactory
	newKeystore     KeystoreFactory
	stdin           io.Reader
	stdout          io.Writer
	stderr          io.Writer
	cfgFile         string
	provider        string
	model           string
	jsonOutput      bool
	verbose         bool
	cfg             *config.Config
	chatPrompt      string
	chatSystem      string
	chatTemperature float32
	chatMaxTokens   int
	chatStream      bool
	initProvider    string
}

// WithConfigLoader injects a config loader dependency.
func WithConfigLoader(loader ConfigLoader) AppOption {
	return func(a *App) {
		if loader != nil {
			a.loadConfig = loader
		}
	}
}

// WithProviderFactory injects a provider factory dependency.
func WithProviderFactory(factory ProviderFactory) AppOption {
	return func(a *App) {
		if factory != nil {
			a.createProvider = factory
		}
	}
}

// WithKeystoreFactory injects a keystore factory dependency.
func WithKeystoreFactory(factory KeystoreFactory) AppOption {
	return func(a *App) {
		if factory != nil {
			a.newKeystore = factory
		}
	}
}

// WithIO injects process I/O streams.
func WithIO(stdin io.Reader, stdout, stderr io.Writer) AppOption {
	return func(a *App) {
		if stdin != nil {
			a.stdin = stdin
		}
		if stdout != nil {
			a.stdout = stdout
		}
		if stderr != nil {
			a.stderr = stderr
		}
	}
}

// NewApp creates a new CLI app with default dependencies.
func NewApp(opts ...AppOption) *App {
	a := &App{
		loadConfig:     config.LoadConfig,
		createProvider: defaultProviderFactory(),
		newKeystore:    keystore.NewKeystore,
		stdin:          os.Stdin,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		initProvider:   "openai",
	}

	for _, opt := range opts {
		opt(a)
	}

	a.root = a.newRootCommand()
	return a
}

func (a *App) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "iris",
		Short: "Iris - Go SDK and CLI for LLM providers",
		Long: `Iris is a command-line interface for working with LLM providers.

Use Iris to manage API keys, chat with models, and scaffold projects.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.initConfig()
		},
		SilenceUsage: true,
	}

	// Global flags available to all commands.
	root.PersistentFlags().StringVar(&a.cfgFile, "config", "", "config file (default is ~/.iris/config.yaml)")
	root.PersistentFlags().StringVar(&a.provider, "provider", "", "provider ID (openai, anthropic, ollama)")
	root.PersistentFlags().StringVar(&a.model, "model", "", "model ID (e.g. gpt-4o)")
	root.PersistentFlags().BoolVar(&a.jsonOutput, "json", false, "emit JSON output")
	root.PersistentFlags().BoolVar(&a.verbose, "verbose", false, "enable debug logging")

	root.AddCommand(a.newChatCommand())
	root.AddCommand(a.newKeysCommand())
	root.AddCommand(a.newInitCommand())
	root.AddCommand(a.newVersionCommand())

	return root
}

// Execute runs the root command.
func (a *App) Execute() error {
	return a.root.Execute()
}

func (a *App) initConfig() error {
	path := a.cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}

	cfg, err := a.loadConfig(path)
	if err != nil {
		return err
	}
	a.cfg = cfg

	// Apply config defaults if flags not set.
	if a.provider == "" && cfg.DefaultProvider != "" {
		a.provider = cfg.DefaultProvider
	}
	if a.model == "" && cfg.DefaultModel != "" {
		a.model = cfg.DefaultModel
	}

	return nil
}

var defaultApp = NewApp()

// Execute runs the default app root command.
func Execute() error {
	return defaultApp.Execute()
}
