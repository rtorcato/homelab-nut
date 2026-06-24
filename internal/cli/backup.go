package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

// backupTimeout caps connect + tar-pull time for a single host.
const backupTimeout = 2 * time.Minute

// defaultBackupDir is where tarballs land when --output isn't given.
const defaultBackupDir = "backups"

// backupResult is the per-host outcome, shaped for `-o json`.
type backupResult struct {
	Host            string `json:"host"`
	Path            string `json:"path,omitempty"`
	Bytes           int64  `json:"bytes"`
	SHA256          string `json:"sha256,omitempty"`
	IncludedSecrets bool   `json:"included_secrets"`
	Error           string `json:"error,omitempty"`
}

// backupSummary is the JSON emitted by `backup -o json`.
type backupSummary struct {
	Results []backupResult `json:"results"`
}

// backupScript runs on the target under sudo. It stages the files
// homelab-nut cares about, writes a MANIFEST and the host's inventory
// snapshot, then tars the lot to STDOUT (gzip). Everything human-readable
// goes to STDERR so STDOUT stays a clean archive stream. Driven by env:
//
//	INCLUDE_SECRETS  1 to include upsd.users + /etc/default/nut-exporter
//	INVENTORY_B64    base64 of this host's homelab-nut.yaml entry (optional)
const backupScript = `set -u
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE" >/dev/null 2>&1 || true' EXIT

copy() { # src dst — copy preserving mode if src exists
  if [ -e "$1" ]; then
    mkdir -p "$(dirname "$2")"
    cp -a "$1" "$2" 2>/dev/null && echo "  + $1" >&2 || true
  fi
}

# /etc/nut configs — upsd.users holds password hashes, so secrets-gated.
if [ -d /etc/nut ]; then
  for f in /etc/nut/*.conf; do
    [ -e "$f" ] && copy "$f" "$STAGE$f"
  done
  if [ "${INCLUDE_SECRETS:-0}" = "1" ]; then
    copy /etc/nut/upsd.users "$STAGE/etc/nut/upsd.users"
  fi
fi

# Custom systemd units written by homelab-nut.
copy /etc/systemd/system/nut-exporter.service "$STAGE/etc/systemd/system/nut-exporter.service"
copy /etc/systemd/system/ups-battery-shutdown.service "$STAGE/etc/systemd/system/ups-battery-shutdown.service"

# Exporter scrape credentials — secret.
if [ "${INCLUDE_SECRETS:-0}" = "1" ]; then
  copy /etc/default/nut-exporter "$STAGE/etc/default/nut-exporter"
fi

# Inventory snapshot for this host (reproducibility / transplant).
if [ -n "${INVENTORY_B64:-}" ]; then
  echo "$INVENTORY_B64" | base64 -d > "$STAGE/homelab-nut.yaml" 2>/dev/null || true
fi

# MANIFEST — versions + capture metadata.
{
  echo "host: $(hostname 2>/dev/null || echo unknown)"
  echo "captured_utc: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "kernel: $(uname -r 2>/dev/null || echo unknown)"
  echo "nut_version: $(upsd -V 2>&1 | head -n1 || echo unknown)"
  if command -v nut_exporter >/dev/null 2>&1; then
    echo "nut_exporter: $(nut_exporter --version 2>&1 | head -n1 || echo unknown)"
  fi
  echo "include_secrets: ${INCLUDE_SECRETS:-0}"
} > "$STAGE/MANIFEST"

# Archive to stdout — the ONLY thing on stdout.
tar czf - -C "$STAGE" .
`

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup [host]",
		Short: "Snapshot a host's NUT + homelab-nut config to a local tarball",
		Long: `Pulls the files homelab-nut cares about off a target over SSH into a
local gzip tarball: /etc/nut configs, the custom systemd units, a MANIFEST
(NUT / exporter / kernel versions), and a snapshot of the host's inventory
entry. Read-only on the target — no --auto-approve needed.

Secrets (upsd.users password hashes, the exporter's scrape credentials)
are excluded by default so the tarball is safe to share; pass
--include-secrets for a true backup-and-restore capture.

Default path: ./backups/<host>-<timestamp>.tar.gz`,
		Example: `  homelab-nut backup pi-rack
  homelab-nut backup --all --output /mnt/backups
  homelab-nut backup pi-rack --include-secrets -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var host string
			if len(args) == 1 {
				host = args[0]
			}
			all, _ := cmd.Flags().GetBool("all")
			output, _ := cmd.Flags().GetString("out")
			includeSecrets, _ := cmd.Flags().GetBool("include-secrets")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
			return runBackup(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				inventoryPath(cmd), host, all, output, includeSecrets, concurrency, ts, getOutputFormat(cmd))
		},
	}
	cmd.Flags().Bool("all", false, "back up every host in the inventory (required if no host is given)")
	cmd.Flags().StringP("out", "O", "", "destination directory, or a .tar.gz file path for a single host (default: ./backups)")
	cmd.Flags().Bool("include-secrets", false, "include upsd.users + /etc/default/nut-exporter (sensitive)")
	cmd.Flags().Int("concurrency", 0, "max hosts to back up in parallel (0 = unlimited)")
	addOutputFlag(cmd)
	return cmd
}

func runBackup(parent context.Context, stdout, stderr io.Writer, path, onlyHost string, all bool, output string, includeSecrets bool, concurrency int, ts string, format outputFormat) error {
	if parent == nil {
		parent = context.Background()
	}
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}

	// Safety: a bare `backup` with no host and no --all would snapshot the
	// whole fleet. Require an explicit opt-in either way.
	if onlyHost == "" && !all {
		fmt.Fprintln(stderr, "no host given — pass a host name, or --all to back up every host")
		return errSilent
	}
	if onlyHost != "" && inv.HostByName(onlyHost) == nil {
		fmt.Fprintf(stderr, "host %q not found in inventory\n", onlyHost)
		return errSilent
	}

	var targets []*inventory.Host
	for i := range inv.Hosts {
		h := &inv.Hosts[i]
		if onlyHost == "" || h.Name == onlyHost {
			targets = append(targets, h)
		}
	}

	// Resolve where tarballs go. A .tar.gz --output is an explicit single
	// file — only valid for one host; everything else is a directory.
	fileMode := strings.HasSuffix(output, ".tar.gz")
	if fileMode && len(targets) > 1 {
		fmt.Fprintln(stderr, "--out as a .tar.gz file works with a single host; use a directory for --all")
		return errSilent
	}
	dir := output
	if !fileMode {
		if dir == "" {
			dir = defaultBackupDir
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "create backup dir %q: %v\n", dir, err)
			return errSilent
		}
	}

	executor := ssh.NewExecutor(ssh.NewConfig())
	defer func() { _ = executor.Close() }()

	results := make([]backupResult, len(targets))
	sem := make(chan struct{}, normaliseConcurrency(concurrency, len(targets)))
	var wg sync.WaitGroup
	for i := range targets {
		h := targets[i]
		dst := output
		if !fileMode {
			dst = filepath.Join(dir, fmt.Sprintf("%s-%s.tar.gz", h.Name, ts))
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, host *inventory.Host, outPath string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = backupHost(parent, executor, host, inv, outPath, includeSecrets)
		}(i, h, dst)
	}
	wg.Wait()

	if format == outputJSON {
		if err := emitJSON(stdout, backupSummary{Results: results}); err != nil {
			return err
		}
	} else {
		printBackupResults(stdout, results)
	}

	for _, r := range results {
		if r.Error != "" {
			return errExit(ExitApplyPartial)
		}
	}
	return nil
}

// normaliseConcurrency clamps the requested parallelism to [1, n].
func normaliseConcurrency(requested, n int) int {
	if n < 1 {
		return 1
	}
	if requested <= 0 || requested > n {
		return n
	}
	return requested
}

// backupHost pulls one host's snapshot tarball to outPath, returning its
// size + sha256. Read-only on the target (tar over SSH); a failure leaves
// no partial file behind.
func backupHost(parent context.Context, executor *ssh.Executor, h *inventory.Host, inv *inventory.Inventory, outPath string, includeSecrets bool) backupResult {
	r := backupResult{Host: h.Name, Path: outPath, IncludedSecrets: includeSecrets}

	ctx, cancel := context.WithTimeout(parent, backupTimeout)
	defer cancel()

	conn, err := executor.Open(h)
	if err != nil {
		r.Path = ""
		r.Error = fmt.Sprintf("ssh: %v", err)
		return r
	}
	defer func() { _ = conn.Close() }()

	f, err := os.Create(outPath)
	if err != nil {
		r.Path = ""
		r.Error = fmt.Sprintf("create %s: %v", outPath, err)
		return r
	}

	hasher := sha256.New()
	counter := &countingWriter{}
	var logBuf bytes.Buffer
	secrets := 0
	if includeSecrets {
		secrets = 1
	}
	cmd := fmt.Sprintf("sudo INCLUDE_SECRETS=%d INVENTORY_B64=%s bash -s", secrets, shellQuote(hostInventoryB64(inv, h)))

	pipeErr := conn.Pipe(ctx, strings.NewReader(backupScript), cmd, io.MultiWriter(f, hasher, counter), &logBuf)
	closeErr := f.Close()
	if pipeErr != nil {
		_ = os.Remove(outPath)
		r.Path = ""
		r.Error = fmt.Sprintf("ssh tar: %v", pipeErr)
		return r
	}
	if closeErr != nil {
		r.Path = ""
		r.Error = fmt.Sprintf("write %s: %v", outPath, closeErr)
		return r
	}

	r.Bytes = counter.n
	r.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	return r
}

// hostInventoryB64 renders just this host's inventory entry to YAML and
// base64-encodes it for the remote snapshot. Returns "" if rendering fails
// (the backup still proceeds — the snapshot is a nice-to-have).
func hostInventoryB64(inv *inventory.Inventory, h *inventory.Host) string {
	snap := &inventory.Inventory{Hosts: []inventory.Host{*h}}
	// Only carry the fleet-wide daemon block when this host actually runs
	// the daemon — otherwise the single-host snapshot fails validation
	// ("shutdown_daemon configured but no host has role shutdown-daemon").
	if h.HasRole(inventory.RoleShutdownDaemon) {
		snap.ShutdownDaemon = inv.ShutdownDaemon
	}
	var buf bytes.Buffer
	if err := snap.Render(&buf); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// countingWriter tallies bytes written so we can report the tarball size
// while it streams, without stat-ing the file afterward.
type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

func printBackupResults(w io.Writer, results []backupResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No matching hosts to back up.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tSIZE\tPATH / ERROR")
	ok := 0
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Host, "-", r.Error)
			continue
		}
		ok++
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Host, humanBytes(r.Bytes), r.Path)
	}
	_ = tw.Flush()
	secretsNote := ""
	for _, r := range results {
		if r.IncludedSecrets {
			secretsNote = " (secrets included — store securely)"
			break
		}
	}
	fmt.Fprintf(w, "\n%d of %d host(s) backed up%s.\n", ok, len(results), secretsNote)
}

// humanBytes renders a byte count as a compact human string (e.g. 12.3 KiB).
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
