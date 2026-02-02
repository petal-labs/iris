package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petal-labs/iris/cli/keystore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
	Long:  `Manage API keys for various providers. Keys are stored securely using encryption.`,
}

var keysSetCmd = &cobra.Command{
	Use:   "set <provider>",
	Short: "Set API key for a provider",
	Long:  `Set the API key for a provider. The key will be prompted without echo for security.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runKeysSet,
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored API keys",
	Long:  `List all stored API keys. Only provider names are shown, never key values.`,
	RunE:  runKeysList,
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete <provider>",
	Short: "Delete API key for a provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runKeysDelete,
}

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keysSetCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysDeleteCmd)
}

func runKeysSet(cmd *cobra.Command, args []string) error {
	provider := args[0]

	// Prompt for API key
	fmt.Printf("Enter API key for %s: ", provider)

	// Read without echo if terminal
	var apiKey string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to read key: %w", err)
		}
		apiKey = string(keyBytes)
		fmt.Println() // Newline after hidden input
	} else {
		// Fallback for non-terminal (e.g., piped input)
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read key: %w", err)
		}
		apiKey = strings.TrimSpace(line)
	}

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	ks, err := keystore.NewKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	if err := ks.Set(provider, apiKey); err != nil {
		return fmt.Errorf("failed to store key: %w", err)
	}

	fmt.Printf("API key for %s stored successfully.\n", provider)
	return nil
}

func runKeysList(cmd *cobra.Command, args []string) error {
	ks, err := keystore.NewKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	names, err := ks.List()
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if len(names) == 0 {
		fmt.Println("No API keys stored.")
		return nil
	}

	fmt.Println("Stored keys:")
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}

	return nil
}

func runKeysDelete(cmd *cobra.Command, args []string) error {
	provider := args[0]

	ks, err := keystore.NewKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	if err := ks.Delete(provider); err != nil {
		if _, ok := err.(*keystore.ErrKeyNotFound); ok {
			return fmt.Errorf("no key stored for %s", provider)
		}
		return fmt.Errorf("failed to delete key: %w", err)
	}

	fmt.Printf("API key for %s deleted.\n", provider)
	return nil
}
