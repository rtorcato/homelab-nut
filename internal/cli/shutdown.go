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
  - a command starting with sudo (e.g. sudo /sbin/shutdown -h now) must be
    allowed without a password (sudo -n -l <command>).

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
//
// A command that starts with sudo is checked with sudo -n -l instead:
// checking that sudo itself is in PATH passes on almost every host, while
// the real failure is a missing NOPASSWD rule (#138). sudo -l is
// non-interactive with -n and has no side effects.
func shutdownCheckCmd(command string) (remoteCmd, failMsg string) {
	fields := strings.Fields(command) // command is non-empty by caller contract
	if fields[0] == "sudo" {
		return sudoCheckCmd(fields[1:])
	}
	token := fields[0]
	if strings.Contains(token, "/") { // covers ~/ too: remoteWord expands it
		return "test -x " + remoteWord(token),
			"script not found or not executable: " + token
	}
	return "command -v " + remoteWord(token),
		"command not found in PATH: " + token
}

// sudoCheckCmd builds `sudo -n [-u user] [-g group] -l <command> [args]`
// from the words after sudo. -u and -g are kept since they change what the
// rule must permit; other flags are dropped. A flag that takes an argument
// other than -u/-g would make that argument look like the command, so the
// check fails closed rather than passing.
func sudoCheckCmd(args []string) (remoteCmd, failMsg string) {
	check := []string{"sudo", "-n"}
	i := 0
	for ; i < len(args) && strings.HasPrefix(args[i], "-"); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		switch {
		case (a == "-u" || a == "-g") && i+1 < len(args):
			check = append(check, a, shellQuote(args[i+1]))
			i++
		case strings.HasPrefix(a, "-u"), strings.HasPrefix(a, "-g"),
			strings.HasPrefix(a, "--user="), strings.HasPrefix(a, "--group="):
			check = append(check, shellQuote(a)) // attached form, e.g. -uroot
		}
	}
	if i >= len(args) {
		return "false", "sudo with no command to run"
	}
	check = append(check, "-l")
	for _, a := range args[i:] {
		check = append(check, remoteWord(a))
	}
	return strings.Join(check, " ") + " >/dev/null",
		"sudo needs a password or no rule allows: " + strings.Join(args[i:], " ")
}

// remoteWord shell-quotes one word for the remote shell, expanding a
// leading ~/ through $HOME the way the daemon's shell would.
func remoteWord(w string) string {
	if rel, ok := strings.CutPrefix(w, "~/"); ok {
		return `"$HOME"/` + shellQuote(rel)
	}
	return shellQuote(w)
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
