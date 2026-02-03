// Package keystore provides secure storage for API keys.
package keystore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Keystore defines the interface for secure key storage.
type Keystore interface {
	// Set stores a key-value pair.
	Set(name, value string) error
	// Get retrieves a value by name. Returns error if not found.
	Get(name string) (string, error)
	// Delete removes a key by name.
	Delete(name string) error
	// List returns all stored key names.
	List() ([]string, error)
}

// MasterKeySource provides the encryption master key.
// Implementations can source the key from various places (env var, prompt, etc.).
type MasterKeySource interface {
	// GetMasterKey returns the master key for encryption/decryption.
	// Returns an error if the key cannot be obtained.
	GetMasterKey() ([]byte, error)
}

// DefaultMasterKeyEnvVar is the environment variable name for the master key.
const DefaultMasterKeyEnvVar = "IRIS_KEYSTORE_KEY"

// EnvMasterKeySource provides the master key from an environment variable.
type EnvMasterKeySource struct {
	EnvVar string
}

// GetMasterKey returns the master key from the configured environment variable.
func (s *EnvMasterKeySource) GetMasterKey() ([]byte, error) {
	envVar := s.EnvVar
	if envVar == "" {
		envVar = DefaultMasterKeyEnvVar
	}
	key := os.Getenv(envVar)
	if key == "" {
		return nil, errors.New("master key not found in environment variable " + envVar)
	}
	return []byte(key), nil
}

// PromptMasterKeySource provides the master key via interactive prompt.
type PromptMasterKeySource struct {
	Prompter func(prompt string) ([]byte, error)
}

// GetMasterKey prompts the user for the master key.
func (s *PromptMasterKeySource) GetMasterKey() ([]byte, error) {
	if s.Prompter == nil {
		return nil, errors.New("no prompter configured")
	}
	return s.Prompter("Enter keystore password: ")
}

// FallbackMasterKeySource tries multiple sources in order.
type FallbackMasterKeySource struct {
	Sources []MasterKeySource
}

// GetMasterKey tries each source in order until one succeeds.
func (s *FallbackMasterKeySource) GetMasterKey() ([]byte, error) {
	var lastErr error
	for _, source := range s.Sources {
		key, err := source.GetMasterKey()
		if err == nil {
			return key, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no master key sources configured")
}

// ErrKeyNotFound is returned when a requested key does not exist.
type ErrKeyNotFound struct {
	Name string
}

func (e *ErrKeyNotFound) Error() string {
	return "key not found: " + e.Name
}

// ErrMasterKeyRequired is returned when a master key is needed but not provided.
var ErrMasterKeyRequired = errors.New("master key required for keystore operation")

// DefaultKeystorePath returns the default keystore file path.
// - macOS/Linux: ~/.iris/keys.enc
// - Windows: %USERPROFILE%\.iris\keys.enc
func DefaultKeystorePath() string {
	var homeDir string

	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
	} else {
		homeDir = os.Getenv("HOME")
	}

	if homeDir == "" {
		return "keys.enc"
	}

	return filepath.Join(homeDir, ".iris", "keys.enc")
}

// NewKeystore creates a new keystore using file-based encrypted storage.
func NewKeystore() (Keystore, error) {
	return NewFileKeystore(DefaultKeystorePath())
}
