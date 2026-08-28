// Command releasecheck validates release inputs before artifacts are published.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var semanticVersionTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var changelogReleasePattern = regexp.MustCompile(`^## \[([0-9A-Za-z.+-]+)\] - ([0-9]{4}-[0-9]{2}-[0-9]{2})\r?$`)

func main() {
	tag := flag.String("tag", "", "semantic-version release tag")
	changelogPath := flag.String("changelog", "CHANGELOG.md", "path to the changelog")
	flag.Parse()

	content, err := os.ReadFile(*changelogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read changelog: %v\n", err)
		os.Exit(1)
	}
	if err := validateReleaseTag(*tag, string(content)); err != nil {
		fmt.Fprintf(os.Stderr, "validate release: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("release tag %s matches the latest CHANGELOG entry\n", *tag)
}

func validateReleaseTag(tag, changelog string) error {
	if !validSemanticVersionTag(tag) {
		return fmt.Errorf("%q is not a valid v-prefixed semantic-version tag", tag)
	}

	scanner := bufio.NewScanner(strings.NewReader(changelog))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "## [") || line == "## [Unreleased]" {
			continue
		}
		match := changelogReleasePattern.FindStringSubmatch(line)
		if len(match) != 3 {
			return fmt.Errorf("latest CHANGELOG release heading %q is malformed", line)
		}
		if _, err := time.Parse("2006-01-02", match[2]); err != nil {
			return fmt.Errorf("latest CHANGELOG release has invalid date %q: %w", match[2], err)
		}
		version := strings.TrimPrefix(tag, "v")
		if match[1] != version {
			return fmt.Errorf("tag %q does not match latest CHANGELOG release %q", tag, match[1])
		}
		return nil
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan CHANGELOG: %w", err)
	}
	return fmt.Errorf("CHANGELOG contains no dated release heading")
}

func validSemanticVersionTag(tag string) bool {
	match := semanticVersionTagPattern.FindStringSubmatch(tag)
	if len(match) == 0 {
		return false
	}
	for _, identifier := range strings.Split(match[4], ".") {
		if len(identifier) > 1 && identifier[0] == '0' && strings.Trim(identifier, "0123456789") == "" {
			return false
		}
	}
	return true
}
