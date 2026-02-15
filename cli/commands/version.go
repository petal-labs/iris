package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information set at build time via ldflags.
// Example: go build -ldflags "-X github.com/petal-labs/iris/cli/commands.Version=v1.0.0"
var (
	// Version is the semantic version of the CLI.
	Version = "dev"
	// Commit is the git commit hash.
	Commit = "unknown"
	// BuildDate is the date when the binary was built.
	BuildDate = "unknown"
)

func (a *App) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Print detailed version information including version, commit, build date, and Go runtime.`,
		Run: func(cmd *cobra.Command, args []string) {
			if a.jsonOutput {
				fmt.Fprintf(a.stdout, `{"version":"%s","commit":"%s","buildDate":"%s","goVersion":"%s","platform":"%s/%s"}`+"\n",
					Version, Commit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
				return
			}

			fmt.Fprintf(a.stdout, "iris %s\n", Version)
			fmt.Fprintf(a.stdout, "  commit:     %s\n", Commit)
			fmt.Fprintf(a.stdout, "  built:      %s\n", BuildDate)
			fmt.Fprintf(a.stdout, "  go version: %s\n", runtime.Version())
			fmt.Fprintf(a.stdout, "  platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
