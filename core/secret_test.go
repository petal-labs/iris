package core

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestNewSecret(t *testing.T) {
	secret := NewSecret("my-api-key")
	if secret.value != "my-api-key" {
		t.Errorf("NewSecret() value = %q, want %q", secret.value, "my-api-key")
	}
}

func TestSecretString(t *testing.T) {
	secret := NewSecret("sk-abc123xyz")
	got := secret.String()
	want := "[REDACTED]"
	if got != want {
		t.Errorf("Secret.String() = %q, want %q", got, want)
	}
}

func TestSecretGoString(t *testing.T) {
	secret := NewSecret("sk-abc123xyz")
	got := secret.GoString()
	want := "core.Secret{[REDACTED]}"
	if got != want {
		t.Errorf("Secret.GoString() = %q, want %q", got, want)
	}
}

func TestSecretMarshalJSON(t *testing.T) {
	secret := NewSecret("sk-abc123xyz")
	got, err := secret.MarshalJSON()
	if err != nil {
		t.Fatalf("Secret.MarshalJSON() error = %v", err)
	}
	want := `"[REDACTED]"`
	if string(got) != want {
		t.Errorf("Secret.MarshalJSON() = %s, want %s", got, want)
	}
}

func TestSecretMarshalText(t *testing.T) {
	secret := NewSecret("sk-abc123xyz")
	got, err := secret.MarshalText()
	if err != nil {
		t.Fatalf("Secret.MarshalText() error = %v", err)
	}
	want := "[REDACTED]"
	if string(got) != want {
		t.Errorf("Secret.MarshalText() = %s, want %s", got, want)
	}
}

func TestSecretExpose(t *testing.T) {
	value := "sk-abc123xyz"
	secret := NewSecret(value)
	got := secret.Expose()
	if got != value {
		t.Errorf("Secret.Expose() = %q, want %q", got, value)
	}
}

func TestSecretIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"empty string", "", true},
		{"non-empty string", "sk-abc123", false},
		{"whitespace only", "  ", false}, // whitespace is not considered empty
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := NewSecret(tt.value)
			got := secret.IsEmpty()
			if got != tt.want {
				t.Errorf("Secret.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecretInFmtPrintf(t *testing.T) {
	secret := NewSecret("sk-abc123xyz")
	actualValue := "sk-abc123xyz"

	tests := []struct {
		name   string
		format string
		want   string
	}{
		{"%v", "%v", "[REDACTED]"},
		{"%s", "%s", "[REDACTED]"},
		{"%+v", "%+v", "[REDACTED]"},
		{"%#v", "%#v", "core.Secret{[REDACTED]}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf(tt.format, secret)
			if got != tt.want {
				t.Errorf("fmt.Sprintf(%q, secret) = %q, want %q", tt.format, got, tt.want)
			}
			// Ensure actual value is never in output
			if got == actualValue || containsSubstring(got, actualValue) {
				t.Errorf("fmt.Sprintf(%q, secret) exposed actual value", tt.format)
			}
		})
	}
}

func TestSecretInStructPrinting(t *testing.T) {
	type Config struct {
		Name   string
		APIKey Secret
	}

	cfg := Config{
		Name:   "test-config",
		APIKey: NewSecret("sk-super-secret-key"),
	}
	actualValue := "sk-super-secret-key"

	tests := []struct {
		name   string
		format string
	}{
		{"%v", "%v"},
		{"%+v", "%+v"},
		{"%#v", "%#v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf(tt.format, cfg)
			if containsSubstring(got, actualValue) {
				t.Errorf("fmt.Sprintf(%q, config) exposed actual secret value: %s", tt.format, got)
			}
			if !containsSubstring(got, "REDACTED") {
				t.Errorf("fmt.Sprintf(%q, config) should contain REDACTED: %s", tt.format, got)
			}
		})
	}
}

func TestSecretJSONInStruct(t *testing.T) {
	type Config struct {
		Name   string `json:"name"`
		APIKey Secret `json:"api_key"`
	}

	cfg := Config{
		Name:   "test-config",
		APIKey: NewSecret("sk-super-secret-key"),
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	got := string(data)
	if containsSubstring(got, "sk-super-secret-key") {
		t.Errorf("json.Marshal() exposed actual secret value: %s", got)
	}
	if !containsSubstring(got, "[REDACTED]") {
		t.Errorf("json.Marshal() should contain [REDACTED]: %s", got)
	}

	// Verify valid JSON structure
	expected := `{"name":"test-config","api_key":"[REDACTED]"}`
	if got != expected {
		t.Errorf("json.Marshal() = %s, want %s", got, expected)
	}
}

func TestSecretEmptyValue(t *testing.T) {
	secret := NewSecret("")

	if secret.String() != "[REDACTED]" {
		t.Error("Empty secret should still return [REDACTED] for String()")
	}

	if !secret.IsEmpty() {
		t.Error("Empty secret should return true for IsEmpty()")
	}

	if secret.Expose() != "" {
		t.Error("Empty secret should return empty string for Expose()")
	}
}

func TestSecretWithSpecialCharacters(t *testing.T) {
	specialValues := []string{
		"key with spaces",
		"key\nwith\nnewlines",
		"key\twith\ttabs",
		`key"with"quotes`,
		"key<with>brackets",
		"key&with&ampersands",
		"emoji-key-\U0001F511",
	}

	for _, value := range specialValues {
		t.Run(value[:10]+"...", func(t *testing.T) {
			secret := NewSecret(value)

			// String should be redacted
			if secret.String() != "[REDACTED]" {
				t.Errorf("Secret.String() = %q, want [REDACTED]", secret.String())
			}

			// Expose should return exact value
			if secret.Expose() != value {
				t.Errorf("Secret.Expose() = %q, want %q", secret.Expose(), value)
			}
		})
	}
}

// containsSubstring checks if s contains substr (case-sensitive)
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
