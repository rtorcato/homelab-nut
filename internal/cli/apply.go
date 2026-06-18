package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rtorcato/homelab-nut/internal/orchestrator"
	hssh "github.com/rtorcato/homelab-nut/internal/ssh"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Plan + execute changes on each host over SSH",
		Long: `Loads the inventory, plans what would change on each host, prompts
for confirmation (unless --auto-approve), then executes Apply for each
role with a non-empty diff. Streams each role's output prefixed with
[host/role] so concurrent fleet output stays attributable.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			autoApprove, _ := cmd.Flags().GetBool("auto-approve")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			return runApply(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), path, autoApprove, concurrency, getOutputFormat(cmd))
		},
	}
	cmd.Flags().BoolP("auto-approve", "y", false, "skip the y/N confirmation prompt before applying")
	cmd.Flags().Int("concurrency", 0, "max hosts to apply against in parallel (0 = unlimited)")
	addOutputFlag(cmd)
	return cmd
}

func runApply(stdin io.Reader, stdout, stderr io.Writer, path string, autoApprove bool, concurrency int, format outputFormat) error {
	inv, err := loadInventoryOrReport(stderr, path)
	if err != nil {
		return err
	}

	// 1. Plan first so the user sees what's about to happen.
	planRes := orchestrator.Plan(commandContext(), inv, orchestrator.Options{
		SSHConfig:      hssh.NewConfig(),
		MaxConcurrency: concurrency,
	})
	if format == outputText {
		printPlanResult(stdout, planRes)
	}

	if planRes.HasErrors() {
		fmt.Fprintln(stderr, "Plan reported errors — fix them before re-running apply.")
		return errSilent
	}
	if planRes.NoOp() {
		if format == outputJSON {
			return emitJSON(stdout, applySummary{Elapsed: "0s", NoOp: true})
		}
		return nil
	}

	// 2. Confirm — text-mode interactive only. JSON callers must pass --auto-approve.
	if !autoApprove {
		if format == outputJSON {
			fmt.Fprintln(stderr, "apply -o json requires --auto-approve (no interactive prompt in JSON mode)")
			return errSilent
		}
		fmt.Fprint(stdout, "\nApply these changes? [y/N] ")
		ok, err := confirm(stdin)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(stdout, "Aborted. Nothing was changed.")
			return nil
		}
	}

	// 3. Apply. JSON mode discards the streamed per-role output (it'd
	// pollute the JSON document) — the final summary captures what
	// happened. Text mode streams as before.
	applyOut := stdout
	if format == outputJSON {
		applyOut = io.Discard
	} else {
		fmt.Fprintln(stdout)
	}
	start := time.Now()
	res := orchestrator.Apply(commandContext(), inv, orchestrator.Options{
		SSHConfig:      hssh.NewConfig(),
		MaxConcurrency: concurrency,
	}, applyOut)

	// 4. Summary.
	changedHosts, totalActions := summarisePlan(res)
	failed := 0
	for _, h := range res.Hosts {
		if h.HasErrors() {
			failed++
		}
	}
	elapsed := time.Since(start).Round(time.Second)

	if format == outputJSON {
		summary := applySummary{
			Elapsed:      elapsed.String(),
			HostsChanged: changedHosts,
			Actions:      totalActions,
			Failed:       failed,
			Result:       res,
		}
		_ = emitJSON(stdout, summary)
	} else {
		fmt.Fprintln(stdout)
		for _, h := range res.Hosts {
			if h.HasErrors() {
				for _, e := range h.Errors {
					fmt.Fprintf(stderr, "[%s] %v\n", h.Host.Name, e)
				}
			}
		}
		fmt.Fprintf(stdout, "Apply complete in %s: %d host(s) changed, %d action(s), %d failed.\n",
			elapsed, changedHosts, totalActions, failed)
	}

	if failed > 0 || res.HasErrors() {
		return errSilent
	}
	return nil
}

// applySummary is the JSON shape emitted by `apply -o json`. Embeds the
// full orchestrator.Result so consumers can drill into per-host detail.
type applySummary struct {
	Elapsed      string                 `json:"elapsed"`
	HostsChanged int                    `json:"hosts_changed"`
	Actions      int                    `json:"actions"`
	Failed       int                    `json:"failed"`
	NoOp         bool                   `json:"noop,omitempty"`
	Result       *orchestrator.Result   `json:"result,omitempty"`
}

func confirm(in io.Reader) (bool, error) {
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}
