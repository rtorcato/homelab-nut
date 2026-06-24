package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

// defaultLogUnit is the shutdown daemon's systemd unit — the unit an
// operator most often needs when something misbehaves, so it's the
// default when --unit isn't given. Matches setup-shutdown-daemon.sh.
const defaultLogUnit = "ups-battery-shutdown"

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <host>",
		Short: "Tail a host's systemd journal over SSH",
		Long: `Connects to a host over SSH and streams its systemd journal for a NUT
unit — by default the battery-shutdown daemon (ups-battery-shutdown), the
service you most often need when a shutdown misbehaves.

Pass --unit to read any other unit (e.g. nut-server, nut-monitor), --lines
to change how much history is shown, and --follow to stream new entries
live until Ctrl+C.

This is raw journal text, so there's no -o json mode.`,
		Example: `  homelab-nut logs pi
  homelab-nut logs pi --unit nut-server --lines 500
  homelab-nut logs pi --follow`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			unit, _ := cmd.Flags().GetString("unit")
			lines, _ := cmd.Flags().GetInt("lines")
			follow, _ := cmd.Flags().GetBool("follow")
			return runLogs(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				inventoryPath(cmd), args[0], unit, lines, follow)
		},
	}
	cmd.Flags().String("unit", defaultLogUnit, "systemd unit to read")
	cmd.Flags().IntP("lines", "n", 200, "number of past journal lines to show")
	cmd.Flags().BoolP("follow", "f", false, "stream new entries live until Ctrl+C")
	return cmd
}

// runLogs resolves host in the inventory, opens an SSH connection, and
// streams journalctl output back to the caller. In --follow mode it wires
// a signal-aware context so Ctrl+C propagates SIGTERM to the remote
// journalctl and exits cleanly.
func runLogs(parent context.Context, stdout, stderr io.Writer, path, hostArg, unit string, lines int, follow bool) error {
	if parent == nil {
		parent = context.Background()
	}
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}
	h := inv.HostByName(hostArg)
	if h == nil {
		return fmt.Errorf("host %q not found in inventory", hostArg)
	}

	ctx := parent
	if follow {
		// Ctrl+C (and SIGTERM) cancels ctx; Connection.Stream turns that
		// into a SIGTERM on the remote journalctl, then returns context
		// cancellation, which we treat as a clean exit below.
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
	}

	executor := ssh.NewExecutor(ssh.NewConfig())
	defer func() { _ = executor.Close() }()

	conn, err := executor.Open(h)
	if err != nil {
		return fmt.Errorf("ssh %s: %w", h.Name, err)
	}
	defer func() { _ = conn.Close() }()

	err = conn.Stream(ctx, journalctlCmd(unit, lines, follow), stdout, stderr)
	if follow && errors.Is(err, context.Canceled) {
		// Ctrl+C in --follow is the normal way to stop — not an error.
		fmt.Fprintln(stdout)
		return nil
	}
	return err
}

// journalctlCmd builds the remote journalctl invocation. --no-pager keeps
// it non-interactive over SSH; the unit is single-quoted so an exotic unit
// name can't break out of the command. -n is omitted for non-positive
// line counts (journalctl then uses its own default).
func journalctlCmd(unit string, lines int, follow bool) string {
	parts := []string{"journalctl", "--no-pager", "-u", shellQuote(unit)}
	if lines > 0 {
		parts = append(parts, "-n", strconv.Itoa(lines))
	}
	if follow {
		parts = append(parts, "-f")
	}
	return strings.Join(parts, " ")
}

// shellQuote wraps s in single quotes, escaping any embedded single quote,
// so it survives the remote shell as a single literal argument.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
