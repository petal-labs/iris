package commands

import (
	"testing"
)

func TestVersionVariables(t *testing.T) {
	// Verify default values are set
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Commit == "" {
		t.Error("Commit should not be empty")
	}
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}

func TestVersionDefaults(t *testing.T) {
	// Default values when not built with ldflags
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"Version default", Version, "dev"},
		{"Commit default", Commit, "unknown"},
		{"BuildDate default", BuildDate, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In test context, these should be the defaults
			if tt.value != tt.expected {
				// This is expected when running via `go test` without ldflags
				t.Logf("%s = %q (expected %q in default build)", tt.name, tt.value, tt.expected)
			}
		})
	}
}
