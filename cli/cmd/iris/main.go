// Iris CLI - AI agent development command-line interface.
package main

import (
	"os"

	"github.com/petal-labs/iris/cli/commands"
)

// ExitCoder is an interface for errors that have an exit code.
type ExitCoder interface {
	ExitCode() int
}

func main() {
	if err := commands.Execute(); err != nil {
		if ec, ok := err.(ExitCoder); ok {
			os.Exit(ec.ExitCode())
		}
		os.Exit(1)
	}
}
