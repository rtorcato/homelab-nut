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

func init() { Register(nutServer{}) }

// nutServer implements Role by piping the existing setup-server.sh
// over SSH. The wrap-then-port plan: this layer ships in v0.1 so the
// CLI works end-to-end; #7 (Phase 6) replaces this with a native Go
// implementation that handles apt/systemd/templating in-process.
type nutServer struct{}

// Name is the canonical role string used in inventory.yaml.
func (nutServer) Name() string { return string(inventory.RoleNUTServer) }

// Applies is the standard one-liner: the role runs on hosts that
// declare it in their roles list.
func (nutServer) Applies(h *inventory.Host) bool {
	return h != nil && h.HasRole(inventory.RoleNUTServer)
}

// detectCmd is the remote shell snippet Detect runs. Lifted to a
// package-level constant so unit tests can verify its construction
// without an SSH connection.
const detectCmd = `set -e
if command -v upsd >/dev/null 2>&1 && systemctl is-active --quiet nut-server; then
    echo OK
elif command -v upsd >/dev/null 2>&1; then
    echo PARTIAL
else
    echo MISSING
fi`

// Detect runs detectCmd on the remote and maps the result to State.
// A nil connection yields StateUnknown without error (a no-op detect
// for the planning UI before we've actually opened the SSH tunnel).
func (nutServer) Detect(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (State, error) {
	if conn == nil {
		return StateUnknown, nil
	}
	res, err := conn.Run(ctx, detectCmd)
	if err != nil {
		return StateUnknown, fmt.Errorf("detect nut-server on %s: %w", h.Name, err)
	}
	switch strings.TrimSpace(res.Stdout) {
	case "OK":
		return StateOK, nil
	case "PARTIAL":
		return StatePartial, nil
	case "MISSING":
		return StateMissing, nil
	default:
		return StateUnknown, fmt.Errorf("nut-server detect: unexpected output %q (stderr: %s)", res.Stdout, res.Stderr)
	}
}

// Plan validates the inventory entry, detects current state, and
// builds a Diff. Validation errors here surface long before Apply
// touches the remote — keep them user-actionable.
func (r nutServer) Plan(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (*Diff, error) {
	if h.UPS == nil || h.UPS.Name == "" || h.UPS.Driver == "" {
		return nil, fmt.Errorf("nut-server on %s needs ups.name and ups.driver in inventory.yaml", h.Name)
	}
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
	d.Actions = []string{
		"install nut-server, nut-driver, nut-client, upsmon (apt)",
		fmt.Sprintf("configure ups.conf for UPS %q (driver %s, auto-detected via nut-scanner when possible)", h.UPS.Name, h.UPS.Driver),
		"generate /root/nut-credentials.txt + upsd.users (random passwords)",
		"enable + start nut-server, nut-driver, nut-client systemd units",
	}
	return d, nil
}

// Apply pipes the embedded setup-server.sh through SSH and runs it as
// root. Output is streamed verbatim to out so the CLI/TUI can show
// progress in real time.
func (r nutServer) Apply(ctx context.Context, conn *ssh.Connection, h *inventory.Host, out io.Writer) error {
	if conn == nil {
		return errors.New("nut-server apply: nil connection")
	}
	if h.UPS == nil || h.UPS.Name == "" || h.UPS.Driver == "" {
		return fmt.Errorf("nut-server on %s needs ups.name and ups.driver", h.Name)
	}

	script, err := readScript("setup-server.sh")
	if err != nil {
		return err
	}

	// Send positional args via `sudo bash -s -- <ups-name> <driver>`.
	// Shell-quoting via shellescape is overkill for these constrained
	// values; we validate them are non-empty and free of whitespace
	// already in inventory.Validate, so simple double-quote is enough.
	cmd := fmt.Sprintf(`sudo bash -s -- %q %q`, h.UPS.Name, h.UPS.Driver)

	if out == nil {
		out = io.Discard
	}
	return conn.Pipe(ctx, bytes.NewReader(script), cmd, out, out)
}

// Uninstall removes the upstream NUT server — but only with PurgeNUT, since
// apt-purging nut + deleting /etc/nut (and the generated credentials file)
// is destructive and irreversible. Without it this is a no-op that reports
// what --purge-nut would remove, so an operator sees the gate clearly.
func (nutServer) Uninstall(ctx context.Context, conn *ssh.Connection, h *inventory.Host, p UninstallParams, out io.Writer) (*Removal, error) {
	if !p.PurgeNUT {
		return &Removal{
			Role:    "nut-server",
			Skipped: []string{"upstream NUT package + /etc/nut (pass --purge-nut to remove)"},
		}, nil
	}
	return removeArtifacts(ctx, conn, "nut-server", out,
		nil,
		[]string{"/etc/nut", "/root/nut-credentials.txt"},
		[]string{"nut-server", "nut"})
}
