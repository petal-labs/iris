package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/petal-labs/iris/cli/keystore"
)

func (a *App) newKeysCommand() *cobra.Command {
	keysCmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys",
		Long:  `Manage API keys for various providers. Keys are stored securely using encryption.`,
	}

	keysCmd.AddCommand(&cobra.Command{
		Use:   "set <provider>",
		Short: "Set API key for a provider",
		Long:  `Set the API key for a provider. The key will be prompted without echo for security.`,
		Args:  cobra.ExactArgs(1),
		RunE:  a.runKeysSet,
	})
	keysCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List stored API keys",
		Long:  `List all stored API keys. Only provider names are shown, never key values.`,
		RunE:  a.runKeysList,
	})
	keysCmd.AddCommand(&cobra.Command{
		Use:   "delete <provider>",
		Short: "Delete API key for a provider",
		Args:  cobra.ExactArgs(1),
		RunE:  a.runKeysDelete,
	})

	return keysCmd
}

func (a *App) runKeysSet(cmd *cobra.Command, args []string) error {
	provider := args[0]

	// Prompt for API key.
	fmt.Fprintf(a.stdout, "Enter API key for %s: ", provider)

	apiKey, err := readSecretInput(a.stdin, a.stdout)
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	ks, err := a.newKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}
	if err := ks.Set(provider, apiKey); err != nil {
		return fmt.Errorf("failed to store key: %w", err)
	}

	fmt.Fprintf(a.stdout, "API key for %s stored successfully.\n", provider)
	return nil
}

func (a *App) runKeysList(cmd *cobra.Command, args []string) error {
	ks, err := a.newKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	names, err := ks.List()
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if len(names) == 0 {
		fmt.Fprintln(a.stdout, "No API keys stored.")
		return nil
	}

	fmt.Fprintln(a.stdout, "Stored keys:")
	for _, name := range names {
		fmt.Fprintf(a.stdout, "  - %s\n", name)
	}

	return nil
}

func (a *App) runKeysDelete(cmd *cobra.Command, args []string) error {
	provider := args[0]

	ks, err := a.newKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	if err := ks.Delete(provider); err != nil {
		if _, ok := err.(*keystore.ErrKeyNotFound); ok {
			return fmt.Errorf("no key stored for %s", provider)
		}
		return fmt.Errorf("failed to delete key: %w", err)
	}

	fmt.Fprintf(a.stdout, "API key for %s deleted.\n", provider)
	return nil
}

func readSecretInput(r io.Reader, w io.Writer) (string, error) {
	// If input is a terminal-backed file, read without echo.
	if f, ok := r.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		keyBytes, err := term.ReadPassword(int(f.Fd()))
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintln(w) // newline after hidden input
		return strings.TrimSpace(string(keyBytes)), nil
	}

	// Fallback for non-terminal (e.g., piped input or tests).
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
