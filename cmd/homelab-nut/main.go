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
	if err := cli.Execute(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
