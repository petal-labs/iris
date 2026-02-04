package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProvidersDocExists verifies PROVIDERS.md exists and contains required sections.
func TestProvidersDocExists(t *testing.T) {
	content := readDocFile(t, "PROVIDERS.md")

	requiredSections := []string{
		"# Provider Comparison",
		"## Feature Support Matrix",
		"## Provider Details",
		"### OpenAI",
		"### Anthropic",
		"### Google Gemini",
		"### xAI (Grok)",
		"### Perplexity",
		"### Z.ai (GLM)",
		"### Ollama",
		"### HuggingFace",
		"### VoyageAI",
		"## Choosing a Provider",
		"## Rate Limits and Pricing",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("PROVIDERS.md missing required section: %q", section)
		}
	}

	// Verify feature matrix table exists
	if !strings.Contains(content, "| Provider |") {
		t.Error("PROVIDERS.md missing feature support matrix table")
	}

	// Verify code examples exist for major providers
	providers := []string{"openai", "anthropic", "gemini", "ollama"}
	for _, p := range providers {
		if !strings.Contains(strings.ToLower(content), "```go") {
			t.Errorf("PROVIDERS.md missing Go code examples")
			break
		}
		if !strings.Contains(strings.ToLower(content), p+".new") {
			t.Errorf("PROVIDERS.md missing usage example for %s provider", p)
		}
	}
}

// TestArchitectureDocExists verifies ARCHITECTURE.md exists and contains required sections.
func TestArchitectureDocExists(t *testing.T) {
	content := readDocFile(t, "ARCHITECTURE.md")

	requiredSections := []string{
		"# Architecture Design Decisions",
		"## Why Streaming Is First-Class",
		"## Why Provider Is an Interface",
		"## Why ChatBuilder Is Not Thread-Safe",
		"## Why Tools Use json.RawMessage",
		"## Why Sentinel Errors",
		"## Why Exponential Backoff",
		"## Why Features Are Explicit",
		"## Summary of Design Principles",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("ARCHITECTURE.md missing required section: %q", section)
		}
	}

	// Verify each section has rationale
	if strings.Count(content, "### Rationale") < 5 {
		t.Error("ARCHITECTURE.md should have Rationale subsections for design decisions")
	}

	// Verify alternatives considered are documented
	if strings.Count(content, "### Alternatives Considered") < 3 {
		t.Error("ARCHITECTURE.md should document alternatives considered for major decisions")
	}

	// Verify code examples are included
	if !strings.Contains(content, "```go") {
		t.Error("ARCHITECTURE.md should include Go code examples")
	}
}

// TestCoreDocGoExists verifies core/doc.go has comprehensive package documentation.
func TestCoreDocGoExists(t *testing.T) {
	content := readCoreDocFile(t)

	requiredSections := []string{
		"Package core provides",
		"# Client and Provider",
		"# ChatBuilder",
		"# Streaming",
		"# Provider Interface",
		"# Features",
		"# Error Handling",
		"# Telemetry",
		"# Retry Policy",
		"# Thread Safety",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("core/doc.go missing required section: %q", section)
		}
	}

	// Verify examples are included
	if !strings.Contains(content, "provider :=") {
		t.Error("core/doc.go should include provider creation example")
	}
	if !strings.Contains(content, "client.Chat(") {
		t.Error("core/doc.go should include Chat usage example")
	}

	// Verify feature constants are documented
	features := []string{
		"FeatureChat",
		"FeatureChatStreaming",
		"FeatureToolCalling",
		"FeatureReasoning",
	}
	for _, f := range features {
		if !strings.Contains(content, f) {
			t.Errorf("core/doc.go should document %s feature", f)
		}
	}

	// Verify error constants are documented
	errors := []string{
		"ErrUnauthorized",
		"ErrRateLimited",
		"ErrBadRequest",
		"ErrModelRequired",
	}
	for _, e := range errors {
		if !strings.Contains(content, e) {
			t.Errorf("core/doc.go should document %s error", e)
		}
	}
}

// readDocFile reads a file from the docs directory.
func readDocFile(t *testing.T, filename string) string {
	t.Helper()

	path := filepath.Join("..", "docs", filename)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", filename, err)
	}

	return string(content)
}

// readCoreDocFile reads the core/doc.go file.
func readCoreDocFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "core", "doc.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read core/doc.go: %v", err)
	}

	return string(content)
}
