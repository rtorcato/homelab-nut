// Package upsdetect probes a host for connected UPS hardware by running
// nut-scanner over an existing SSH connection and parsing its output.
//
// It backs three call sites: the `homelab-nut detect` command, the TUI's
// scan shortcut, and (best-effort) the add-host wizard. Keeping the scan +
// parse logic here means all three agree on what "detected" means.
package upsdetect

import (
	"context"
	"errors"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/ssh"
)

// DetectedUPS is one UPS nut-scanner reported on a host. Driver is the
// field that matters for inventory; the rest are surfaced so the user can
// confirm the scanner found the device they expect.
type DetectedUPS struct {
	Driver    string `json:"driver"`
	Port      string `json:"port,omitempty"`
	VendorID  string `json:"vendorid,omitempty"`
	ProductID string `json:"productid,omitempty"`
	Vendor    string `json:"vendor,omitempty"`
	Product   string `json:"product,omitempty"`
}

// Description returns a short human label for the detected device, e.g.
// "CPS CP1500PFCLCD" — empty when the scanner reported no vendor/product.
func (d DetectedUPS) Description() string {
	parts := make([]string, 0, 2)
	if d.Vendor != "" {
		parts = append(parts, d.Vendor)
	}
	if d.Product != "" {
		parts = append(parts, d.Product)
	}
	return strings.Join(parts, " ")
}

// ErrScannerMissing means nut-scanner isn't installed on the host. It
// ships with the nut package, which `homelab-nut apply` installs — so a
// fresh host needs an apply before detection can work.
var ErrScannerMissing = errors.New("nut-scanner not installed on host (run `homelab-nut apply` first)")

// sentinelMissing is echoed by scanCmd when nut-scanner isn't on PATH, so
// Scan can distinguish "tool absent" from "tool ran, found nothing".
const sentinelMissing = "__NO_NUT_SCANNER__"

// scanCmd checks for nut-scanner, then runs a USB-only scan. nut-scanner
// usually needs root for raw USB access, so try passwordless sudo first
// and fall back to a plain invocation. Output (if any) is ups.conf-style
// sections; empty output means no UPS was found.
const scanCmd = `if ! command -v nut-scanner >/dev/null 2>&1; then echo ` + sentinelMissing + `; exit 0; fi
sudo -n nut-scanner -U 2>/dev/null || nut-scanner -U 2>/dev/null || true`

// Scan runs nut-scanner over conn and returns the detected UPS devices.
// A nil result with ErrScannerMissing means the tool isn't installed yet;
// an empty (non-nil-error) result means it ran but found nothing.
func Scan(ctx context.Context, conn *ssh.Connection) ([]DetectedUPS, error) {
	if conn == nil {
		return nil, errors.New("upsdetect: nil connection")
	}
	res, err := conn.Run(ctx, scanCmd)
	if err != nil {
		return nil, err
	}
	if strings.Contains(res.Stdout, sentinelMissing) {
		return nil, ErrScannerMissing
	}
	return parseScan(res.Stdout), nil
}

// parseScan turns nut-scanner's ups.conf-style output into DetectedUPS
// values. Each `[section]` header starts a new device; `key = "value"`
// lines fill its fields. Unknown keys are ignored.
func parseScan(out string) []DetectedUPS {
	var devices []DetectedUPS
	var cur *DetectedUPS
	flush := func() {
		if cur != nil && cur.Driver != "" {
			devices = append(devices, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			cur = &DetectedUPS{}
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch key {
		case "driver":
			cur.Driver = val
		case "port":
			cur.Port = val
		case "vendorid":
			cur.VendorID = val
		case "productid":
			cur.ProductID = val
		case "vendor":
			cur.Vendor = val
		case "product":
			cur.Product = val
		}
	}
	flush()
	return devices
}
