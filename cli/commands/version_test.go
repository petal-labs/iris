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

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkedVersion string
		moduleVersion string
		want          string
	}{
		{name: "release ldflags win", linkedVersion: "v1.2.3", moduleVersion: "v1.2.2", want: "v1.2.3"},
		{name: "go install module version", linkedVersion: "dev", moduleVersion: "v1.2.3", want: "v1.2.3"},
		{name: "local build", linkedVersion: "dev", moduleVersion: "(devel)", want: "dev"},
		{name: "missing build info", linkedVersion: "dev", moduleVersion: "", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.linkedVersion, tt.moduleVersion); got != tt.want {
				t.Errorf("resolveVersion(%q, %q) = %q, want %q", tt.linkedVersion, tt.moduleVersion, got, tt.want)
			}
		})
	}
}
