// Example: Hosted ElevenAgents Voice Session and Signed URL Minting
//
// This example demonstrates connecting to a hosted ElevenLabs Conversational AI
// agent (ElevenAgents) through Iris's provider-agnostic core.VoiceSession API:
//   - Default mode: opens a hosted voice session via client.StartVoiceSession
//     (automatically minting a short-lived signed URL for private agents),
//     sends a text prompt, streams typed VoiceEvent transcripts and audio
//     chunks to disk, and closes cleanly.
//   - Signed-URL mode (-signed-url-only): mints a short-lived signed URL on a
//     trusted backend via provider.GetAgentSignedURL without printing raw
//     credential tokens.
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	export ELEVENLABS_AGENT_ID=agent_...
//	go run ./examples/voice/elevenagents -prompt "Hello! What can you help with?" -out agent.pcm
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/elevenlabs"
)

type elevenAgentsExampleConfig struct {
	AgentID       string
	Prompt        string
	OutPath       string
	SignedURLOnly bool
}

func runElevenAgents(ctx context.Context, provider *elevenlabs.ElevenLabs, cfg elevenAgentsExampleConfig, stdout io.Writer) error {
	if provider == nil {
		return fmt.Errorf("elevenlabs provider cannot be nil")
	}
	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		return fmt.Errorf("agent ID is required (pass -agent or set ELEVENLABS_AGENT_ID)")
	}

	if cfg.SignedURLOnly {
		signed, err := provider.GetAgentSignedURL(ctx, &elevenlabs.AgentSignedURLRequest{
			AgentID: agentID,
		})
		if err != nil {
			return fmt.Errorf("mint agent signed URL: %w", err)
		}
		// Never print raw signed URL tokens to logs; signed.String() is redacted.
		fmt.Fprintf(stdout, "Minted signed URL for agent %s (redacted=%s, non_empty=%t)\n",
			agentID, signed.String(), !signed.SignedURL.IsEmpty())
		return nil
	}

	client := core.NewClient(provider)
	sess, err := client.StartVoiceSession(ctx, core.VoiceSessionConfig{
		AgentID: agentID,
	})
	if err != nil {
		return fmt.Errorf("start ElevenAgents session: %w", err)
	}
	defer func() { _ = sess.Close() }()

	return consumeSingleAgentTurn(ctx, sess, cfg.Prompt, cfg.OutPath, stdout)
}

func consumeSingleAgentTurn(ctx context.Context, sess core.VoiceSession, prompt, outPath string, stdout io.Writer) error {
	if strings.TrimSpace(prompt) != "" {
		if err := sess.SendText(ctx, prompt); err != nil {
			return fmt.Errorf("send text to agent: %w", err)
		}
	}

	outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", outPath, err)
	}
	defer outFile.Close()

	tracker := core.NewVoiceTurnTracker("consumer")
	var (
		audioBytes    int64
		assistantText string
		gotResponse   bool
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-sess.Events():
			if !ok {
				if err := <-sess.Err(); err != nil {
					return fmt.Errorf("agent session failed: %w", err)
				}
				if gotResponse {
					fmt.Fprintf(stdout, "Agent responded (audio_bytes=%d, out=%s)\n", audioBytes, outPath)
					return nil
				}
				return fmt.Errorf("agent session closed before response completed")
			}
			tracker.ObserveEvent(evt)

			switch evt.Type {
			case core.VoiceEventAgentTextCommitted:
				assistantText = evt.Text
				gotResponse = true
			case core.VoiceEventAudioOutput:
				if evt.Audio != nil && len(evt.Audio.Data) > 0 && tracker.ShouldPlayAudio(evt) {
					n, wErr := outFile.Write(evt.Audio.Data)
					if wErr != nil {
						return fmt.Errorf("write agent audio: %w", wErr)
					}
					audioBytes += int64(n)
					gotResponse = true
				}
			case core.VoiceEventTurnCompleted:
				fmt.Fprintf(stdout, "Turn %s completed (audio_bytes=%d, out=%s)\n", evt.TurnID, audioBytes, outPath)
				if assistantText != "" {
					fmt.Fprintf(stdout, "Agent: %s\n", assistantText)
				}
				return nil
			case core.VoiceEventError:
				if evt.Err != nil {
					return fmt.Errorf("agent error: %w", evt.Err)
				}
			}
		}
	}
}

func resolveAgentID(flagAgent string) string {
	if v := strings.TrimSpace(flagAgent); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("ELEVENLABS_AGENT_ID"))
}

func main() {
	agentID := flag.String("agent", "", "ElevenLabs Agent ID (defaults to ELEVENLABS_AGENT_ID)")
	prompt := flag.String("prompt", "Hello! Give me a brief greeting.", "Initial text message sent to the hosted agent")
	outPath := flag.String("out", "agent.pcm", "Output audio file path for agent audio chunks")
	signedURLOnly := flag.Bool("signed-url-only", false, "Only mint a short-lived signed URL via GetAgentSignedURL and exit")
	flag.Parse()

	provider, err := elevenlabs.NewFromEnv(elevenlabs.WithTimeout(30 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := elevenAgentsExampleConfig{
		AgentID:       resolveAgentID(*agentID),
		Prompt:        *prompt,
		OutPath:       *outPath,
		SignedURLOnly: *signedURLOnly,
	}
	if err := runElevenAgents(ctx, provider, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
