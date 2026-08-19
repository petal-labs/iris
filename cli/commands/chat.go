package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/petal-labs/iris/cli/keystore"
	"github.com/petal-labs/iris/core"
)

// Exit codes.
const (
	ExitSuccess    = 0
	ExitValidation = 1
	ExitProvider   = 2
	ExitNetwork    = 3
	ExitTimeout    = 4
)

func (a *App) newChatCommand() *cobra.Command {
	chatCmd := &cobra.Command{
		Use:   "chat [prompt]",
		Short: "Send a chat completion request or start an interactive REPL",
		Long: `Send a chat completion request to an LLM provider, or start an interactive
conversation when no prompt is supplied.

The prompt may be given as a flag, a positional argument, or piped via stdin.
When no prompt is provided and stdin is a terminal, an interactive REPL starts
backed by the SDK's Conversation API (Ctrl-D or Ctrl-C to exit).

Examples:
  iris chat --provider openai --model gpt-4o --prompt "Hello"
  iris chat "Hello"
  echo "Hello" | iris chat
  iris chat --system "Be concise"        # interactive REPL
  iris chat --schema person.json --prompt "Extract: John is 30"
  iris chat --prompt "Hello" --stream --timeout 30s`,
		RunE: a.runChat,
	}

	chatCmd.Flags().StringVar(&a.chatPrompt, "prompt", "", "User message (omitted for interactive REPL)")
	chatCmd.Flags().StringVar(&a.chatSystem, "system", "", "System message")
	chatCmd.Flags().Float32Var(&a.chatTemperature, "temperature", 0, "Temperature (0 = use default)")
	chatCmd.Flags().IntVar(&a.chatMaxTokens, "max-tokens", 0, "Max tokens (0 = use default)")
	chatCmd.Flags().BoolVar(&a.chatStream, "stream", false, "Enable streaming output")
	chatCmd.Flags().DurationVar(&a.chatTimeout, "timeout", 0, "Execution timeout (e.g. 30s, 2m); 0 = SDK default")
	chatCmd.Flags().StringVar(&a.chatSchema, "schema", "", "Path to a JSON Schema file constraining output (strict mode)")
	chatCmd.Flags().BoolVar(&a.chatSchemaNonStrict, "schema-non-strict", false, "Relax --schema to non-strict mode")
	chatCmd.Flags().BoolVarP(&a.chatInteractive, "interactive", "i", false, "Force interactive REPL mode")
	return chatCmd
}

func (a *App) runChat(cmd *cobra.Command, args []string) error {
	client, err := a.resolveClient(true, true)
	if err != nil {
		return err
	}

	// Base context is cancelled on SIGINT/SIGTERM so in-flight requests and
	// the REPL can exit cleanly on user interrupt.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	prompt := a.chatPrompt
	if prompt == "" && len(args) > 0 {
		prompt = args[0]
	}

	if prompt == "" {
		// No prompt: enter REPL when stdin is a terminal or --interactive is
		// set; otherwise treat piped stdin as a one-shot prompt.
		isTerminal := false
		if f, ok := a.stdin.(*os.File); ok {
			isTerminal = term.IsTerminal(int(f.Fd()))
		}
		if isTerminal || a.chatInteractive {
			return a.runREPLChat(ctx, client, core.ModelID(a.model))
		}
		data, rerr := io.ReadAll(a.stdin)
		if rerr != nil {
			return exitWithCode(ExitValidation, fmt.Errorf("failed to read prompt from stdin: %w", rerr))
		}
		prompt = strings.TrimSpace(string(data))
		if prompt == "" {
			return exitWithCode(ExitValidation, fmt.Errorf("no prompt: provide --prompt, a positional argument, or pipe input via stdin"))
		}
	}

	return a.runOneShotChat(ctx, client, core.ModelID(a.model), prompt)
}

