// Example: ElevenLabs Speech Engine Upstream WebSocket Server
//
// ElevenLabs Speech Engine reverses the WebSocket direction: ElevenLabs dials
// outbound to your server's publicly reachable wsUrl, handles audio capture,
// STT, turn-taking, and TTS, and streams user_transcript frames while expecting
// agent_response frames back.
//
// This example mounts elevenlabs.SpeechEngineHandler (an http.Handler) backed
// by any Iris LLM provider (OpenAI by default), with built-in HS256 JWT
// verification of X-Elevenlabs-Speech-Engine-Authorization.
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	export OPENAI_API_KEY=your-key
//	go run ./examples/voice/speech-engine -addr :8080 -path /speech-engine
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
	"github.com/petal-labs/iris/providers/openai"
)

type speechEngineExampleConfig struct {
	Addr         string
	Path         string
	Model        core.ModelID
	SystemPrompt string
}

func buildSpeechEngineMux(el *elevenlabs.ElevenLabs, llm core.Provider, cfg speechEngineExampleConfig) (*http.ServeMux, error) {
	if el == nil {
		return nil, fmt.Errorf("elevenlabs provider cannot be nil")
	}
	handler, err := el.NewSpeechEngineHandler(elevenlabs.SpeechEngineConfig{
		LLM:          llm,
		Model:        cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("create speech engine handler: %w", err)
	}

	path := cfg.Path
	if path == "" {
		path = "/speech-engine"
	}

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux, nil
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	path := flag.String("path", "/speech-engine", "WebSocket endpoint path for ElevenLabs Speech Engine wsUrl")
	model := flag.String("model", string(openai.ModelGPT4oMini), "OpenAI LLM model ID")
	systemPrompt := flag.String("system", "You are a concise, helpful voice assistant.", "System prompt for the LLM")
	flag.Parse()

	el, err := elevenlabs.NewFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	llm, err := openai.NewFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	mux, err := buildSpeechEngineMux(el, llm, speechEngineExampleConfig{
		Addr:         *addr,
		Path:         *path,
		Model:        core.ModelID(*model),
		SystemPrompt: *systemPrompt,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("ElevenLabs Speech Engine upstream server listening on %s (ws path=%s, model=%s)\n", *addr, *path, *model)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
