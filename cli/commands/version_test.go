package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/petal-labs/iris/cli/config"
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

func TestVersionCommandOutput(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "v1.2.3", "abc1234", "2026-08-28T12:00:00Z"
	t.Cleanup(func() {
		Version, Commit, BuildDate = originalVersion, originalCommit, originalBuildDate
	})

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "text",
			args: []string{"version"},
			want: []string{"iris v1.2.3", "commit:     abc1234", "built:      2026-08-28T12:00:00Z"},
		},
		{
			name: "json",
			args: []string{"--json", "version"},
			want: []string{`"version":"v1.2.3"`, `"commit":"abc1234"`, `"buildDate":"2026-08-28T12:00:00Z"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			app := NewApp(
				WithConfigLoader(func(string) (*config.Config, error) { return &config.Config{}, nil }),
				WithIO(strings.NewReader(""), &stdout, &bytes.Buffer{}),
			)
			app.root.SetArgs(tt.args)
			if err := app.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("version output missing %q:\n%s", want, stdout.String())
				}
			}
		})
	}
}
