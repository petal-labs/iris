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
	keysCmd.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Migrate the keystore to v2 format under the master key",
		Long: `Re-encrypt the keystore at ~/.iris/keys.enc in v2 format using the
IRIS_KEYSTORE_KEY master key (Argon2id + AES-256-GCM).

This upgrades stores written by older versions, which were encrypted with a
key derived from machine-specific data. The original file is preserved at
~/.iris/keys.enc.bak; delete it once you have verified the migration.

Requires IRIS_KEYSTORE_KEY to be set, e.g.:
  export IRIS_KEYSTORE_KEY=$(openssl rand -base64 32)`,
		RunE: a.runKeysMigrate,
	})

	return keysCmd
}

// openKeystore opens the keystore via the configured factory and warns when
// it is running with the legacy machine-derived key.
func (a *App) openKeystore() (keystore.Keystore, error) {
	ks, err := a.newKeystore()
	if err != nil {
		return nil, err
	}
	if lk, ok := ks.(interface{ UsesLegacyKey() bool }); ok && lk.UsesLegacyKey() {
		fmt.Fprintln(a.stderr, "Warning: IRIS_KEYSTORE_KEY is not set; API keys are encrypted with a predictable machine-derived key.")
		fmt.Fprintln(a.stderr, "Set a master key and migrate: export IRIS_KEYSTORE_KEY=$(openssl rand -base64 32) && iris keys migrate")
	}
	return ks, nil
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

	ks, err := a.openKeystore()
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
	ks, err := a.openKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	a.printKeystoreStatus(ks)

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

// printKeystoreStatus prints a one-line summary of the keystore encryption
// state, and nudges the user toward migrating when needed.
func (a *App) printKeystoreStatus(ks keystore.Keystore) {
	fk, ok := ks.(*keystore.FileKeystore)
	if !ok {
		return
	}
	if fk.UsesLegacyKey() {
		fmt.Fprintln(a.stdout, "Keystore: legacy machine-derived key (set IRIS_KEYSTORE_KEY and run 'iris keys migrate' to upgrade).")
		return
	}
	needs, err := fk.NeedsMigration()
	if err != nil {
		return
	}
	if needs {
		fmt.Fprintln(a.stdout, "Keystore: pending migration to the IRIS_KEYSTORE_KEY master key (run 'iris keys migrate').")
		return
	}
	fmt.Fprintln(a.stdout, "Keystore: v2 format, IRIS_KEYSTORE_KEY master key.")
}

func (a *App) runKeysDelete(cmd *cobra.Command, args []string) error {
	provider := args[0]

	ks, err := a.openKeystore()
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

func (a *App) runKeysMigrate(cmd *cobra.Command, args []string) error {
	ks, err := a.newKeystore()
	if err != nil {
		return fmt.Errorf("failed to open keystore: %w", err)
	}

	fk, ok := ks.(*keystore.FileKeystore)
	if !ok {
		return fmt.Errorf("keystore does not support migration (%T)", ks)
	}

	if fk.UsesLegacyKey() {
		return fmt.Errorf("IRIS_KEYSTORE_KEY is not set; migration requires a master key.\n\nSet one first, e.g.:\n  export IRIS_KEYSTORE_KEY=$(openssl rand -base64 32)\n\nThen re-run: iris keys migrate")
	}

	result, err := fk.MigrateToV2()
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	switch result {
	case keystore.MigrateNone:
		fmt.Fprintln(a.stdout, "No keystore found; nothing to migrate.")
	case keystore.MigrateAlreadyCurrent:
		fmt.Fprintln(a.stdout, "Keystore is already in v2 format under the current master key.")
	case keystore.MigrateRekeyed:
		fmt.Fprintf(a.stdout, "Keystore migrated to v2 format under the IRIS_KEYSTORE_KEY master key.\n")
		fmt.Fprintf(a.stdout, "Backup of the previous store saved to %s.bak\n", fk.Path())
		fmt.Fprintf(a.stdout, "Delete the backup once verified: rm %s.bak\n", fk.Path())
	}

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
