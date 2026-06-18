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

func init() { Register(exporter{}) }

// exporter installs druggeri/nut_exporter as a bare-metal systemd
// service so Prometheus can scrape UPS metrics from this host.
//
// Two modes, driven by the inventory shape:
//   - Co-located with nut-server: scrape localhost, no auth required.
//   - Standalone: scrape a separate nut-server host, requires the
//     NUT_MONITOR_PASSWORD env var at Apply time (same credential
//     nut-client uses).
type exporter struct{}

func (exporter) Name() string { return string(inventory.RoleExporter) }

func (exporter) Applies(h *inventory.Host) bool {
	return h != nil && h.HasRole(inventory.RoleExporter)
}

// isCoLocated reports whether the host runs nut-server on the same
// machine. Co-located exporters don't need a server-lookup or auth.
func (exporter) isCoLocated(h *inventory.Host) bool {
	return h != nil && h.HasRole(inventory.RoleNUTServer)
}

// exporterDetectCmd is the shell snippet Detect runs.
const exporterDetectCmd = `set -e
if ! command -v nut_exporter >/dev/null 2>&1; then
    echo MISSING
elif systemctl is-active --quiet nut-exporter; then
    echo OK
else
    echo PARTIAL
fi`

func (exporter) Detect(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (State, error) {
	if conn == nil {
		return StateUnknown, nil
	}
	res, err := conn.Run(ctx, exporterDetectCmd)
	if err != nil {
		return StateUnknown, fmt.Errorf("detect exporter on %s: %w", h.Name, err)
	}
	switch strings.TrimSpace(res.Stdout) {
	case "OK":
		return StateOK, nil
	case "PARTIAL":
		return StatePartial, nil
	case "MISSING":
		return StateMissing, nil
	default:
		return StateUnknown, fmt.Errorf("exporter detect: unexpected output %q (stderr: %s)", res.Stdout, res.Stderr)
	}
}

func (r exporter) Plan(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (*Diff, error) {
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

	scrapeTarget := "localhost (co-located with nut-server)"
	if !r.isCoLocated(h) {
		// Standalone exporter — needs to find a nut-server host in inventory.
		// If no inventory in ctx, fall back to a placeholder action — Apply
		// will catch the real problem with a clearer error.
		if inv := inventoryFrom(ctx); inv != nil {
			server, ferr := findNUTServer(inv)
			if ferr != nil {
				return nil, fmt.Errorf("exporter on %s (standalone): %w", h.Name, ferr)
			}
			scrapeTarget = fmt.Sprintf("%s (remote nut-server: %s)", server.Address, server.Name)
		} else {
			scrapeTarget = "remote nut-server (resolved at apply time)"
		}
	}

	if current == StateOK {
		return d, nil
	}
	d.Actions = []string{
		"download druggeri/nut_exporter binary (auto-detect arch)",
		"install /etc/systemd/system/nut-exporter.service",
		"create unprivileged nut-exporter user",
		fmt.Sprintf("scrape target: %s", scrapeTarget),
		"enable + start nut-exporter systemd unit (port 9199)",
	}
	return d, nil
}

func (r exporter) Apply(ctx context.Context, conn *ssh.Connection, h *inventory.Host, out io.Writer) error {
	if conn == nil {
		return errors.New("exporter apply: nil connection")
	}

	script, err := readScript("setup-exporter.sh")
	if err != nil {
		return err
	}

	// Build the arg list based on mode.
	var cmd string
	if r.isCoLocated(h) {
		// Defaults to localhost, no auth.
		cmd = `sudo bash -s --`
	} else {
		inv := inventoryFrom(ctx)
		if inv == nil {
			return errors.New("exporter apply: standalone mode needs inventory in ctx (use roles.WithInventory)")
		}
		server, err := findNUTServer(inv)
		if err != nil {
			return fmt.Errorf("exporter apply on %s: %w", h.Name, err)
		}
		password := os.Getenv(nutMonitorPasswordEnv)
		if password == "" {
			return fmt.Errorf("exporter apply on %s (standalone): %s not set — run `sudo grep upsmon_remote /root/nut-credentials.txt` on %s and export it",
				h.Name, nutMonitorPasswordEnv, server.Name)
		}
		// setup-exporter.sh args: <NUT_SERVER> <NUT_USER> <NUT_PASSWORD>
		cmd = fmt.Sprintf(`sudo bash -s -- %q upsmon_remote %q`, server.Address, password)
	}

	if out == nil {
		out = io.Discard
	}
	return conn.Pipe(ctx, bytes.NewReader(script), cmd, out, out)
}
