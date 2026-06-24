// Command homelab-nut is a CLI + TUI for setting up and operating
// Network UPS Tools (NUT) across a homelab fleet from a single laptop.
//
// Run with no arguments to open the interactive TUI, or use subcommands
// (init, plan, apply, status, ...) for scripted use.
package main

import (
	"fmt"
	"os"

	"github.com/rtorcato/homelab-nut/internal/cli"
)

// Build-time variables. Populated by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	err := cli.Execute(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
	if err == nil {
		return
	}
	// Empty Error() = the command already printed a formatted message.
	if msg := err.Error(); msg != "" {
		fmt.Fprintf(os.Stderr, "homelab-nut: %s\n", msg)
	}
	os.Exit(cli.ExitCode(err))
}
