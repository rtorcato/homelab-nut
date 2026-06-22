package roles

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

func init() { Register(shutdownDaemon{}) }

// shutdownDaemon installs the battery-shutdown systemd service on the
// designated host. Generates config from inventory's ShutdownDaemon
// block plus a cross-host lookup of every shutdown-target host.
//
// SSH key distribution to targets is out of scope for this role —
// Apply prints the daemon's public key and tells the user to push it
// to each target's authorized_keys. Auto-distribution can land later
// as a post-apply step in the orchestrator.
type shutdownDaemon struct{}

func (shutdownDaemon) Name() string { return string(inventory.RoleShutdownDaemon) }

func (shutdownDaemon) Applies(h *inventory.Host) bool {
	return h != nil && h.HasRole(inventory.RoleShutdownDaemon)
}

// remoteNodesFromInventory builds the space-separated "user@address"
// list the daemon's REMOTE_NODES env expects. Returns the list in
// inventory order so the config file is reproducible across runs.
func remoteNodesFromInventory(inv *inventory.Inventory) string {
	if inv == nil {
		return ""
	}
	nodes := make([]string, 0)
	for i := range inv.Hosts {
		h := &inv.Hosts[i]
		if h.HasRole(inventory.RoleShutdownTarget) {
			nodes = append(nodes, fmt.Sprintf("%s@%s", h.User, h.Address))
		}
	}
	return strings.Join(nodes, " ")
}

// sanitizeNodeHost mirrors battery-shutdown.sh's hostname sanitization
// (the address portion, with '-' and '.' replaced by '_') so the
// CMD_<host> override keys we emit line up exactly with what the daemon
// looks up per node at trigger time.
func sanitizeNodeHost(address string) string {
	s := strings.ReplaceAll(address, "-", "_")
	return strings.ReplaceAll(s, ".", "_")
}

// remoteCmdsFromInventory builds the per-target "CMD_<host>=<command>"
// override lines for every shutdown-target that declares a command in the
// inventory. Returned newline-joined and in inventory order (may be empty).
// Without these the daemon falls back to REMOTE_SHUTDOWN_CMD for every node,
// which silently breaks inline-command targets (e.g. UniFi `poweroff`).
func remoteCmdsFromInventory(inv *inventory.Inventory) string {
	if inv == nil {
		return ""
	}
	lines := make([]string, 0)
	for i := range inv.Hosts {
		h := &inv.Hosts[i]
		if !h.HasRole(inventory.RoleShutdownTarget) || h.Shutdown == nil || h.Shutdown.Command == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("CMD_%s=%s", sanitizeNodeHost(h.Address), h.Shutdown.Command))
	}
	return strings.Join(lines, "\n")
}

// upsRefFromInventory returns the local UPS reference for the daemon
// host (assumes the daemon runs on the nut-server host — common config).
// Defaults to "myups@localhost" if no nut-server is colocated.
func upsRefFromInventory(daemonHost *inventory.Host) string {
	if daemonHost != nil && daemonHost.HasRole(inventory.RoleNUTServer) && daemonHost.UPS != nil && daemonHost.UPS.Name != "" {
		return daemonHost.UPS.Name + "@localhost"
	}
	return "myups@localhost"
}

// shutdownDaemonDetectCmd checks for all four install artefacts.
const shutdownDaemonDetectCmd = `set -e
PARTIAL=0
COUNT=0
[ -x /usr/local/bin/ups-battery-shutdown ] && COUNT=$((COUNT+1))
[ -f /etc/ups-battery-shutdown.conf ] && COUNT=$((COUNT+1))
[ -f /etc/systemd/system/ups-battery-shutdown.service ] && COUNT=$((COUNT+1))
systemctl is-active --quiet ups-battery-shutdown.service && COUNT=$((COUNT+1))
case "$COUNT" in
    4) echo OK ;;
    0) echo MISSING ;;
    *) echo PARTIAL ;;
esac`

