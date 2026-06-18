package roles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

func init() { Register(nutClient{}) }

// nutClient configures a host to monitor a remote NUT server via upsmon.
// Pipes the existing scripts/setup-client.sh through SSH — same
// wrap-then-port pattern as nutServer.
type nutClient struct{}

// nutMonitorPasswordEnv is the env var Apply reads the upsmon password
// from. setup-server.sh generates this and stores it on the server in
// /root/nut-credentials.txt; auto-discovery via SSH is a Phase 6 nice-to-have.
const nutMonitorPasswordEnv = "NUT_MONITOR_PASSWORD"

func (nutClient) Name() string { return string(inventory.RoleNUTClient) }

func (nutClient) Applies(h *inventory.Host) bool {
	return h != nil && h.HasRole(inventory.RoleNUTClient)
}

// findNUTServer returns the first host in inv that has role nut-server
// and a populated ups.name. Returns nil + error suitable for Plan output.
func findNUTServer(inv *inventory.Inventory) (*inventory.Host, error) {
	if inv == nil {
		return nil, errors.New("inventory is nil")
	}
	for i := range inv.Hosts {
		h := &inv.Hosts[i]
		if h.HasRole(inventory.RoleNUTServer) && h.UPS != nil && h.UPS.Name != "" && h.Address != "" {
			return h, nil
		}
	}
	return nil, errors.New("no nut-server host with ups.name + address found in inventory")
}

// PlanContext is a small carrier so Plan can reach the full inventory
// when figuring out cross-host dependencies. The Role interface keeps
// Plan/Apply per-host; we stash the inventory on the context via a
// well-known key. Roles that need it fish it out; the rest ignore it.
type planContextKey struct{}

// WithInventory returns a context carrying inv for roles that need
// cross-host data (like nut-client looking up the nut-server host).
func WithInventory(ctx context.Context, inv *inventory.Inventory) context.Context {
	return context.WithValue(ctx, planContextKey{}, inv)
}

func inventoryFrom(ctx context.Context) *inventory.Inventory {
	if v := ctx.Value(planContextKey{}); v != nil {
		if inv, ok := v.(*inventory.Inventory); ok {
			return inv
		}
	}
	return nil
}

// nutClientDetectCmd is the shell snippet Detect runs.
const nutClientDetectCmd = `set -e
if ! command -v upsmon >/dev/null 2>&1; then
    echo MISSING
elif systemctl is-active --quiet nut-client && grep -q '^MONITOR ' /etc/nut/upsmon.conf 2>/dev/null; then
    echo OK
else
    echo PARTIAL
fi`

func (nutClient) Detect(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (State, error) {
	if conn == nil {
		return StateUnknown, nil
	}
	res, err := conn.Run(ctx, nutClientDetectCmd)
	if err != nil {
		return StateUnknown, fmt.Errorf("detect nut-client on %s: %w", h.Name, err)
	}
	switch strings.TrimSpace(res.Stdout) {
	case "OK":
		return StateOK, nil
	case "PARTIAL":
		return StatePartial, nil
	case "MISSING":
		return StateMissing, nil
	default:
		return StateUnknown, fmt.Errorf("nut-client detect: unexpected output %q (stderr: %s)", res.Stdout, res.Stderr)
	}
}

func (r nutClient) Plan(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (*Diff, error) {
	inv := inventoryFrom(ctx)
	if inv == nil {
		// No inventory in context — we can still describe a generic plan,
		// but flag that cross-host resolution will happen at Apply time.
		// In practice the apply orchestrator always provides one; this
		// branch is for ad-hoc Plan() calls (tests, future TUI previews).
		current, err := r.Detect(ctx, conn, h)
		if err != nil {
			return nil, err
		}
		return &Diff{
			Host:    h,
			Role:    r.Name(),
			Current: current,
			Target:  StateOK,
			Actions: []string{
				"resolve nut-server host from inventory",
				"install nut-client + upsmon (apt)",
				"configure /etc/nut/upsmon.conf",
				"enable + start nut-client systemd unit",
			},
		}, nil
	}

	server, err := findNUTServer(inv)
	if err != nil {
		return nil, fmt.Errorf("nut-client on %s: %w", h.Name, err)
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
		"install nut-client, nut-driver, upsmon (apt)",
		fmt.Sprintf("configure upsmon.conf for %s@%s", server.UPS.Name, server.Address),
		"enable + start nut-client systemd unit",
	}
	return d, nil
}

func (r nutClient) Apply(ctx context.Context, conn *ssh.Connection, h *inventory.Host, out io.Writer) error {
	if conn == nil {
		return errors.New("nut-client apply: nil connection")
	}

	inv := inventoryFrom(ctx)
	if inv == nil {
		return errors.New("nut-client apply: inventory missing from context (use roles.WithInventory)")
	}
	server, err := findNUTServer(inv)
	if err != nil {
		return fmt.Errorf("nut-client apply on %s: %w", h.Name, err)
	}

	password := os.Getenv(nutMonitorPasswordEnv)
	if password == "" {
		return fmt.Errorf("nut-client apply on %s: %s not set — run `sudo grep upsmon_remote /root/nut-credentials.txt` on %s and export it",
			h.Name, nutMonitorPasswordEnv, server.Name)
	}

	script, err := readScript("setup-client.sh")
	if err != nil {
		return err
	}

	// Args: <SERVER_IP> <UPS_NAME> <PASSWORD>
	cmd := fmt.Sprintf(`sudo bash -s -- %q %q %q`, server.Address, server.UPS.Name, password)

	if out == nil {
		out = io.Discard
	}
	return conn.Pipe(ctx, bytes.NewReader(script), cmd, out, out)
}
