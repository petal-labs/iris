// Package iris provides convenience functions for creating LLM clients.
//
// This package provides one-liner factory functions for the most common use cases:
//
//	// Create an OpenAI client from environment variable
//	client, err := iris.OpenAI()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Auto-detect provider from available environment variables
//	client, err := iris.FromEnv()
//
// For more control, use the provider packages directly:
//
//	provider := openai.New(apiKey, openai.WithBaseURL(customURL))
//	client := core.NewClient(provider)
package iris

import (
	"errors"
	"os"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/gemini"
	"github.com/petal-labs/iris/providers/ollama"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/xai"
)

// ErrNoAPIKey is returned when no API key is found in environment variables.
var ErrNoAPIKey = errors.New("no API key found in environment")

// OpenAI creates a client using the OPENAI_API_KEY environment variable.
//
// Example:
//
//	client, err := iris.OpenAI()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, _ := client.Chat("gpt-4o").User("Hello!").Send(ctx)
func OpenAI() (*core.Client, error) {
	provider, err := openai.NewFromEnv()
	if err != nil {
		return nil, err
	}
	return core.NewClient(provider), nil
}

// Anthropic creates a client using the ANTHROPIC_API_KEY environment variable.
//
// Example:
//
//	client, err := iris.Anthropic()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, _ := client.Chat("claude-sonnet-4-5").User("Hello!").Send(ctx)
func Anthropic() (*core.Client, error) {
	provider, err := anthropic.NewFromEnv()
	if err != nil {
		return nil, err
	}
	return core.NewClient(provider), nil
}

// Gemini creates a client using the GEMINI_API_KEY or GOOGLE_API_KEY environment variable.
//
// Example:
//
//	client, err := iris.Gemini()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, _ := client.Chat("gemini-2.5-flash").User("Hello!").Send(ctx)
func Gemini() (*core.Client, error) {
	provider, err := gemini.NewFromEnv()
	if err != nil {
		return nil, err
	}
	return core.NewClient(provider), nil
}

// XAI creates a client using the XAI_API_KEY environment variable.
//
// Example:
//
//	client, err := iris.XAI()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, _ := client.Chat("grok-2").User("Hello!").Send(ctx)
func XAI() (*core.Client, error) {
	provider, err := xai.NewFromEnv()
	if err != nil {
		return nil, err
	}
	return core.NewClient(provider), nil
}

// Ollama creates a client for a local Ollama instance.
// No API key is required for local Ollama.
//
// Example:
//
//	client, err := iris.Ollama()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, _ := client.Chat("llama3.2").User("Hello!").Send(ctx)
func Ollama() (*core.Client, error) {
	provider := ollama.New()
	return core.NewClient(provider), nil
}

// FromEnv auto-detects the provider from available environment variables.
// It checks for API keys in this order:
//  1. OPENAI_API_KEY → OpenAI
//  2. ANTHROPIC_API_KEY → Anthropic
//  3. GEMINI_API_KEY or GOOGLE_API_KEY → Gemini
//  4. XAI_API_KEY → XAI
//
// If no API key is found, it returns ErrNoAPIKey.
//
// Example:
//
//	client, err := iris.FromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
func FromEnv() (*core.Client, error) {
	// Check OpenAI
	if os.Getenv("OPENAI_API_KEY") != "" {
		return OpenAI()
	}

	// Check Anthropic
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return Anthropic()
	}

	// Check Gemini
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		return Gemini()
	}

	// Check XAI
	if os.Getenv("XAI_API_KEY") != "" {
		return XAI()
	}

	return nil, ErrNoAPIKey
}

// MustOpenAI creates an OpenAI client or panics if the API key is not set.
// Use this for simple scripts where error handling is not needed.
func MustOpenAI() *core.Client {
	client, err := OpenAI()
	if err != nil {
		panic(err)
	}
	return client
}

// MustAnthropic creates an Anthropic client or panics if the API key is not set.
func MustAnthropic() *core.Client {
	client, err := Anthropic()
	if err != nil {
		panic(err)
	}
	return client
}

// MustGemini creates a Gemini client or panics if the API key is not set.
func MustGemini() *core.Client {
	client, err := Gemini()
	if err != nil {
		panic(err)
	}
	return client
}

// MustFromEnv creates a client from environment or panics if no API key is found.
func MustFromEnv() *core.Client {
	client, err := FromEnv()
	if err != nil {
		panic(err)
	}
	return client
}