func (shutdownDaemon) Detect(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (State, error) {
	if conn == nil {
		return StateUnknown, nil
	}
	res, err := conn.Run(ctx, shutdownDaemonDetectCmd)
	if err != nil {
		return StateUnknown, fmt.Errorf("detect shutdown-daemon on %s: %w", h.Name, err)
	}
	switch strings.TrimSpace(res.Stdout) {
	case "OK":
		return StateOK, nil
	case "PARTIAL":
		return StatePartial, nil
	case "MISSING":
		return StateMissing, nil
	default:
		return StateUnknown, fmt.Errorf("shutdown-daemon detect: unexpected output %q (stderr: %s)", res.Stdout, res.Stderr)
	}
}

func (r shutdownDaemon) Plan(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (*Diff, error) {
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

	// Resolve cross-host data when inventory is available.
	inv := inventoryFrom(ctx)
	remoteNodes := "(resolved at apply time)"
	cmdOverrides := ""
	threshold := 50
	pollInterval := 30
	if inv != nil {
		remoteNodes = remoteNodesFromInventory(inv)
		if remoteNodes == "" {
			return nil, fmt.Errorf("shutdown-daemon on %s: inventory has no hosts with role 'shutdown-target' — nothing for the daemon to power down", h.Name)
		}
		cmdOverrides = remoteCmdsFromInventory(inv)
		if inv.ShutdownDaemon != nil {
			threshold = inv.ShutdownDaemon.Threshold
			pollInterval = inv.ShutdownDaemon.PollInterval
		}
	}

	if current == StateOK {
		return d, nil
	}
	d.Actions = []string{
		"install /usr/local/bin/ups-battery-shutdown (chmod 700)",
		fmt.Sprintf("write /etc/ups-battery-shutdown.conf (UPS=%s, threshold=%d%%, poll=%ds)", upsRefFromInventory(h), threshold, pollInterval),
		"generate /root/.ssh/id_ed25519_ups if missing",
		"install + enable + restart ups-battery-shutdown.service",
		fmt.Sprintf("targets: %s", remoteNodes),
	}
	if cmdOverrides != "" {
		d.Actions = append(d.Actions,
			fmt.Sprintf("per-target shutdown commands: %s", strings.ReplaceAll(cmdOverrides, "\n", ", ")))
	}
	d.Actions = append(d.Actions, "output public key for manual distribution to shutdown-targets")
	return d, nil
}

func (r shutdownDaemon) Apply(ctx context.Context, conn *ssh.Connection, h *inventory.Host, out io.Writer) error {
	if conn == nil {
		return errors.New("shutdown-daemon apply: nil connection")
	}
	inv := inventoryFrom(ctx)
	if inv == nil {
		return errors.New("shutdown-daemon apply: inventory missing from context (use roles.WithInventory)")
	}
	remoteNodes := remoteNodesFromInventory(inv)
	if remoteNodes == "" {
		return fmt.Errorf("shutdown-daemon on %s: no shutdown-target hosts in inventory", h.Name)
	}

	threshold := 50
	pollInterval := 30
	slackWebhook := ""
	if inv.ShutdownDaemon != nil {
		threshold = inv.ShutdownDaemon.Threshold
		pollInterval = inv.ShutdownDaemon.PollInterval
		if env := inv.ShutdownDaemon.SlackWebhookEnv; env != "" {
			slackWebhook = os.Getenv(env)
		}
	}

	setupScript, err := readScript("setup-shutdown-daemon.sh")
	if err != nil {
		return err
	}
	daemonScript, err := readScript("battery-shutdown.sh")
	if err != nil {
		return err
	}
	daemonB64 := base64.StdEncoding.EncodeToString(daemonScript)
	// Per-target command overrides, base64'd so newline-joined lines survive
	// the env var cleanly (same single-stdin reasoning as the daemon script).
	remoteCmdsB64 := base64.StdEncoding.EncodeToString([]byte(remoteCmdsFromInventory(inv)))

	// Wire all the inputs via env vars on the remote sudo invocation.
	// We pass the daemon's bytes as a base64 env var so the orchestrator
	// script only needs a single stdin (itself).
	cmd := fmt.Sprintf(
		`sudo HOMELAB_NUT_DAEMON_B64=%q UPS=%q THRESHOLD=%d POLL_INTERVAL=%d REMOTE_NODES=%q REMOTE_CMDS_B64=%q SLACK_WEBHOOK=%q bash -s --`,
		daemonB64,
		upsRefFromInventory(h),
		threshold,
		pollInterval,
		remoteNodes,
		remoteCmdsB64,
		slackWebhook,
	)

	if out == nil {
		out = io.Discard
	}
	if err := conn.Pipe(ctx, bytes.NewReader(setupScript), cmd, out, out); err != nil {
		return fmt.Errorf("shutdown-daemon apply on %s: %w", h.Name, err)
	}
	return nil
}