// resolveClient builds a core.Client from the configured provider/model/key.
// requireKey controls whether a missing keystore entry is fatal; requireModel
// controls whether a model ID must be configured.
func (a *App) resolveClient(requireKey, requireModel bool) (*core.Client, error) {
	providerID := a.provider
	if providerID == "" {
		return nil, exitWithCode(ExitValidation, fmt.Errorf("provider required: use --provider flag or set default_provider in config"))
	}
	if requireModel && a.model == "" {
		return nil, exitWithCode(ExitValidation, fmt.Errorf("model required: use --model flag or set default_model in config"))
	}

	ks, err := a.openKeystore()
	if err != nil {
		return nil, exitWithCode(ExitValidation, fmt.Errorf("failed to open keystore: %w", err))
	}

	apiKey, err := ks.Get(providerID)
	if err != nil {
		if _, ok := err.(*keystore.ErrKeyNotFound); ok {
			// Providers that run locally (e.g. Ollama) work without a stored
			// key; proceed with an empty key and let the provider decide.
			// Ollama Cloud still fails at request time with its own
			// unauthorized error if a key was actually required.
			if !providerAllowsEmptyAPIKey(providerID) && requireKey {
				return nil, exitWithCode(ExitValidation, fmt.Errorf("no API key for %s: run 'iris keys set %s' first", providerID, providerID))
			}
			apiKey = ""
		} else {
			return nil, exitWithCode(ExitValidation, fmt.Errorf("failed to get API key: %w", err))
		}
	}

	provider, err := a.createProvider(providerID, apiKey, a.cfg)
	if err != nil {
		return nil, exitWithCode(ExitValidation, err)
	}

	return core.NewClient(provider), nil
}

// runOneShotChat executes a single chat request with the given prompt.
func (a *App) runOneShotChat(ctx context.Context, client *core.Client, model core.ModelID, prompt string) error {
	builder := client.Chat(model)
	if a.chatSystem != "" {
		builder = builder.System(a.chatSystem)
	}
	builder = builder.User(prompt)

	if a.chatTemperature > 0 {
		builder = builder.Temperature(a.chatTemperature)
	}
	if a.chatMaxTokens > 0 {
		builder = builder.MaxTokens(a.chatMaxTokens)
	}
	if a.chatTimeout > 0 {
		// Let the SDK apply the deadline so it is mapped to ErrTimeout.
		builder = builder.Timeout(a.chatTimeout)
	}
	if a.chatSchema != "" {
		schema, err := loadSchema(a.chatSchema)
		if err != nil {
			return exitWithCode(ExitValidation, err)
		}
		if a.chatSchemaNonStrict {
			builder = builder.ResponseJSONSchemaNonStrict(schema)
		} else {
			builder = builder.ResponseJSONSchema(schema)
		}
	}

	if a.chatStream {
		return a.runStreamingChat(ctx, builder, prompt)
	}
	return a.runNonStreamingChat(ctx, builder, prompt)
}

func (a *App) runNonStreamingChat(ctx context.Context, builder *core.ChatBuilder, prompt string) error {
	resp, err := builder.GetResponse(ctx)
	if err != nil {
		return a.handleChatError(err)
	}

	if a.jsonOutput {
		return a.outputJSON(resp)
	}

	// Text output.
	fmt.Fprintf(a.stdout, "> %s\n", prompt)
	fmt.Fprintln(a.stdout, resp.Output)
	if a.verbose {
		a.printUsage(resp.Usage)
	}
	return nil
}

func (a *App) runStreamingChat(ctx context.Context, builder *core.ChatBuilder, prompt string) error {
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
	fmt.Fprintf(a.stdout, "> %s\n", prompt)

	finalResp, streamErr := a.drainStream(chatStream)
	if streamErr != nil {
		return a.handleChatError(streamErr)
	}

	if a.verbose && finalResp != nil {
		a.printUsage(finalResp.Usage)
	}

	return nil
}

// drainStream prints streamed deltas to stdout and returns the final response
// and/or error observed on the stream. It prints a trailing newline after the
// last delta. The Final/Err selects use default cases to preserve the original
// non-blocking behavior: the telemetry wrapper buffers both channels (cap 1)
// and closes them once the stream completes.
func (a *App) drainStream(chatStream *core.ChatStream) (finalResp *core.ChatResponse, streamErr error) {
	for chunk := range chatStream.Ch {
		fmt.Fprint(a.stdout, chunk.Delta)
	}

	select {
	case err := <-chatStream.Err:
		if err != nil {
			streamErr = err
		}
	default:
	}

	select {
	case resp := <-chatStream.Final:
		finalResp = resp
	default:
	}

	fmt.Fprintln(a.stdout)
	return finalResp, streamErr
}

