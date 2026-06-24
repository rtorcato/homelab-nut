package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

// shutdownTestTimeout caps connect + check time for a single host.
const shutdownTestTimeout = 15 * time.Second

// shutdownTestResult is the per-host outcome, shaped for `-o json`.
type shutdownTestResult struct {
	Host    string `json:"host"`
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

func newShutdownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Operate the fleet's shutdown chain",
		Long: `Commands for the battery-shutdown chain. Today this is just 'test', a
dry-run that verifies each target can be reached and its shutdown command
resolved — without powering anything off.`,
	}
	cmd.AddCommand(newShutdownTestCmd())
	return cmd
}

func newShutdownTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Dry-run the shutdown chain on every shutdown-target host (no power-off)",
		Long: `Verifies the shutdown chain works without powering anything off. For each
host with the shutdown-target role, this connects over SSH as the
configured user and checks that the host's shutdown command can be
resolved:

  - a script path (contains '/', e.g. ~/shutdown.sh) must exist and be
    executable (test -x);
  - an inline command (e.g. poweroff) must be found in PATH (command -v).

It prints a host / command / result table. With -o json it emits an array
of {host, command, ok, error}. Exits 3 if any host fails the check.`,
		Example: `  homelab-nut shutdown test
  homelab-nut shutdown test -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShutdownTest(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				inventoryPath(cmd), getOutputFormat(cmd))
		},
	}
	addOutputFlag(cmd)
	return cmd
}

// runShutdownTest checks every shutdown-target host and renders the result.
// Returns errExit(ExitApplyPartial) if any host fails so the documented
// non-zero exit code is emitted; per-host detail lives in each result.
func runShutdownTest(parent context.Context, stdout, stderr io.Writer, path string, format outputFormat) error {
	if parent == nil {
		parent = context.Background()
	}
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}
	targets := inv.HostsWithRole(inventory.RoleShutdownTarget)

	executor := ssh.NewExecutor(ssh.NewConfig())
	defer func() { _ = executor.Close() }()

	results := make([]shutdownTestResult, 0, len(targets))
	for _, h := range targets {
		results = append(results, checkShutdownTarget(parent, executor, h))
	}

	if format == outputJSON {
		if err := emitJSON(stdout, results); err != nil {
			return err
		}
	} else {
		printShutdownTestResults(stdout, results)
	}

	for _, r := range results {
		if !r.OK {
			return errExit(ExitApplyPartial)
		}
	}
	return nil
}

// checkShutdownTarget runs the dry-run check for one host: resolve the
// command, SSH in, and confirm the command is runnable.
func checkShutdownTarget(parent context.Context, executor *ssh.Executor, h *inventory.Host) shutdownTestResult {
	r := shutdownTestResult{Host: h.Name}
	if h.Shutdown != nil {
		r.Command = strings.TrimSpace(h.Shutdown.Command)
	}
	if r.Command == "" {
		r.Error = "no shutdown command configured"
		return r
	}

	ctx, cancel := context.WithTimeout(parent, shutdownTestTimeout)
	defer cancel()

	conn, err := executor.Open(h)
	if err != nil {
		r.Error = fmt.Sprintf("ssh: %v", err)
		return r
	}
	defer func() { _ = conn.Close() }()

	check, failMsg := shutdownCheckCmd(r.Command)
	res, err := conn.Run(ctx, check)
	if err != nil {
		r.Error = fmt.Sprintf("ssh run: %v", err)
		return r
	}
	if res.ExitCode != 0 {
		r.Error = failMsg
		return r
	}
	r.OK = true
	return r
}

// shutdownCheckCmd builds the remote check for command and the message to
// report if it fails. A command whose first token looks like a path
// (contains '/' or starts with '~/') is checked with test -x; anything
// else is treated as a PATH lookup with command -v. The first token alone
// is checked — trailing args (e.g. "shutdown -h now") don't affect whether
// the binary exists.
func shutdownCheckCmd(command string) (remoteCmd, failMsg string) {
	token := strings.Fields(command)[0] // command is non-empty by caller contract
	switch {
	case strings.HasPrefix(token, "~/"):
		// Home-relative script: expand ~ via $HOME (the daemon runs the
		// command through a shell, so this mirrors real resolution).
		rel := strings.TrimPrefix(token, "~/")
		return `test -x "$HOME"/` + shellQuote(rel),
			"script not found or not executable: " + token
	case strings.Contains(token, "/"):
		return "test -x " + shellQuote(token),
			"script not found or not executable: " + token
	default:
		return "command -v " + shellQuote(token),
			"command not found in PATH: " + token
	}
}

func printShutdownTestResults(w io.Writer, results []shutdownTestResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No hosts with the shutdown-target role in the inventory.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tCOMMAND\tRESULT")
	failed := 0
	for _, r := range results {
		result := "OK"
		if !r.OK {
			failed++
			result = r.Error
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Host, dash(r.Command), result)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\n%d host(s) checked, %d failed. No machines were powered off.\n",
		len(results), failed)
}
