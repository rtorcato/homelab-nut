package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/orchestrator"
	"github.com/rtorcato/homelab-nut/internal/roles"
	hssh "github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall [host]",
		Short: "Remove homelab-nut's services + binaries from a host (the inverse of apply)",
		Long: `Reverses what apply installed on a host: stops + removes the custom
systemd units (nut-exporter, ups-battery-shutdown), their binaries, the
config they wrote, and the shutdown-target sudoers rule + script. The
upstream NUT package is left alone unless you pass --purge-nut.

Built on the same orchestrator as apply, so single-host and fleet-wide
(--all) share one code path, with the same --concurrency and the same
--auto-approve / -o json gating. Removals are idempotent — re-running on
an already-clean host is a no-op that reports everything as skipped.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := inventoryPath(cmd)
			var host string
			if len(args) == 1 {
				host = args[0]
			}
			all, _ := cmd.Flags().GetBool("all")
			roleFilter, _ := cmd.Flags().GetString("role")
			purgeNUT, _ := cmd.Flags().GetBool("purge-nut")
			autoApprove, _ := cmd.Flags().GetBool("auto-approve")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			return runUninstall(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				path, host, all, roleFilter, purgeNUT, autoApprove, concurrency, getOutputFormat(cmd))
		},
	}
	cmd.Flags().Bool("all", false, "uninstall every host in the inventory (required if no host is given)")
	cmd.Flags().String("role", "all", "limit to one role: nut-server|nut-client|exporter|shutdown-daemon|shutdown-target|all")
	cmd.Flags().Bool("purge-nut", false, "also apt-purge the upstream NUT package and delete /etc/nut (destructive)")
	cmd.Flags().BoolP("auto-approve", "y", false, "skip the y/N confirmation prompt before removing")
	cmd.Flags().Int("concurrency", 0, "max hosts to uninstall in parallel (0 = unlimited)")
	addOutputFlag(cmd)
	return cmd
}

func runUninstall(stdin io.Reader, stdout, stderr io.Writer, path, onlyHost string, all bool, roleFilter string, purgeNUT, autoApprove bool, concurrency int, format outputFormat) error {
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}

	// Safety: a bare `uninstall` with no host and no --all would tear down
	// the whole fleet. Require an explicit opt-in either way.
	if onlyHost == "" && !all {
		fmt.Fprintln(stderr, "refusing to uninstall the whole fleet implicitly — pass a host name, or --all to mean every host")
		return errSilent
	}
	if onlyHost != "" && inv.HostByName(onlyHost) == nil {
		fmt.Fprintf(stderr, "host %q not found in inventory\n", onlyHost)
		return errSilent
	}

	// Resolve the --role filter. "all"/"" means every applicable role.
	var onlyRole inventory.Role
	if roleFilter != "" && roleFilter != "all" {
		onlyRole = inventory.Role(roleFilter)
		if !onlyRole.Valid() {
			fmt.Fprintf(stderr, "unknown --role %q (valid: %s, or all)\n", roleFilter, joinRoleNames())
			return errSilent
		}
	}

	// 1. Preview what's about to be removed (text mode), so the confirm
	//    prompt isn't a leap of faith.
	targets := selectUninstallTargets(inv, onlyHost)
	if len(targets) == 0 {
		fmt.Fprintln(stderr, "no matching hosts to uninstall")
		return errSilent
	}
	if format == outputText {
		printUninstallPreview(stdout, targets, onlyRole, purgeNUT)
	}

	// 2. Confirm — text-mode interactive only. JSON callers must --auto-approve.
	if !autoApprove {
		if format == outputJSON {
			fmt.Fprintln(stderr, "uninstall -o json requires --auto-approve (no interactive prompt in JSON mode)")
			return errSilent
		}
		prompt := "\nRemove these? [y/N] "
		if purgeNUT {
			prompt = "\n⚠️  --purge-nut will apt-purge NUT and delete /etc/nut. Proceed? [y/N] "
		}
		fmt.Fprint(stdout, prompt)
		ok, err := confirm(stdin)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(stdout, "Aborted. Nothing was removed.")
			return nil
		}
	}

	// 3. Run. JSON mode discards the streamed output (it'd pollute the
	//    document); the summary captures what happened.
	out := stdout
	if format == outputJSON {
		out = io.Discard
	} else {
		fmt.Fprintln(stdout)
	}
	start := time.Now()
	res := orchestrator.Uninstall(commandContext(), inv, orchestrator.Options{
		SSHConfig:      hssh.NewConfig(),
		MaxConcurrency: concurrency,
		OnlyHost:       onlyHost,
		OnlyRole:       onlyRole,
		Uninstall:      roles.UninstallParams{PurgeNUT: purgeNUT},
	}, out)
	elapsed := time.Since(start).Round(time.Second)

	// 4. Summary + exit code.
	removedCount, failed := summariseUninstall(res)
	if format == outputJSON {
		_ = emitJSON(stdout, buildUninstallSummary(res, elapsed, removedCount, failed))
	} else {
		fmt.Fprintln(stdout)
		printUninstallResult(stdout, stderr, res)
		fmt.Fprintf(stdout, "Uninstall complete in %s: %d item(s) removed across %d host(s), %d failed.\n",
			elapsed, removedCount, len(res.Hosts), failed)
	}

	if failed > 0 || res.HasErrors() {
		return errExit(ExitApplyPartial)
	}
	if res.NothingRemoved() {
		return errExit(ExitNothingToRemove)
	}
	return nil
}

// selectUninstallTargets returns the hosts an uninstall run will touch:
// just the named one, or all of them. Mirrors the orchestrator's own
// target selection so the preview matches what actually runs.
func selectUninstallTargets(inv *inventory.Inventory, onlyHost string) []*inventory.Host {
	out := make([]*inventory.Host, 0, len(inv.Hosts))
	for i := range inv.Hosts {
		h := &inv.Hosts[i]
		if onlyHost != "" && h.Name != onlyHost {
			continue
		}
		out = append(out, h)
	}
	return out
}

// printUninstallPreview lists, per host, the roles that will be removed,
// honoring the --role filter. A static preview (no SSH) — the live run
// reports what was actually present vs already gone.
func printUninstallPreview(w io.Writer, targets []*inventory.Host, onlyRole inventory.Role, purgeNUT bool) {
	fmt.Fprintln(w, "About to uninstall:")
	for _, h := range targets {
		names := make([]string, 0, len(h.Roles))
		for _, r := range orchestrator.RoleOrder() {
			if onlyRole != "" && r != onlyRole {
				continue
			}
			if h.HasRole(r) {
				names = append(names, string(r))
			}
		}
		if len(names) == 0 {
			fmt.Fprintf(w, "  %s — (no matching roles)\n", h.Name)
			continue
		}
		fmt.Fprintf(w, "  %s — %s\n", h.Name, strings.Join(names, ", "))
	}
	if purgeNUT {
		fmt.Fprintln(w, "\n  --purge-nut: upstream NUT package + /etc/nut will be removed on nut-server/nut-client hosts.")
	} else {
		fmt.Fprintln(w, "\n  (upstream NUT package left in place — pass --purge-nut to remove it too)")
	}
}

// printUninstallResult prints each host's removed/skipped items and any
// errors in text mode.
func printUninstallResult(stdout, stderr io.Writer, res *orchestrator.Result) {
	for _, h := range res.Hosts {
		removed, skipped := flattenRemovals(h)
		fmt.Fprintf(stdout, "[%s]\n", h.Host.Name)
		for _, r := range removed {
			fmt.Fprintf(stdout, "  removed: %s\n", r)
		}
		for _, s := range skipped {
			fmt.Fprintf(stdout, "  skipped: %s\n", s)
		}
		if len(removed) == 0 && len(skipped) == 0 {
			fmt.Fprintln(stdout, "  (nothing)")
		}
		for _, e := range h.Errors {
			fmt.Fprintf(stderr, "  error: %v\n", e)
		}
	}
}

// flattenRemovals collapses a host's per-role Removals into flat
// removed/skipped string lists for the host-level summary + JSON contract.
func flattenRemovals(h *orchestrator.HostResult) (removed, skipped []string) {
	for _, rem := range h.Removals {
		removed = append(removed, rem.Removed...)
		skipped = append(skipped, rem.Skipped...)
	}
	return removed, skipped
}

// summariseUninstall returns the total count of removed items and the
// number of hosts that errored.
func summariseUninstall(res *orchestrator.Result) (removed, failed int) {
	for _, h := range res.Hosts {
		r, _ := flattenRemovals(h)
		removed += len(r)
		if h.HasErrors() {
			failed++
		}
	}
	return removed, failed
}

// uninstallHostResult is the per-host JSON shape (flattened across roles),
// matching the contract documented in AGENTS.md.
type uninstallHostResult struct {
	Host    string   `json:"host"`
	Removed []string `json:"removed"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors"`
}

// uninstallSummary is the JSON emitted by `uninstall -o json`.
type uninstallSummary struct {
	Elapsed string                `json:"elapsed"`
	Removed int                   `json:"removed"`
	Failed  int                   `json:"failed"`
	Results []uninstallHostResult `json:"results"`
}

func buildUninstallSummary(res *orchestrator.Result, elapsed time.Duration, removedCount, failed int) uninstallSummary {
	results := make([]uninstallHostResult, 0, len(res.Hosts))
	for _, h := range res.Hosts {
		removed, skipped := flattenRemovals(h)
		results = append(results, uninstallHostResult{
			Host:    h.Host.Name,
			Removed: removed,
			Skipped: skipped,
			Errors:  h.ErrorStrings(),
		})
	}
	return uninstallSummary{
		Elapsed: elapsed.String(),
		Removed: removedCount,
		Failed:  failed,
		Results: results,
	}
}

// joinRoleNames lists the valid role names for error messages.
func joinRoleNames() string {
	names := make([]string, len(inventory.AllRoles))
	for i, r := range inventory.AllRoles {
		names[i] = string(r)
	}
	return strings.Join(names, ", ")
}
