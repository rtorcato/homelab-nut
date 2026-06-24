package roles

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/ssh"
)

// UninstallParams controls how aggressive a role's Uninstall is. The zero
// value removes only homelab-nut's own artifacts (the custom systemd units,
// binaries, and config it wrote) and leaves the upstream NUT package alone.
type UninstallParams struct {
	// PurgeNUT also apt-purges the upstream NUT packages and deletes
	// /etc/nut (plus the generated credentials file). Destructive and
	// irreversible without a backup — gated behind an explicit flag and
	// confirmation at the CLI/TUI layer.
	PurgeNUT bool
}

// Removal is a role's contribution to an uninstall: what it deleted on the
// host and what was already absent. JSON-tagged for `uninstall -o json`
// (the documented shape in AGENTS.md aggregates these per host).
type Removal struct {
	Role    string   `json:"role"`
	Removed []string `json:"removed"`
	Skipped []string `json:"skipped"`
}

// shellQuote single-quotes s for safe interpolation into the generated
// bash, escaping any embedded single quotes the POSIX way ('\”).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// buildRemovalScript emits an idempotent bash program that removes the
// given systemd units, files/dirs, and apt packages, printing one
// "removed:"/"absent:"/"error:" line per artifact so the caller can build
// the structured Removal. Units are stopped + disabled before their unit
// file is deleted; a single daemon-reload runs at the end if any unit was
// listed. It uses `set -u` (not `-e`) and guards every step with `|| true`
// so one already-absent artifact never aborts the rest.
func buildRemovalScript(units, files, pkgs []string) string {
	var b strings.Builder
	b.WriteString("set -u\n")
	for _, u := range units {
		q := shellQuote(u)
		path := shellQuote("/etc/systemd/system/" + u)
		fmt.Fprintf(&b, "if [ -f %[2]s ] || systemctl is-enabled %[1]s >/dev/null 2>&1; then\n", q, path)
		fmt.Fprintf(&b, "  systemctl stop %s >/dev/null 2>&1 || true\n", q)
		fmt.Fprintf(&b, "  systemctl disable %s >/dev/null 2>&1 || true\n", q)
		fmt.Fprintf(&b, "  rm -f %s\n", path)
		fmt.Fprintf(&b, "  echo \"removed: unit %s\"\n", u)
		fmt.Fprintf(&b, "else echo \"absent: unit %s\"; fi\n", u)
	}
	for _, f := range files {
		q := shellQuote(f)
		fmt.Fprintf(&b, "if [ -e %[1]s ]; then rm -rf %[1]s; echo \"removed: %[2]s\"; else echo \"absent: %[2]s\"; fi\n", q, f)
	}
	for _, p := range pkgs {
		q := shellQuote(p)
		fmt.Fprintf(&b, "if dpkg-query -W -f='${Status}' %s 2>/dev/null | grep -q 'install ok installed'; then ", q)
		fmt.Fprintf(&b, "DEBIAN_FRONTEND=noninteractive apt-get purge -y %s && echo \"removed: package %s\" || echo \"error: package %s purge failed\"; ", q, p, p)
		fmt.Fprintf(&b, "else echo \"absent: package %s\"; fi\n", p)
	}
	if len(units) > 0 {
		b.WriteString("systemctl daemon-reload >/dev/null 2>&1 || true\n")
	}
	return b.String()
}

// parseRemoval scans the removal script's stdout for the "removed:" /
// "absent:" / "error:" marker lines and sorts them into a Removal plus a
// list of error messages (apt purge failures). Non-marker lines — apt's
// own chatter, systemctl output — are ignored here but still streamed to
// the user via the live writer in removeArtifacts.
func parseRemoval(role, output string) (*Removal, []string) {
	rem := &Removal{Role: role}
	var errs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "removed: "):
			rem.Removed = append(rem.Removed, strings.TrimPrefix(line, "removed: "))
		case strings.HasPrefix(line, "absent: "):
			rem.Skipped = append(rem.Skipped, strings.TrimPrefix(line, "absent: "))
		case strings.HasPrefix(line, "error: "):
			errs = append(errs, strings.TrimPrefix(line, "error: "))
		}
	}
	return rem, errs
}

// removeArtifacts runs buildRemovalScript on the host under sudo, streaming
// command output to out as it arrives and returning a structured Removal.
// A non-nil error means SSH failed, or a package purge errored — the
// Removal is still returned with whatever the script managed to remove.
func removeArtifacts(ctx context.Context, conn *ssh.Connection, role string, out io.Writer, units, files, pkgs []string) (*Removal, error) {
	if conn == nil {
		return nil, fmt.Errorf("%s uninstall: nil connection", role)
	}
	if out == nil {
		out = io.Discard
	}
	script := buildRemovalScript(units, files, pkgs)

	// Tee stdout: stream live to out and capture for marker parsing.
	var buf bytes.Buffer
	runErr := conn.Pipe(ctx, strings.NewReader(script), "sudo bash -s", io.MultiWriter(out, &buf), out)

	rem, removalErrs := parseRemoval(role, buf.String())
	if runErr != nil {
		return rem, fmt.Errorf("%s uninstall: %w", role, runErr)
	}
	if len(removalErrs) > 0 {
		return rem, fmt.Errorf("%s uninstall: %s", role, strings.Join(removalErrs, "; "))
	}
	return rem, nil
}
