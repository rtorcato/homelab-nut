// Package cli wires the Cobra command tree for homelab-nut.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtorcato/homelab-nut/internal/inventory"
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
		// Default action when no subcommand: open the TUI in a loop
		// that dispatches on the user's exit action (init / edit / quit).
		// init and edit suspend the TUI, run the relevant flow, then
		// relaunch the TUI so the user lands back where they started.
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			return runTUILoop(cmd, info.Version, path)
		},
	}

	cmd.PersistentFlags().StringP("inventory", "i", defaultInventoryPath,
		"path to the inventory YAML file")

	cmd.SetVersionTemplate("{{ .Version }}\n")
	cmd.AddCommand(newVersionCmd(info))
	cmd.AddCommand(newInventoryCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newApplyCmd())

	return cmd
}

// commandContext returns the context Cobra commands use for orchestrator
// calls. Today it's just context.Background; future work will plumb the
// cobra Context (which respects ctrl+C / SIGTERM) through here.
func commandContext() context.Context {
	return context.Background()
}

// runTUILoop launches the TUI and dispatches on its exit action. When
// the user presses 'i' (init) or 'e' (edit), the TUI quits with that
// action set; we run the corresponding flow (huh forms for init,
// $EDITOR for edit) and then relaunch the TUI with the updated
// inventory. A normal quit ('q'/ctrl+c) breaks the loop.
func runTUILoop(cmd *cobra.Command, version, path string) error {
	for {
		finalModel, err := tea.NewProgram(tui.New(version, path), tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		switch tui.ExitAction(finalModel) {
		case "init":
			if err := runInit(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), path); err != nil {
				return err
			}
		case "edit":
			if err := openEditor(path); err != nil {
				// Edit failures shouldn't kill the TUI loop — show the
				// error and relaunch so the user can try again.
				fmt.Fprintf(cmd.ErrOrStderr(), "edit failed: %v\n", err)
			}
		default:
			return nil
		}
	}
}

// openEditor opens $EDITOR (default vi) on path, then re-loads +
// validates the inventory. Mirrors the body of `inventory edit` so
// the TUI 'e' shortcut behaves identically.
func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	ed := exec.Command(editor, path)
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ed.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}
	if _, err := inventory.Load(path); err != nil {
		return err
	}
	return nil
}
