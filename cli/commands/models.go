package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/petal-labs/iris/core"
)

func (a *App) newModelsCommand() *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "List models available for a provider",
		Long: `List models available for a provider from its static catalog.

The provider is selected with the global --provider flag (or the
default_provider in config). Output is a table by default; pass --json for
machine-readable output.

Examples:
  iris models --provider openai
  iris models --provider ollama --json`,
		RunE: a.runModels,
	}

	return modelsCmd
}

func (a *App) runModels(cmd *cobra.Command, args []string) error {
	// The model catalog is static, so a missing API key is not fatal here:
	// the provider is constructed with an empty key solely to read its
	// catalog. Keyed providers that need a live listing should expose that
	// through their own discovery API.
	client, err := a.resolveClient(false, false)
	if err != nil {
		return err
	}

	models := client.Provider().Models()

	if a.jsonOutput {
		return a.outputModelsJSON(models)
	}

	if len(models) == 0 {
		fmt.Fprintf(a.stdout, "No models in the %s catalog.\n", a.provider)
		return nil
	}

	fmt.Fprintf(a.stdout, "Models for %s (%d):\n\n", a.provider, len(models))
	w := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDISPLAY NAME\tCAPABILITIES")
	for _, m := range models {
		fmt.Fprintf(w, "%s\t%s\t%s\n", m.ID, m.DisplayName, capabilitiesString(m.Capabilities))
	}
	return w.Flush()
}

func (a *App) outputModelsJSON(models []core.ModelInfo) error {
	out := make([]map[string]interface{}, 0, len(models))
	for _, m := range models {
		caps := make([]string, 0, len(m.Capabilities))
		for _, c := range m.Capabilities {
			caps = append(caps, string(c))
		}
		out = append(out, map[string]interface{}{
			"id":           string(m.ID),
			"display_name": m.DisplayName,
			"capabilities": caps,
		})
	}

	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func capabilitiesString(caps []core.Feature) string {
	if len(caps) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(caps))
	for _, c := range caps {
		parts = append(parts, string(c))
	}
	return strings.Join(parts, ", ")
}
