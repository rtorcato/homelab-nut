package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/forms"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate homelab-nut.yaml interactively",
		Long: `Walks through a guided form, then writes the resulting inventory.
With no --inventory/-i flag it writes ./homelab-nut.yaml when that file
already exists, otherwise ~/homelab-nut.yaml.

If an inventory already exists, you'll be asked whether to overwrite it.

This subcommand and the TUI's empty-state 'i' shortcut run the same forms —
use whichever fits your context.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := inventoryPath(cmd)
			return runInit(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), path)
		},
	}
}

func runInit(_ io.Reader, stdout, stderr io.Writer, path string) error {
	// 1. Handle existing file: confirm before clobbering.
	if _, err := os.Stat(path); err == nil {
		overwrite, err := forms.ConfirmOverwrite(path)
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintln(stdout, "Cancelled. Existing inventory left untouched.")
			return nil
		}
	}

	inv := &inventory.Inventory{}

	// 2. Add hosts in a loop. At least one is required.
	for {
		host, err := forms.AskHost(len(inv.Hosts) + 1)
		if err != nil {
			return err
		}
		inv.Hosts = append(inv.Hosts, *host)

		addAnother, err := forms.ConfirmAddAnother()
		if err != nil {
			return err
		}
		if !addAnother {
			break
		}
	}

	// 3. If any host has shutdown-daemon, collect daemon config.
	if len(inv.HostsWithRole(inventory.RoleShutdownDaemon)) > 0 {
		d, err := forms.AskShutdownDaemon()
		if err != nil {
			return err
		}
		inv.ShutdownDaemon = d
	}

	// 4. Preview + confirm.
	var preview bytes.Buffer
	if err := inv.Render(&preview); err != nil {
		return fmt.Errorf("render preview: %w", err)
	}
	fmt.Fprintln(stdout, strings.Repeat("─", 60))
	fmt.Fprintln(stdout, preview.String())
	fmt.Fprintln(stdout, strings.Repeat("─", 60))

	save, err := forms.ConfirmSave(path)
	if err != nil {
		return err
	}
	if !save {
		fmt.Fprintln(stdout, "Discarded. Nothing was written.")
		return nil
	}

	if err := inv.Save(path); err != nil {
		var vErr *inventory.ValidationError
		if errors.As(err, &vErr) {
			fmt.Fprintf(stderr, "%s\n", err)
			return errSilent
		}
		return err
	}
	fmt.Fprintf(stdout, "Wrote %s — run `homelab-nut inventory list` to confirm.\n", path)
	return nil
}
