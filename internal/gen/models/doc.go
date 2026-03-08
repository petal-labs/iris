// Package models provides a code generator for model constants from models.dev.
//
// This package allows generating Go code for model definitions from either:
//   - Local TOML files (recommended for reliable builds)
//   - The models.dev GitHub repository (requires API token for rate limits)
//
// The generated code includes:
//   - Model ID constants (e.g., ModelGPT4o, ModelClaudeOpus)
//   - ModelInfo slice with capabilities and API endpoints
//   - Model registry for quick lookups
//
// Usage:
//
// To regenerate model constants from local TOML files:
//
//	go run ./cmd/gen-models -provider=openai -local=./internal/gen/models/data/openai
//
// To regenerate from models.dev (requires GitHub token):
//
//	go run ./cmd/gen-models -provider=openai -token=$GITHUB_TOKEN
//
// The generator is designed to produce code compatible with the existing
// provider implementations in the providers/ directory.
package models
