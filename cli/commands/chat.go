package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers"
	"github.com/petal-labs/iris/providers/anthropic"
	"github.com/petal-labs/iris/providers/gemini"
	"github.com/petal-labs/iris/providers/huggingface"
	"github.com/petal-labs/iris/providers/ollama"
	"github.com/petal-labs/iris/providers/openai"
	"github.com/petal-labs/iris/providers/xai"
	"github.com/petal-labs/iris/providers/zai"
	"github.com/spf13/cobra"
)

// Exit codes
const (
	ExitSuccess    = 0
	ExitValidation = 1
	ExitProvider   = 2
	ExitNetwork    = 3
)

var (
	prompt      string
	system      string
	temperature float32
	maxTokens   int
	stream      bool
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Send a chat completion request",
	Long: `Send a chat completion request to an LLM provider.

Examples:
  iris chat --provider openai --model gpt-4o --prompt "Hello"
  iris chat --prompt "Hello" --stream
  iris chat --prompt "Hello" --json`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVar(&prompt, "prompt", "", "User message (required)")
	chatCmd.Flags().StringVar(&system, "system", "", "System message")
	chatCmd.Flags().Float32Var(&temperature, "temperature", 0, "Temperature (0 = use default)")
	chatCmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max tokens (0 = use default)")
	chatCmd.Flags().BoolVar(&stream, "stream", false, "Enable streaming output")

	_ = chatCmd.MarkFlagRequired("prompt")
}

func runChat(cmd *cobra.Command, args []string) error {
	// Validate provider
	providerID := GetProvider()
	if providerID == "" {
		return exitWithCode(ExitValidation, fmt.Errorf("provider required: use --provider flag or set default_provider in config"))
	}

	// Validate model
	modelID := GetModel()
	if modelID == "" {
		return exitWithCode(ExitValidation, fmt.Errorf("model required: use --model flag or set default_model in config"))
	}

	// Get API key from keystore
	ks, err := keystore.NewKeystore()
	if err != nil {
		return exitWithCode(ExitValidation, fmt.Errorf("failed to open keystore: %w", err))
	}

	apiKey, err := ks.Get(providerID)
	if err != nil {
		if _, ok := err.(*keystore.ErrKeyNotFound); ok {
			return exitWithCode(ExitValidation, fmt.Errorf("no API key for %s: run 'iris keys set %s' first", providerID, providerID))
		}
		return exitWithCode(ExitValidation, fmt.Errorf("failed to get API key: %w", err))
	}

	// Create provider
	provider, err := createProvider(providerID, apiKey)
	if err != nil {
		return exitWithCode(ExitValidation, err)
	}

	// Create client and build request
	client := core.NewClient(provider)
	builder := client.Chat(core.ModelID(modelID)).User(prompt)

	if system != "" {
		// System message should come before user message
		builder = client.Chat(core.ModelID(modelID)).System(system).User(prompt)
	}

	if temperature > 0 {
		builder = builder.Temperature(temperature)
	}

	if maxTokens > 0 {
		builder = builder.MaxTokens(maxTokens)
	}

	ctx := context.Background()

	if stream {
		return runStreamingChat(ctx, builder)
	}
	return runNonStreamingChat(ctx, builder)
}

func runNonStreamingChat(ctx context.Context, builder *core.ChatBuilder) error {
	resp, err := builder.GetResponse(ctx)
	if err != nil {
		return handleChatError(err)
	}

	if IsJSONOutput() {
		return outputJSON(resp)
	}

	// Text output
	fmt.Printf("> %s\n", prompt)
	fmt.Println(resp.Output)
	return nil
}

func runStreamingChat(ctx context.Context, builder *core.ChatBuilder) error {
	chatStream, err := builder.Stream(ctx)
	if err != nil {
		return handleChatError(err)
	}

	if IsJSONOutput() {
		// Accumulate for JSON output
		resp, err := core.DrainStream(ctx, chatStream)
		if err != nil {
			return handleChatError(err)
		}
		return outputJSON(resp)
	}

	// Stream text output
	fmt.Printf("> %s\n", prompt)

	var finalResp *core.ChatResponse
	var streamErr error

	// Read chunks as they arrive
	for chunk := range chatStream.Ch {
		fmt.Print(chunk.Delta)
	}

	// Check for errors
	select {
	case err := <-chatStream.Err:
		if err != nil {
			streamErr = err
		}
	default:
	}

	// Get final response
	select {
	case resp := <-chatStream.Final:
		finalResp = resp
	default:
	}

	// Print final newline
	fmt.Println()

	if streamErr != nil {
		return handleChatError(streamErr)
	}

	// Log usage if verbose
	if IsVerbose() && finalResp != nil {
		fmt.Fprintf(os.Stderr, "Usage: %d prompt + %d completion = %d total tokens\n",
			finalResp.Usage.PromptTokens,
			finalResp.Usage.CompletionTokens,
			finalResp.Usage.TotalTokens)
	}

	return nil
}

