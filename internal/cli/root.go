// Package cli wires the Cobra command tree for homelab-nut.
package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtorcato/homelab-nut/internal/tui"
	"github.com/spf13/cobra"
)

// BuildInfo is populated at link time and surfaced through `version`.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// Execute builds the command tree and runs it.
func Execute(info BuildInfo) error {
	root := newRootCmd(info)
	return root.Execute()
}

// NewRootForDocs exposes the constructed Cobra tree to the docs generator
// (cmd/gen-docs). Not intended for runtime use — Execute is the entry point.
func NewRootForDocs(info BuildInfo) *cobra.Command {
	return newRootCmd(info)
}

func newRootCmd(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "homelab-nut",
		Short: "Modern CLI + TUI for Network UPS Tools (NUT)",
		Long: `homelab-nut sets up and operates a NUT-based UPS fleet from your laptop.

Run with no arguments to open the interactive TUI. Subcommands like
` + "`init`, `plan`, `apply`, `status`" + ` are exposed for scripted use.

See https://github.com/rtorcato/homelab-nut/blob/main/ROADMAP.md`,
		Version:       info.Version,
		SilenceUsage:  true,
		SilenceErrors: true, // main.go owns error printing
		// Default action when no subcommand: open the TUI.
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			_, err := tea.NewProgram(tui.New(info.Version, path), tea.WithAltScreen()).Run()
			return err
		},
	}

	cmd.PersistentFlags().StringP("inventory", "i", defaultInventoryPath,
		"path to the inventory YAML file")

	cmd.SetVersionTemplate("{{ .Version }}\n")
	cmd.AddCommand(newVersionCmd(info))
	cmd.AddCommand(newInventoryCmd())
	cmd.AddCommand(newInitCmd())

	return cmd
}
