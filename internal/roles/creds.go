package roles

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

// sshConfigKey carries the SSH config on the context so roles that need a
// second connection (e.g. nut-client reading the server's credentials)
// can dial with the same settings the orchestrator used.
type sshConfigKey struct{}

// WithSSHConfig returns a context carrying cfg for roles that open their
// own connections during Apply. The orchestrator sets this alongside
// WithInventory.
func WithSSHConfig(ctx context.Context, cfg ssh.Config) context.Context {
	return context.WithValue(ctx, sshConfigKey{}, cfg)
}

func sshConfigFrom(ctx context.Context) (ssh.Config, bool) {
	if v := ctx.Value(sshConfigKey{}); v != nil {
		if cfg, ok := v.(ssh.Config); ok {
			return cfg, true
		}
	}
	return ssh.Config{}, false
}

// readMonitorPassCmd extracts the upsmon_remote password from the
// credentials file setup-server.sh writes (a "Remote Pass:   <value>"
// line). Runs under sudo because the file is root-only (mode 600).
const readMonitorPassCmd = `sudo awk -F': *' '/^Remote Pass:/{print $2; exit}' /root/nut-credentials.txt`

// resolveMonitorPassword returns the password a client/exporter uses to
// authenticate to the NUT server as upsmon_remote. It prefers the
// NUT_MONITOR_PASSWORD env var (explicit override / CI), and otherwise
// auto-discovers it by SSHing into the server and reading the credentials
// file the server generated. Removing the manual export was the deferred
// "Phase 6 nice-to-have" noted on nutMonitorPasswordEnv.
func resolveMonitorPassword(ctx context.Context, server *inventory.Host) (string, error) {
	if p := os.Getenv(nutMonitorPasswordEnv); p != "" {
		return p, nil
	}
	if server == nil {
		return "", fmt.Errorf("%s not set and no nut-server host to auto-fetch it from", nutMonitorPasswordEnv)
	}

	cfg, ok := sshConfigFrom(ctx)
	if !ok {
		// No SSH config in context (ad-hoc call outside the orchestrator).
		return "", fmt.Errorf("%s not set and no SSH config available to auto-fetch it from %s", nutMonitorPasswordEnv, server.Name)
	}

	executor := ssh.NewExecutor(cfg)
	defer func() { _ = executor.Close() }()

	conn, err := executor.Open(server)
	if err != nil {
		return "", fmt.Errorf("couldn't reach nut-server %s to read credentials: %w (or set %s manually)", server.Name, err, nutMonitorPasswordEnv)
	}
	defer func() { _ = conn.Close() }()

	res, err := conn.Run(ctx, readMonitorPassCmd)
	if err != nil {
		return "", fmt.Errorf("reading credentials on %s: %w (or set %s manually)", server.Name, err, nutMonitorPasswordEnv)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("reading /root/nut-credentials.txt on %s failed (exit %d): %s — has %s been applied yet? (or set %s manually)",
			server.Name, res.ExitCode, strings.TrimSpace(res.Stderr), server.Name, nutMonitorPasswordEnv)
	}
	pass := strings.TrimSpace(res.Stdout)
	if pass == "" {
		return "", fmt.Errorf("upsmon_remote password not found in /root/nut-credentials.txt on %s (or set %s manually)", server.Name, nutMonitorPasswordEnv)
	}
	return pass, nil
}
