package roles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

func init() { Register(shutdownTarget{}) }

// shutdownTarget configures a host to receive a graceful shutdown
// command from the shutdown-daemon when battery hits the threshold.
//
// Two modes, driven by the host's shutdown.command in inventory:
//   - Script mode (default, when command starts with `~/` or ends in
//     `.sh`): installs the embedded shutdown.sh as ~/shutdown.sh,
//     chmods it 700, and configures passwordless sudo for /sbin/shutdown.
//   - Inline mode (e.g., `poweroff` for UniFi devices that wipe ~/ on
//     firmware updates): only configures the sudoers rule. The daemon
//     sends the command inline at trigger time.
type shutdownTarget struct{}

func (shutdownTarget) Name() string { return string(inventory.RoleShutdownTarget) }

func (shutdownTarget) Applies(h *inventory.Host) bool {
	return h != nil && h.HasRole(inventory.RoleShutdownTarget)
}

// isScriptCommand reports whether cmd looks like a path to a script
// (vs an inline command like `poweroff`). A script needs deploying;
// an inline command is sent over SSH at trigger time.
func isScriptCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		// Empty defaults to script mode with the canonical ~/shutdown.sh
		return true
	}
	return strings.HasPrefix(cmd, "~/") ||
		strings.HasPrefix(cmd, "./") ||
		strings.HasPrefix(cmd, "/") ||
		strings.HasSuffix(cmd, ".sh")
}

// resolvedMode returns "script" or "inline" for the host's shutdown
// configuration. Lifted into a method so tests + Plan + Apply share
// the same decision.
func (shutdownTarget) resolvedMode(h *inventory.Host) string {
	if h == nil || h.Shutdown == nil {
		return "script"
	}
	if isScriptCommand(h.Shutdown.Command) {
		return "script"
	}
	return "inline"
}

// shutdownTargetDetectCmd checks for both halves of the install:
// the script (if present) and the sudoers rule.
const shutdownTargetDetectCmd = `set -e
SUDOERS_OK=0
SCRIPT_OK=0
if [ -f /etc/sudoers.d/ups-shutdown ]; then SUDOERS_OK=1; fi
if [ -x "$HOME/shutdown.sh" ]; then SCRIPT_OK=1; fi
if [ "$SUDOERS_OK" = 1 ] && [ "$SCRIPT_OK" = 1 ]; then
    echo OK
elif [ "$SUDOERS_OK" = 1 ] || [ "$SCRIPT_OK" = 1 ]; then
    echo PARTIAL
else
    echo MISSING
fi`

// shutdownTargetDetectInlineCmd is the inline-mode equivalent — we
// only care about the sudoers rule. ~/shutdown.sh isn't expected.
const shutdownTargetDetectInlineCmd = `set -e
if [ -f /etc/sudoers.d/ups-shutdown ]; then echo OK; else echo MISSING; fi`

func (r shutdownTarget) Detect(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (State, error) {
	if conn == nil {
		return StateUnknown, nil
	}
	cmd := shutdownTargetDetectCmd
	if r.resolvedMode(h) == "inline" {
		cmd = shutdownTargetDetectInlineCmd
	}
	res, err := conn.Run(ctx, cmd)
	if err != nil {
		return StateUnknown, fmt.Errorf("detect shutdown-target on %s: %w", h.Name, err)
	}
	switch strings.TrimSpace(res.Stdout) {
	case "OK":
		return StateOK, nil
	case "PARTIAL":
		return StatePartial, nil
	case "MISSING":
		return StateMissing, nil
	default:
		return StateUnknown, fmt.Errorf("shutdown-target detect: unexpected output %q (stderr: %s)", res.Stdout, res.Stderr)
	}
}

func (r shutdownTarget) Plan(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (*Diff, error) {
	mode := r.resolvedMode(h)
	current, err := r.Detect(ctx, conn, h)
	if err != nil {
		return nil, err
	}
	d := &Diff{
		Host:    h,
		Role:    r.Name(),
		Current: current,
		Target:  StateOK,
	}
	if current == StateOK {
		return d, nil
	}
	if mode == "script" {
		d.Actions = []string{
			fmt.Sprintf("install /home/%s/shutdown.sh (chmod 700, chown %s)", h.User, h.User),
			fmt.Sprintf("configure /etc/sudoers.d/ups-shutdown for passwordless `/sbin/shutdown` as %s", h.User),
		}
	} else {
		d.Actions = []string{
			fmt.Sprintf("configure /etc/sudoers.d/ups-shutdown for passwordless `/sbin/shutdown` as %s", h.User),
			fmt.Sprintf("(inline mode — daemon will send `%s` over SSH at trigger time, no script deployed)", h.Shutdown.Command),
		}
	}
	return d, nil
}

func (r shutdownTarget) Apply(ctx context.Context, conn *ssh.Connection, h *inventory.Host, out io.Writer) error {
	if conn == nil {
		return errors.New("shutdown-target apply: nil connection")
	}
	if h.User == "" {
		return fmt.Errorf("shutdown-target apply on %s: host.user is required", h.Name)
	}
	if out == nil {
		out = io.Discard
	}

	mode := r.resolvedMode(h)

	// Step 1: install shutdown.sh (script mode only)
	if mode == "script" {
		script, err := readScript("shutdown.sh")
		if err != nil {
			return err
		}
		installCmd := fmt.Sprintf(
			`sudo tee /home/%[1]s/shutdown.sh > /dev/null && sudo chmod 700 /home/%[1]s/shutdown.sh && sudo chown %[1]s:%[1]s /home/%[1]s/shutdown.sh`,
			h.User,
		)
		if err := conn.Pipe(ctx, bytes.NewReader(script), installCmd, out, out); err != nil {
			return fmt.Errorf("shutdown-target apply on %s: install shutdown.sh: %w", h.Name, err)
		}
		fmt.Fprintf(out, "installed /home/%s/shutdown.sh\n", h.User)
	}

	// Step 2: configure sudoers, then visudo-validate to avoid lockout
	sudoersCmd := fmt.Sprintf(
		`echo '%[1]s ALL=(ALL) NOPASSWD: /sbin/shutdown' | sudo tee /etc/sudoers.d/ups-shutdown > /dev/null && sudo chmod 440 /etc/sudoers.d/ups-shutdown && sudo visudo -c -f /etc/sudoers.d/ups-shutdown`,
		h.User,
	)
	if err := conn.Stream(ctx, sudoersCmd, out, out); err != nil {
		return fmt.Errorf("shutdown-target apply on %s: configure sudoers: %w", h.Name, err)
	}
	fmt.Fprintf(out, "configured /etc/sudoers.d/ups-shutdown for %s\n", h.User)

	return nil
}
