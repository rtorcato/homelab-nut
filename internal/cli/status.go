package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ups"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Poll each nut-server host for live UPS state",
		Long: `Connects to every host in the inventory with the nut-server role over
the NUT TCP protocol (port 3493) and reports its current UPS state:
status string (OL, OB, OB LB, …), battery %, load %, and runtime estimate.

Hosts without the nut-server role are skipped (no live UPS state to read).
With -o json, output is a JSON array suitable for AI agents and scripts.
With --watch, the command redraws every --interval until Ctrl+C; in JSON
mode each tick emits a fresh JSON array on its own line (NDJSON-friendly).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			watch, _ := cmd.Flags().GetBool("watch")
			interval, _ := cmd.Flags().GetDuration("interval")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			return runStatus(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				path, watch, timeout, interval, getOutputFormat(cmd))
		},
	}
	cmd.Flags().BoolP("watch", "w", false, "redraw on an interval instead of exiting (Ctrl+C to stop)")
	cmd.Flags().Duration("interval", 5*time.Second, "redraw cadence when --watch is set")
	cmd.Flags().Duration("timeout", 2*time.Second, "per-host TCP connect + read deadline")
	addOutputFlag(cmd)
	return cmd
}

func runStatus(parent context.Context, stdout, stderr io.Writer, path string, watch bool, timeout, interval time.Duration, format outputFormat) error {
	if parent == nil {
		parent = context.Background()
	}
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}
	hosts := inv.HostsWithRole(inventory.RoleNUTServer)

	if !watch {
		return render(stdout, format, pollAll(parent, hosts, timeout))
	}

	// --watch: signal-aware context so Ctrl+C (and SIGTERM) exits cleanly.
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Immediate first draw so the user sees something before the first tick.
	if err := renderWatch(stdout, format, pollAll(ctx, hosts, timeout), interval); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			// Drop a newline after the cleared screen so the shell prompt
			// lands on a fresh line; suppressed in JSON mode.
			if format == outputText {
				fmt.Fprintln(stdout)
			}
			return nil
		case <-ticker.C:
			if err := renderWatch(stdout, format, pollAll(ctx, hosts, timeout), interval); err != nil {
				return err
			}
		}
	}
}

func render(w io.Writer, format outputFormat, rows []statusRow) error {
	if format == outputJSON {
		return emitJSON(w, rows)
	}
	return printStatusTable(w, rows)
}

// renderWatch clears the screen (text mode only) before redrawing. JSON
// mode just emits a fresh complete array per tick — streaming consumers
// can parse line-by-line.
func renderWatch(w io.Writer, format outputFormat, rows []statusRow, interval time.Duration) error {
	if format == outputText {
		// ANSI: cursor home + clear from cursor down. Avoids the flicker
		// of \033[2J's full-screen clear when the table is the same size.
		fmt.Fprint(w, "\033[H\033[J")
		fmt.Fprintf(w, "homelab-nut status — every %s (Ctrl+C to stop) — %s\n\n",
			interval, time.Now().Format("15:04:05"))
	}
	return render(w, format, rows)
}

// statusRow is one entry in `status -o json` output. Fields without a
// reading are omitted (omitempty on numerics, blank string elsewhere)
// so consumers can distinguish "unknown" from "zero".
type statusRow struct {
	Host           string   `json:"host"`
	Address        string   `json:"address"`
	UPS            string   `json:"ups,omitempty"`
	Status         string   `json:"status,omitempty"`
	BatteryCharge  *float64 `json:"battery_charge,omitempty"`
	BatteryRuntime *int     `json:"battery_runtime,omitempty"`
	Load           *float64 `json:"load,omitempty"`
	Error          string   `json:"error,omitempty"`
}

func pollAll(ctx context.Context, hosts []*inventory.Host, timeout time.Duration) []statusRow {
	out := make([]statusRow, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(i int, h *inventory.Host) {
			defer wg.Done()
			out[i] = pollHost(ctx, h, timeout)
		}(i, h)
	}
	wg.Wait()
	return out
}

func pollHost(ctx context.Context, h *inventory.Host, timeout time.Duration) statusRow {
	row := statusRow{Host: h.Name, Address: h.Address}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client, err := ups.Dial(dialCtx, h.Address, ups.DialOptions{
		Timeout:  timeout,
		Deadline: timeout,
	})
	if err != nil {
		row.Error = sanitizeErr(err)
		return row
	}
	defer client.Close()

	list, err := client.ListUPS()
	if err != nil {
		row.Error = sanitizeErr(err)
		return row
	}
	if len(list) == 0 {
		row.Error = "no UPS reported by server"
		return row
	}
	// Use the first UPS on the host. Multi-UPS hosts are a future concern
	// — when they land, statusRow becomes one row per (host, UPS).
	upsName := list[0].Name
	row.UPS = upsName

	vars, err := client.ListVar(upsName)
	if err != nil {
		row.Error = sanitizeErr(err)
		return row
	}
	row.Status = vars["ups.status"]
	row.BatteryCharge = parseFloatPtr(vars["battery.charge"])
	row.BatteryRuntime = parseIntPtr(vars["battery.runtime"])
	row.Load = parseFloatPtr(vars["ups.load"])
	return row
}

// sanitizeErr collapses a deadline-exceeded error into a shorter form
// that survives line truncation in the table view. Other errors pass through.
func sanitizeErr(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return err.Error()
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseIntPtr(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

func printStatusTable(w io.Writer, rows []statusRow) error {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No hosts with the nut-server role in the inventory.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tADDRESS\tUPS\tSTATUS\tBATTERY\tLOAD\tRUNTIME\tERROR")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Host, r.Address, dash(r.UPS), dash(r.Status),
			formatPct(r.BatteryCharge), formatPct(r.Load),
			formatRuntime(r.BatteryRuntime), truncate(r.Error, 40))
	}
	return tw.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatPct(p *float64) string {
	if p == nil {
		return "-"
	}
	return strconv.FormatFloat(*p, 'f', 0, 64) + "%"
}

func formatRuntime(seconds *int) string {
	if seconds == nil {
		return "-"
	}
	d := time.Duration(*seconds) * time.Second
	return d.Round(time.Second).String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}