func createProvider(providerID, apiKey string) (core.Provider, error) {
	switch providerID {
	case "openai":
		// Check for custom base URL in config
		var opts []openai.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, openai.WithBaseURL(pc.BaseURL))
			}
		}
		return openai.New(apiKey, opts...), nil
	case "anthropic":
		// Check for custom base URL in config
		var opts []anthropic.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, anthropic.WithBaseURL(pc.BaseURL))
			}
		}
		return anthropic.New(apiKey, opts...), nil
	case "gemini":
		// Check for custom base URL in config
		var opts []gemini.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, gemini.WithBaseURL(pc.BaseURL))
			}
		}
		return gemini.New(apiKey, opts...), nil
	case "xai":
		// Check for custom base URL in config
		var opts []xai.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, xai.WithBaseURL(pc.BaseURL))
			}
		}
		return xai.New(apiKey, opts...), nil
	case "zai":
		// Check for custom base URL in config
		var opts []zai.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, zai.WithBaseURL(pc.BaseURL))
			}
		}
		return zai.New(apiKey, opts...), nil
	case "ollama":
		// Check for custom base URL in config
		var opts []ollama.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, ollama.WithBaseURL(pc.BaseURL))
			}
		}
		// API key is optional for local Ollama
		if apiKey != "" {
			opts = append(opts, ollama.WithAPIKey(apiKey))
		}
		return ollama.New(opts...), nil
	case "huggingface":
		// Check for custom base URL in config
		var opts []huggingface.Option
		if cfg := GetConfig(); cfg != nil {
			if pc := cfg.GetProvider(providerID); pc != nil && pc.BaseURL != "" {
				opts = append(opts, huggingface.WithBaseURL(pc.BaseURL))
			}
		}
		return huggingface.New(apiKey, opts...), nil
	default:
		// Try the registry for any additional providers
		if providers.IsRegistered(providerID) {
			return providers.Create(providerID, apiKey)
		}
		return nil, fmt.Errorf("unsupported provider: %s (available: %v)", providerID, providers.List())
	}
}

func handleChatError(err error) error {
	var provErr *core.ProviderError
	if errors.As(err, &provErr) {
		if IsJSONOutput() {
			outputErrorJSON(provErr)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", provErr.Message)
			if provErr.RequestID != "" {
				fmt.Fprintf(os.Stderr, "  Provider: %s, Request ID: %s\n", provErr.Provider, provErr.RequestID)
			}
		}

		// Determine exit code based on error type
		switch {
		case errors.Is(err, core.ErrNetwork):
			return exitWithCode(ExitNetwork, err)
		default:
			return exitWithCode(ExitProvider, err)
		}
	}

	// Network errors
	if errors.Is(err, core.ErrNetwork) {
		if IsJSONOutput() {
			outputSimpleErrorJSON("network_error", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: network error: %v\n", err)
		}
		return exitWithCode(ExitNetwork, err)
	}

	// Validation errors
	if errors.Is(err, core.ErrModelRequired) || errors.Is(err, core.ErrNoMessages) {
		if IsJSONOutput() {
			outputSimpleErrorJSON("validation_error", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return exitWithCode(ExitValidation, err)
	}

	// Generic error
	if IsJSONOutput() {
		outputSimpleErrorJSON("error", err.Error())
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return exitWithCode(ExitProvider, err)
}

func outputJSON(resp *core.ChatResponse) error {
	output := map[string]interface{}{
		"id":     resp.ID,
		"model":  resp.Model,
		"output": resp.Output,
		"usage": map[string]int{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputErrorJSON(provErr *core.ProviderError) {
	output := map[string]interface{}{
		"error": map[string]interface{}{
			"type":       provErr.Code,
			"message":    provErr.Message,
			"provider":   provErr.Provider,
			"request_id": provErr.RequestID,
		},
	}

	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func outputSimpleErrorJSON(errType, message string) {
	output := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	}

	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

// exitError wraps an error with an exit code.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

func exitWithCode(code int, err error) error {
	return &exitError{code: code, err: err}
}