// runREPLChat runs an interactive, Conversation-backed REPL. Ctrl-D (EOF) and
// Ctrl-C (signal cancellation) produce a clean exit.
func (a *App) runREPLChat(ctx context.Context, client *core.Client, model core.ModelID) error {
	var opts []core.ConversationOption
	if a.chatSystem != "" {
		opts = append(opts, core.WithSystemMessage(a.chatSystem))
	}
	conv := core.NewConversation(ctx, client, model, opts...)

	fmt.Fprintf(a.stderr, "Iris REPL — provider=%s model=%s (Ctrl-D or Ctrl-C to exit)\n", a.provider, a.model)

	reader := bufio.NewReader(a.stdin)
	for {
		if ctx.Err() != nil {
			fmt.Fprintln(a.stderr, "Interrupted.")
			return nil
		}

		fmt.Fprint(a.stdout, "> ")
		line, err := reader.ReadString('\n')
		text := strings.TrimSpace(line)

		if err == io.EOF {
			if text != "" {
				if e := a.runREPLTurn(ctx, conv, text); e != nil {
					return e
				}
			}
			fmt.Fprintln(a.stdout)
			return nil
		}
		if err != nil {
			return exitWithCode(ExitValidation, fmt.Errorf("failed to read input: %w", err))
		}

		if text == "" {
			continue
		}
		if e := a.runREPLTurn(ctx, conv, text); e != nil {
			return e
		}
	}
}

// runREPLTurn sends a single user turn and prints the result. Recoverable
// errors are reported inline and the REPL continues; context cancellation
// returns nil so the caller can exit cleanly.
func (a *App) runREPLTurn(ctx context.Context, conv *core.Conversation, text string) error {
	if a.chatStream {
		stream, err := conv.Stream(ctx, text)
		if err != nil {
			return a.reportREPLError(err)
		}
		finalResp, streamErr := a.drainStream(stream)
		if streamErr != nil {
			return a.reportREPLError(streamErr)
		}
		if a.verbose && finalResp != nil {
			a.printUsage(finalResp.Usage)
		}
		return nil
	}

	resp, err := conv.Send(ctx, text)
	if err != nil {
		return a.reportREPLError(err)
	}
	fmt.Fprintln(a.stdout, resp.Output)
	if a.verbose {
		a.printUsage(resp.Usage)
	}
	return nil
}

// reportREPLError prints an error for an in-REPL turn and returns nil so the
// loop continues. Context cancellation is swallowed here; the REPL loop's
// ctx.Err() check reports "Interrupted." and exits on the next iteration.
func (a *App) reportREPLError(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	_ = a.handleChatError(err) // print classified error
	return nil
}

func (a *App) printUsage(u core.TokenUsage) {
	fmt.Fprintf(a.stderr, "Usage: %d prompt + %d completion = %d total tokens\n",
		u.PromptTokens, u.CompletionTokens, u.TotalTokens)
}

// loadSchema reads a JSON Schema from path and returns a definition suitable
// for ResponseJSONSchema(ResponseJSONSchemaNonStrict). The schema name is
// derived from the file's base name (without extension).
func loadSchema(path string) (*core.JSONSchemaDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", path, err)
	}
	// Validate that the file is valid JSON before sending it to a provider.
	var node json.RawMessage
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("schema file %s is not valid JSON: %w", path, err)
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "" {
		name = "output"
	}
	return &core.JSONSchemaDefinition{
		Name:   name,
		Schema: json.RawMessage(data),
	}, nil
}

func (a *App) handleChatError(err error) error {
	// User-initiated cancellation: clean exit, no error envelope.
	if errors.Is(err, context.Canceled) {
		if !a.jsonOutput {
			fmt.Fprintln(a.stderr, "Interrupted.")
		}
		return nil
	}

	// Iris-imposed execution timeout.
	if errors.Is(err, core.ErrTimeout) {
		if a.jsonOutput {
			a.outputSimpleErrorJSON("timeout", err.Error())
		} else {
			fmt.Fprintf(a.stderr, "Error: %v\n", err)
		}
		return exitWithCode(ExitTimeout, err)
	}

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

	// Validation errors (request-level, before any HTTP call).
	if errors.Is(err, core.ErrModelRequired) || errors.Is(err, core.ErrNoMessages) ||
		errors.Is(err, core.ErrInvalidSchema) || errors.Is(err, core.ErrStructuredOutputUnsupported) ||
		errors.Is(err, core.ErrSearchUnsupported) {
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
