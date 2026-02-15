package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

// Exit codes.
const (
	ExitSuccess    = 0
	ExitValidation = 1
	ExitProvider   = 2
	ExitNetwork    = 3
)

func (a *App) newChatCommand() *cobra.Command {
	chatCmd := &cobra.Command{
		Use:   "chat",
		Short: "Send a chat completion request",
		Long: `Send a chat completion request to an LLM provider.

Examples:
  iris chat --provider openai --model gpt-4o --prompt "Hello"
  iris chat --prompt "Hello" --stream
  iris chat --prompt "Hello" --json`,
		RunE: a.runChat,
	}

	chatCmd.Flags().StringVar(&a.chatPrompt, "prompt", "", "User message (required)")
	chatCmd.Flags().StringVar(&a.chatSystem, "system", "", "System message")
	chatCmd.Flags().Float32Var(&a.chatTemperature, "temperature", 0, "Temperature (0 = use default)")
	chatCmd.Flags().IntVar(&a.chatMaxTokens, "max-tokens", 0, "Max tokens (0 = use default)")
	chatCmd.Flags().BoolVar(&a.chatStream, "stream", false, "Enable streaming output")

	_ = chatCmd.MarkFlagRequired("prompt")
	return chatCmd
}

func (a *App) runChat(cmd *cobra.Command, args []string) error {
	// Validate provider.
	providerID := a.provider
	if providerID == "" {
		return exitWithCode(ExitValidation, fmt.Errorf("provider required: use --provider flag or set default_provider in config"))
	}

	// Validate model.
	modelID := a.model
	if modelID == "" {
		return exitWithCode(ExitValidation, fmt.Errorf("model required: use --model flag or set default_model in config"))
	}

	// Get API key from keystore.
	ks, err := a.newKeystore()
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

	// Create provider.
	provider, err := a.createProvider(providerID, apiKey, a.cfg)
	if err != nil {
		return exitWithCode(ExitValidation, err)
	}

	// Create client and build request.
	client := core.NewClient(provider)
	builder := client.Chat(core.ModelID(modelID)).User(a.chatPrompt)

	if a.chatSystem != "" {
		// System message should come before user message.
		builder = client.Chat(core.ModelID(modelID)).System(a.chatSystem).User(a.chatPrompt)
	}
	if a.chatTemperature > 0 {
		builder = builder.Temperature(a.chatTemperature)
	}
	if a.chatMaxTokens > 0 {
		builder = builder.MaxTokens(a.chatMaxTokens)
	}

	ctx := context.Background()
	if a.chatStream {
		return a.runStreamingChat(ctx, builder)
	}
	return a.runNonStreamingChat(ctx, builder)
}

func (a *App) runNonStreamingChat(ctx context.Context, builder *core.ChatBuilder) error {
	resp, err := builder.GetResponse(ctx)
	if err != nil {
		return a.handleChatError(err)
	}

	if a.jsonOutput {
		return a.outputJSON(resp)
	}

	// Text output.
	fmt.Fprintf(a.stdout, "> %s\n", a.chatPrompt)
	fmt.Fprintln(a.stdout, resp.Output)
	return nil
}

func (a *App) runStreamingChat(ctx context.Context, builder *core.ChatBuilder) error {
	chatStream, err := builder.Stream(ctx)
	if err != nil {
		return a.handleChatError(err)
	}

	if a.jsonOutput {
		resp, err := core.DrainStream(ctx, chatStream)
		if err != nil {
			return a.handleChatError(err)
		}
		return a.outputJSON(resp)
	}

	// Stream text output.
	fmt.Fprintf(a.stdout, "> %s\n", a.chatPrompt)

	var finalResp *core.ChatResponse
	var streamErr error

	// Read chunks as they arrive.
	for chunk := range chatStream.Ch {
		fmt.Fprint(a.stdout, chunk.Delta)
	}

	// Check for errors.
	select {
	case err := <-chatStream.Err:
		if err != nil {
			streamErr = err
		}
	default:
	}

	// Get final response.
	select {
	case resp := <-chatStream.Final:
		finalResp = resp
	default:
	}

	// Print final newline.
	fmt.Fprintln(a.stdout)

	if streamErr != nil {
		return a.handleChatError(streamErr)
	}

	// Log usage if verbose.
	if a.verbose && finalResp != nil {
		fmt.Fprintf(a.stderr, "Usage: %d prompt + %d completion = %d total tokens\n",
			finalResp.Usage.PromptTokens,
			finalResp.Usage.CompletionTokens,
			finalResp.Usage.TotalTokens)
	}

	return nil
}

func (a *App) handleChatError(err error) error {
	var provErr *core.ProviderError
	if errors.As(err, &provErr) {
		if a.jsonOutput {
			a.outputErrorJSON(provErr)
		} else {
			fmt.Fprintf(a.stderr, "Error: %s\n", provErr.Message)
			if provErr.RequestID != "" {
				fmt.Fprintf(a.stderr, "  Provider: %s, Request ID: %s\n", provErr.Provider, provErr.RequestID)
			}
		}

		// Determine exit code based on error type.
		switch {
		case errors.Is(err, core.ErrNetwork):
			return exitWithCode(ExitNetwork, err)
		default:
			return exitWithCode(ExitProvider, err)
		}
	}

	// Network errors.
	if errors.Is(err, core.ErrNetwork) {
		if a.jsonOutput {
			a.outputSimpleErrorJSON("network_error", err.Error())
		} else {
			fmt.Fprintf(a.stderr, "Error: network error: %v\n", err)
		}
		return exitWithCode(ExitNetwork, err)
	}

	// Validation errors.
	if errors.Is(err, core.ErrModelRequired) || errors.Is(err, core.ErrNoMessages) {
		if a.jsonOutput {
			a.outputSimpleErrorJSON("validation_error", err.Error())
		} else {
			fmt.Fprintf(a.stderr, "Error: %v\n", err)
		}
		return exitWithCode(ExitValidation, err)
	}

	// Generic error.
	if a.jsonOutput {
		a.outputSimpleErrorJSON("error", err.Error())
	} else {
		fmt.Fprintf(a.stderr, "Error: %v\n", err)
	}
	return exitWithCode(ExitProvider, err)
}

func (a *App) outputJSON(resp *core.ChatResponse) error {
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

	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func (a *App) outputErrorJSON(provErr *core.ProviderError) {
	output := map[string]interface{}{
		"error": map[string]interface{}{
			"type":       provErr.Code,
			"message":    provErr.Message,
			"provider":   provErr.Provider,
			"request_id": provErr.RequestID,
		},
	}

	enc := json.NewEncoder(a.stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
}

func (a *App) outputSimpleErrorJSON(errType, message string) {
	output := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	}

	enc := json.NewEncoder(a.stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
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
