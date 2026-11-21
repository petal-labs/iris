// Example: ElevenLabs Voice Discovery and Settings (Read-Only)
//
// This example demonstrates discovering available ElevenLabs voices via
// (*elevenlabs.ElevenLabs).ListVoices and inspecting a voice's default
// synthesis settings via GetVoiceSettings.
//
// Voice discovery is always read-only. Mutating operations (CreateVoice,
// EditVoice, EditVoiceSettings, DeleteVoice) remain disabled unless explicitly
// enabled with elevenlabs.WithVoiceManagementEnabled().
//
// Run with:
//
//	export ELEVENLABS_API_KEY=your-key
//	go run ./examples/speech/elevenlabs-voices -category premade -limit 10
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/petal-labs/iris/providers/elevenlabs"
)

type voiceDiscoveryConfig struct {
	Search   string
	Category string
	VoiceID  string
	Limit    int
}

func runVoiceDiscovery(ctx context.Context, provider *elevenlabs.ElevenLabs, cfg voiceDiscoveryConfig, stdout io.Writer) error {
	if provider == nil {
		return fmt.Errorf("elevenlabs provider cannot be nil")
	}

	if id := strings.TrimSpace(cfg.VoiceID); id != "" {
		voice, err := provider.GetVoice(ctx, id)
		if err != nil {
			return fmt.Errorf("get voice %q: %w", id, err)
		}
		settings, err := provider.GetVoiceSettings(ctx, id)
		if err != nil {
			return fmt.Errorf("get voice settings %q: %w", id, err)
		}
		fmt.Fprintf(stdout, "Voice: %s (%s) category=%s\n", voice.Name, voice.VoiceID, voice.Category)
		if settings != nil {
			fmt.Fprintf(stdout, "Settings: stability=%v similarity_boost=%v style=%v speed=%v\n",
				formatOptFloat(settings.Stability),
				formatOptFloat(settings.SimilarityBoost),
				formatOptFloat(settings.Style),
				formatOptFloat(settings.Speed),
			)
		}
		return nil
	}

	pageSize := cfg.Limit
	if pageSize <= 0 {
		pageSize = 10
	}
	page, err := provider.ListVoices(ctx, &elevenlabs.VoiceListRequest{
		Search:   strings.TrimSpace(cfg.Search),
		Category: strings.TrimSpace(cfg.Category),
		PageSize: pageSize,
	})
	if err != nil {
		return fmt.Errorf("list voices: %w", err)
	}

	fmt.Fprintf(stdout, "Found %d voice(s) (has_more=%t):\n", len(page.Voices), page.HasMore)
	for _, v := range page.Voices {
		fmt.Fprintf(stdout, "  - %s  (%s)  [%s]\n", v.Name, v.VoiceID, v.Category)
	}
	return nil
}

func formatOptFloat(v *float64) string {
	if v == nil {
		return "default"
	}
	return fmt.Sprintf("%.2f", *v)
}

func main() {
	search := flag.String("search", "", "Optional case-insensitive voice name search filter")
	category := flag.String("category", "premade", "Optional voice category filter (e.g., premade, cloned, generated)")
	voiceID := flag.String("voice", "", "Inspect a single voice ID and its synthesis settings instead of listing")
	limit := flag.Int("limit", 10, "Maximum number of voices to list (1-100)")
	flag.Parse()

	provider, err := elevenlabs.NewFromEnv(elevenlabs.WithTimeout(30 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := voiceDiscoveryConfig{
		Search:   *search,
		Category: *category,
		VoiceID:  *voiceID,
		Limit:    *limit,
	}
	if err := runVoiceDiscovery(ctx, provider, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
