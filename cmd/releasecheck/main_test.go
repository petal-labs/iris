package main

import (
	"strings"
	"testing"
)

func TestValidateReleaseTagMatchesLatestChangelogEntry(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

## [1.2.0] - 2026-08-28

## [1.1.0] - 2026-08-01
`
	if err := validateReleaseTag("v1.2.0", changelog); err != nil {
		t.Fatalf("validate matching release: %v", err)
	}
}

func TestValidateReleaseTagSupportsCRLFChangelog(t *testing.T) {
	changelog := "## [Unreleased]\r\n\r\n## [1.2.0] - 2026-08-28\r\n"
	if err := validateReleaseTag("v1.2.0", changelog); err != nil {
		t.Fatalf("validate CRLF changelog: %v", err)
	}
}

func TestValidateReleaseTagRejectsStaleTag(t *testing.T) {
	changelog := "## [1.2.0] - 2026-08-28\n\n## [1.1.0] - 2026-08-01\n"
	err := validateReleaseTag("v1.1.0", changelog)
	if err == nil || !strings.Contains(err.Error(), "latest CHANGELOG release") {
		t.Fatalf("expected latest release mismatch, got %v", err)
	}
}

func TestValidateReleaseTagRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		changelog string
		wantError string
	}{
		{name: "invalid tag", tag: "1.2.0", changelog: "## [1.2.0] - 2026-08-28\n", wantError: "semantic-version tag"},
		{name: "invalid numeric prerelease", tag: "v1.2.0-01", changelog: "## [1.2.0-01] - 2026-08-28\n", wantError: "semantic-version tag"},
		{name: "missing release", tag: "v1.2.0", changelog: "## [Unreleased]\n", wantError: "no dated release"},
		{name: "invalid date", tag: "v1.2.0", changelog: "## [1.2.0] - 2026-99-99\n", wantError: "invalid date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReleaseTag(tt.tag, tt.changelog)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}
