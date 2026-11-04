package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/rtorcato/homelab-nut/internal/doctor"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [host]",
		Short: "Check that shutdown protection works on server, client and daemon hosts",
		Long: `Connects over SSH to every host with the nut-server, nut-client or
shutdown-daemon role and asserts, from that host's own point of view:

  - nut.conf MODE matches the host's roles (a nut-server is not a netclient)
  - upsmon is running and every MONITOR target answers on the NUT port
  - the UPS named in each MONITOR line exists on the server it points at
  - the shutdown daemon is running and can read its configured UPS right now

Unlike 'plan' (diffs against the inventory) and 'status' (nut-server hosts
only), this catches a client monitoring a decommissioned server while
every service still reports "active".

Exits 5 when any check fails or a host can't be probed, so cron and CI can
page someone. With -o json, output is a JSON array of per-host results.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := ""
			if len(args) == 1 {
				host = args[0]
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			return runDoctor(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				inventoryPath(cmd), host, timeout, getOutputFormat(cmd))
		},
	}
	cmd.Flags().Duration("timeout", 30*time.Second, "per-host SSH connect + probe deadline")
	addOutputFlag(cmd)
	return cmd
}

func runDoctor(parent context.Context, stdout, stderr io.Writer, path, hostArg string, timeout time.Duration, format outputFormat) error {
	if parent == nil {
		parent = context.Background()
	}
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}
	targets, err := doctorTargets(inv, hostArg)
	if err != nil {
		return err
	}

	executor := ssh.NewExecutor(ssh.NewConfig())
	defer func() { _ = executor.Close() }()

	results := make([]doctor.Result, len(targets))
	var wg sync.WaitGroup
	for i, h := range targets {
		wg.Add(1)
		go func(i int, h *inventory.Host) {
			defer wg.Done()
			results[i] = doctorHost(parent, executor, h, timeout)
		}(i, h)
	}
	wg.Wait()

	if format == outputJSON {
		err = emitJSON(stdout, results)
	} else {
		err = printDoctorResults(stdout, results)
	}
	if err != nil {
		return err
	}
	for _, r := range results {
		if !r.Healthy() {
			return errExit(ExitUnhealthy)
		}
	}
	return nil
}

// doctorTargets resolves the hosts to probe: the named host (which must
// carry a role doctor can assert on) or every such host.
func doctorTargets(inv *inventory.Inventory, hostArg string) ([]*inventory.Host, error) {
	if hostArg != "" {
		h := inv.HostByName(hostArg)
		if h == nil {
			return nil, fmt.Errorf("host %q not found in inventory", hostArg)
		}
		if !doctor.Applies(h) {
			return nil, fmt.Errorf("host %q has no nut-server, nut-client or shutdown-daemon role — nothing to check", hostArg)
		}
		return []*inventory.Host{h}, nil
	}
	var out []*inventory.Host
	for i := range inv.Hosts {
		if doctor.Applies(&inv.Hosts[i]) {
			out = append(out, &inv.Hosts[i])
		}
	}
	return out, nil
}

func doctorHost(parent context.Context, executor *ssh.Executor, h *inventory.Host, timeout time.Duration) doctor.Result {
	r := doctor.Result{Host: h.Name}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	conn, err := executor.Open(h)
	if err != nil {
		r.Error = fmt.Sprintf("ssh %s: %v", h.Name, err)
		return r
	}
	r.Checks, err = doctor.Run(ctx, conn, h)
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func printDoctorResults(w io.Writer, results []doctor.Result) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No hosts with the nut-server, nut-client or shutdown-daemon role in the inventory.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tCHECK\tRESULT\tDETAIL")
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(tw, "%s\t-\tFAIL\t%s\n", printable(r.Host), printable(r.Error))
			continue
		}
		for _, c := range r.Checks {
			res := "ok"
			if !c.OK {
				res = "FAIL"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", printable(r.Host), printable(c.Name), res, printable(c.Detail))
		}
	}
	return tw.Flush()
}

// printable drops control characters (ESC, CR, tabs, ...) from s. UPS names,
// MODE, ups.status and probe stderr come from the host or its NUT server, so
// a hostile one could otherwise send ANSI/OSC sequences that rewrite the
// terminal or break the table. JSON output needs no help: encoding/json
// escapes them.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
