package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petal-labs/iris/cli/keystore"
)

// newKeysTestApp builds an App whose keystore points at a temp file and whose
// I/O is captured. The factory replicates keystore.NewKeystoreAtPath so tests
// can control both the path and the master key environment.
func newKeysTestApp(t *testing.T, path string, envKey string, stdin string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	if envKey != "" {
		t.Setenv(keystore.DefaultMasterKeyEnvVar, envKey)
	} else {
		t.Setenv(keystore.DefaultMasterKeyEnvVar, "")
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(
		WithKeystoreFactory(func() (keystore.Keystore, error) {
			return keystore.NewKeystoreAtPath(path)
		}),
		WithIO(strings.NewReader(stdin), stdout, stderr),
	)
	return app, stdout, stderr
}

func TestKeysSetAndListLegacyModeWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.enc")
	app, stdout, stderr := newKeysTestApp(t, path, "", "sk-test-key-123\n")

	if err := app.runKeysSet(nil, []string{"openai"}); err != nil {
		t.Fatalf("runKeysSet() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "stored successfully") {
		t.Errorf("set output = %q, want 'stored successfully'", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Warning: IRIS_KEYSTORE_KEY is not set") {
		t.Errorf("stderr = %q, want legacy-key warning", stderr.String())
	}

	stdout.Reset()
	if err := app.runKeysList(nil, nil); err != nil {
		t.Fatalf("runKeysList() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "openai") {
		t.Errorf("list output = %q, want 'openai'", out)
	}
	if !strings.Contains(out, "legacy machine-derived key") {
		t.Errorf("list output = %q, want legacy status line", out)
	}
}

func TestKeysSetListDeleteWithMasterKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.enc")
	app, stdout, stderr := newKeysTestApp(t, path, "test-master-key", "sk-test-key-123\n")

	if err := app.runKeysSet(nil, []string{"openai"}); err != nil {
		t.Fatalf("runKeysSet() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want no warning with master key set", stderr.String())
	}

	stdout.Reset()
	if err := app.runKeysList(nil, nil); err != nil {
		t.Fatalf("runKeysList() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "v2 format, IRIS_KEYSTORE_KEY master key") {
		t.Errorf("list output = %q, want v2 status line", stdout.String())
	}

	if err := app.runKeysDelete(nil, []string{"openai"}); err != nil {
		t.Fatalf("runKeysDelete() error = %v", err)
	}

	stdout.Reset()
	if err := app.runKeysList(nil, nil); err != nil {
		t.Fatalf("runKeysList() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "No API keys stored.") {
		t.Errorf("list output = %q after delete, want empty", stdout.String())
	}
}

func TestKeysSetEmptyKeyRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.enc")
	app, _, _ := newKeysTestApp(t, path, "test-master-key", "\n")

	err := app.runKeysSet(nil, []string{"openai"})
	if err == nil {
		t.Fatal("runKeysSet() should reject empty keys")
	}
}

func TestKeysListPendingMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.enc")

	// Create a store with the legacy machine key (no master key set).
	t.Setenv(keystore.DefaultMasterKeyEnvVar, "")
	legacy, err := keystore.NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}
	if err := legacy.Set("openai", "sk-old"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	app, stdout, _ := newKeysTestApp(t, path, "test-master-key", "")
	if err := app.runKeysList(nil, nil); err != nil {
		t.Fatalf("runKeysList() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "pending migration") {
		t.Errorf("list output = %q, want pending-migration status line", stdout.String())
	}
}

func TestKeysMigrateRequiresMasterKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.enc")
	app, _, _ := newKeysTestApp(t, path, "", "")

	err := app.runKeysMigrate(nil, nil)
	if err == nil {
		t.Fatal("runKeysMigrate() should fail without IRIS_KEYSTORE_KEY")
	}
	if !strings.Contains(err.Error(), "IRIS_KEYSTORE_KEY") {
		t.Errorf("error = %q, want mention of IRIS_KEYSTORE_KEY", err.Error())
	}
}

func TestKeysMigrateRekeysStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.enc")

	// Seed a legacy machine-keyed store.
	t.Setenv(keystore.DefaultMasterKeyEnvVar, "")
	legacy, err := keystore.NewFileKeystore(path)
	if err != nil {
		t.Fatalf("NewFileKeystore() error = %v", err)
	}
	if err := legacy.Set("openai", "sk-legacy-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	app, stdout, _ := newKeysTestApp(t, path, "test-master-key", "")
	if err := app.runKeysMigrate(nil, nil); err != nil {
		t.Fatalf("runKeysMigrate() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "migrated to v2 format") {
		t.Errorf("migrate output = %q, want migration confirmation", out)
	}
	if !strings.Contains(out, path+".bak") {
		t.Errorf("migrate output = %q, want backup path %q", out, path+".bak")
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	// The migrated store must be readable by a strict master-key store.
	strict, err := keystore.NewFileKeystoreWithSource(path, &staticKeySource{key: "test-master-key"})
	if err != nil {
		t.Fatalf("NewFileKeystoreWithSource() error = %v", err)
	}
	value, err := strict.Get("openai")
	if err != nil {
		t.Fatalf("strict Get() error = %v", err)
	}
	if value != "sk-legacy-value" {
		t.Errorf("strict Get() = %q, want sk-legacy-value", value)
	}

	// Second run is a no-op.
	stdout.Reset()
	if err := app.runKeysMigrate(nil, nil); err != nil {
		t.Fatalf("second runKeysMigrate() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "already in v2 format") {
		t.Errorf("second migrate output = %q, want already-current message", stdout.String())
	}
}

// staticKeySource adapts a string key to keystore.MasterKeySource for tests
// in this package.
type staticKeySource struct {
	key string
}

func (s *staticKeySource) GetMasterKey() ([]byte, error) {
	return []byte(s.key), nil
}
