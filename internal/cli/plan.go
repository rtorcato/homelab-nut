package cli

import (
	"fmt"
	"io"

	"github.com/rtorcato/homelab-nut/internal/orchestrator"
	hssh "github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Preview what `apply` would change on each host",
		Long: `Loads the inventory, opens SSH connections to each host, runs each
role's Detect + Plan, and prints a per-role diff. Read-only — makes
no changes on any host.

Use the same flags as ` + "`apply`" + ` so you can dry-run with one
command and execute with another.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			return runPlan(cmd.OutOrStdout(), cmd.ErrOrStderr(), path)
		},
	}
	return cmd
}

func runPlan(stdout, stderr io.Writer, path string) error {
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}
	res := orchestrator.Plan(commandContext(), inv, orchestrator.Options{
		SSHConfig: hssh.NewConfig(),
	})
	printPlanResult(stdout, res)
	if res.HasErrors() {
		return errSilent
	}
	return nil
}

// printPlanResult renders a Terraform-style diff list.
func printPlanResult(out io.Writer, res *orchestrator.Result) {
	if res == nil || len(res.Hosts) == 0 {
		fmt.Fprintln(out, "Nothing to do — inventory has no hosts.")
		return
	}
	for _, h := range res.Hosts {
		for _, d := range h.Diffs {
			fmt.Fprintln(out, d.Format())
		}
		for _, e := range h.Errors {
			fmt.Fprintf(out, "[%s] ERROR: %v\n", h.Host.Name, e)
		}
	}
	fmt.Fprintln(out)
	if res.NoOp() {
		fmt.Fprintln(out, "Plan: no changes — every host is already at target state.")
		return
	}
	hosts, actions := summarisePlan(res)
	fmt.Fprintf(out, "Plan: %d host(s) would change, %d action(s) total.\n", hosts, actions)
}

// summarisePlan counts hosts with at least one action and total actions.
// Used by both `plan` and `apply` summaries.
func summarisePlan(res *orchestrator.Result) (changedHosts, totalActions int) {
	for _, h := range res.Hosts {
		hostChanged := false
		for _, d := range h.Diffs {
			if !d.NoOp() {
				totalActions += len(d.Actions)
				hostChanged = true
			}
		}
		if hostChanged {
			changedHosts++
		}
	}
	return
}

