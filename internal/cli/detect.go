package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/rtorcato/homelab-nut/internal/upsdetect"
	"github.com/spf13/cobra"
)

// detectTimeout caps how long we wait on a single host (connect + scan).
const detectTimeout = 20 * time.Second

// detectResult is the per-host outcome, shaped for `-o json`.
type detectResult struct {
	Host     string                  `json:"host"`
	Detected []upsdetect.DetectedUPS `json:"detected"`
	// WroteDriver is the driver persisted to the inventory for this host,
	// set only when --write changed it. Empty otherwise.
	WroteDriver string `json:"wrote_driver,omitempty"`
	Error       string `json:"error,omitempty"`
}

func newDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect [host]",
		Short: "Scan nut-server host(s) for connected UPS hardware (nut-scanner)",
		Long: `Connects over SSH to a nut-server host and runs nut-scanner to detect the
connected UPS and the driver NUT should use for it. With no argument, every
host with the nut-server role is scanned.

By default detect is read-only: it reports what's connected and the
recommended driver. Pass --write to persist the detected driver into the
inventory (only when exactly one UPS is found, to avoid guessing).

nut-scanner ships with the NUT package, which 'homelab-nut apply' installs —
so a brand-new host must be applied once before detect can see anything.

With -o json, output is a JSON array suitable for AI agents and scripts.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := inventoryPath(cmd)
			write, _ := cmd.Flags().GetBool("write")
			host := ""
			if len(args) == 1 {
				host = args[0]
			}
			return runDetect(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				path, host, write, getOutputFormat(cmd))
		},
	}
	cmd.Flags().Bool("write", false, "persist the detected driver into the inventory (single-UPS hosts only)")
	addOutputFlag(cmd)
	return cmd
}

// runDetect scans the requested host(s), optionally writes the detected
// driver back to the inventory, and renders the results.
func runDetect(parent context.Context, stdout, stderr io.Writer, path, hostArg string, write bool, format outputFormat) error {
	if parent == nil {
		parent = context.Background()
	}
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}

	targets, err := detectTargets(inv, hostArg)
	if err != nil {
		return err
	}

	executor := ssh.NewExecutor(ssh.NewConfig())
	defer func() { _ = executor.Close() }()

	results := make([]detectResult, 0, len(targets))
	changed := false
	for _, h := range targets {
		r := detectResult{Host: h.Name}
		devices, scanErr := scanHost(parent, executor, h)
		if scanErr != nil {
			r.Error = scanErr.Error()
		} else {
			r.Detected = devices
			if write && len(devices) == 1 {
				if applyDetectedDriver(h, devices[0].Driver) {
					r.WroteDriver = devices[0].Driver
					changed = true
				}
			}
		}
		results = append(results, r)
	}

	if changed {
		if err := inv.Save(path); err != nil {
			var vErr *inventory.ValidationError
			if errors.As(err, &vErr) {
				fmt.Fprintf(stderr, "%s\n", err)
				return errSilent
			}
			return err
		}
	}

	if format == outputJSON {
		return emitJSON(stdout, results)
	}
	printDetectResults(stdout, results, write)
	return nil
}

// detectTargets resolves which hosts to scan: the named host (which must
// exist and carry the nut-server role) or every nut-server host.
func detectTargets(inv *inventory.Inventory, hostArg string) ([]*inventory.Host, error) {
	if hostArg == "" {
		return inv.HostsWithRole(inventory.RoleNUTServer), nil
	}
	h := inv.HostByName(hostArg)
	if h == nil {
		return nil, fmt.Errorf("host %q not found in inventory", hostArg)
	}
	if !h.HasRole(inventory.RoleNUTServer) {
		return nil, fmt.Errorf("host %q has no nut-server role — only nut-server hosts have a UPS to detect", hostArg)
	}
	return []*inventory.Host{h}, nil
}

// scanHost opens an SSH connection to h and runs the UPS scan, capping the
// whole operation at detectTimeout.
func scanHost(parent context.Context, executor *ssh.Executor, h *inventory.Host) ([]upsdetect.DetectedUPS, error) {
	ctx, cancel := context.WithTimeout(parent, detectTimeout)
	defer cancel()

	conn, err := executor.Open(h)
	if err != nil {
		return nil, fmt.Errorf("ssh %s: %w", h.Name, err)
	}
	defer func() { _ = conn.Close() }()

	return upsdetect.Scan(ctx, conn)
}

// applyDetectedDriver writes driver into the host's UPS block, creating
// the block (with a default name) when the host doesn't have one yet.
// Returns true when something actually changed.
func applyDetectedDriver(h *inventory.Host, driver string) bool {
	if driver == "" {
		return false
	}
	if h.UPS == nil {
		h.UPS = &inventory.UPS{Name: "myups", Driver: driver}
		return true
	}
	if h.UPS.Driver == driver {
		return false
	}
	h.UPS.Driver = driver
	return true
}

func printDetectResults(w io.Writer, results []detectResult, write bool) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No hosts with the nut-server role in the inventory.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tDRIVER\tDEVICE\tRESULT")
	for _, r := range results {
		switch {
		case r.Error != "":
			fmt.Fprintf(tw, "%s\t-\t-\t%s\n", r.Host, truncate(r.Error, 50))
		case len(r.Detected) == 0:
			fmt.Fprintf(tw, "%s\t-\t-\tno UPS detected\n", r.Host)
		default:
			for i, d := range r.Detected {
				result := "detected"
				if r.WroteDriver == d.Driver {
					result = "written to inventory"
				} else if write && len(r.Detected) > 1 {
					result = "ambiguous — set manually"
				}
				host := r.Host
				if i > 0 {
					host = ""
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", host, d.Driver, dash(d.Description()), result)
			}
		}
	}
	_ = tw.Flush()
	if !write {
		fmt.Fprintln(w, "\nRe-run with --write to save the detected driver into the inventory.")
	}
}

// runDetectHost is the TUI entry point: scan + write a single host by its
// inventory index, then print a one-line summary. Used by the 's' shortcut.
func runDetectHost(stdout, stderr io.Writer, path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	return runDetect(context.Background(), stdout, stderr, path, inv.Hosts[idx].Name, true, outputText)
}
