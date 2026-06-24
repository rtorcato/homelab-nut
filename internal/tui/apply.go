package tui

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/orchestrator"
	hssh "github.com/rtorcato/homelab-nut/internal/ssh"
)

// applyStatus is the lifecycle of an apply run from the TUI's view.
type applyStatus int

const (
	applyIdle applyStatus = iota
	applyRunning
	applyDone
	applyFailed
)

// applyState lives on rootModel — the Apply screen's local state.
//
// Mid-flight streaming of per-host progress is intentionally out of
// scope for v1: it'd require the orchestrator to emit events on a
// channel rather than just an io.Writer. v1 shows before/running/after
// states + the full captured log when apply completes.
type applyState struct {
	status   applyStatus
	startedAt time.Time
	elapsed  time.Duration
	logBuf   *bytes.Buffer
	result   *orchestrator.Result
	err      error
	// onlyHost is the single host this run targets, or "" for the whole
	// fleet. Set when apply is launched from a selected host.
	onlyHost string
}

// applyCompleteMsg is delivered when the background apply finishes.
type applyCompleteMsg struct {
	result *orchestrator.Result
	err    error
	logs   string
}

// startApply runs the orchestrator in a background goroutine and
// returns a tea.Cmd that delivers an applyCompleteMsg when done.
// onlyHost limits the run to a single host ("" = whole fleet).
// We don't try to stream progress to the TUI yet — see applyState.
func startApply(inv *inventory.Inventory, onlyHost string) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		res := orchestrator.Apply(context.Background(), inv, orchestrator.Options{
			SSHConfig: hssh.NewConfig(),
			OnlyHost:  onlyHost,
		}, &buf)
		var err error
		if res != nil && res.HasErrors() {
			err = fmt.Errorf("apply finished with errors on %d host(s)", countFailedHosts(res))
		}
		return applyCompleteMsg{
			result: res,
			err:    err,
			logs:   buf.String(),
		}
	}
}

// applyScopeLabel describes what an apply run targets: a single host or
// the whole fleet. Shown in the running/done headers so the user can tell
// a per-host apply from a fleet apply at a glance.
func applyScopeLabel(onlyHost string) string {
	if onlyHost != "" {
		return "— " + onlyHost
	}
	return "— whole fleet"
}

func countFailedHosts(res *orchestrator.Result) int {
	if res == nil {
		return 0
	}
	n := 0
	for _, h := range res.Hosts {
		if h.HasErrors() {
			n++
		}
	}
	return n
}

// viewApply renders the Apply screen given the current state.
func (m rootModel) viewApply() string {
	if m.inv == nil || len(m.inv.Hosts) == 0 {
		return bodyStyle.Render(m.emptyDashboard())
	}

	var b strings.Builder

	switch m.apply.status {
	case applyIdle:
		writeApplyPlan(&b, m.inv)

	case applyRunning:
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[int(time.Since(m.apply.startedAt).Milliseconds()/100)%10]
		fmt.Fprintln(&b, titleStyle.Render(fmt.Sprintf("%s Apply running… %s", spinner, applyScopeLabel(m.apply.onlyHost))))
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "Started %s ago\n", time.Since(m.apply.startedAt).Round(time.Second))
		fmt.Fprintln(&b, hintStyle.Render("Mid-flight progress streaming lands in a follow-up — see TODOS.md."))

	case applyDone:
		fmt.Fprintln(&b, lipgloss.NewStyle().Foreground(palette.primary).Bold(true).Render("✓ Apply complete "+applyScopeLabel(m.apply.onlyHost)))
		fmt.Fprintln(&b)
		writeApplySummary(&b, m.apply.result, m.apply.elapsed)

	case applyFailed:
		fmt.Fprintln(&b, lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Bold(true).Render("✗ Apply failed"))
		fmt.Fprintln(&b)
		if m.apply.err != nil {
			fmt.Fprintf(&b, "%s\n\n", m.apply.err)
		}
		writeApplySummary(&b, m.apply.result, m.apply.elapsed)
	}

	return bodyStyle.Render(b.String())
}

// writeApplyPlan renders the idle-state preview: per host, the roles that
// will run (in Apply's execution order) and a one-line description of what
// each does. It's a static, no-SSH plan — enough to answer "what will this
// do?" before the user commits. The live Detect-based "will change vs.
// already configured" split is a deliberate follow-up (would need SSH on
// the idle screen); see #78.
func writeApplyPlan(b *strings.Builder, inv *inventory.Inventory) {
	fmt.Fprintln(b, titleStyle.Render("Apply — set up NUT across your fleet"))
	fmt.Fprintln(b)
	fmt.Fprintln(b, "Connects to each host over SSH and runs its role scripts.")
	fmt.Fprintln(b, hintStyle.Render("Idempotent — safe to re-run; already-configured hosts converge with no harm."))
	fmt.Fprintln(b)

	order := orchestrator.RoleOrder()
	for i := range inv.Hosts {
		h := &inv.Hosts[i]
		fmt.Fprintf(b, "%s %s\n",
			hostNameStyle.Render(h.Name),
			hintStyle.Render(fmt.Sprintf("%s · %s", h.Address, h.User)))

		step := 0
		for _, r := range order {
			if !h.HasRole(r) {
				continue
			}
			step++
			fmt.Fprintf(b, "  %d. %-16s %s\n",
				step, r.String(), hintStyle.Render(roleActionLine(r, h, inv)))
		}
		if step == 0 {
			fmt.Fprintln(b, hintStyle.Render("  (no roles — nothing to apply)"))
		}
	}

	fmt.Fprintln(b)
	fmt.Fprintln(b, hintStyle.Render("press 'a' to apply the whole fleet · to apply just one host, select it on the Hosts screen and press 'a'"))
}

// roleActionLine is a short, inventory-aware description of what applying
// a role does on a host. Wording mirrors the roles' own Plan actions.
func roleActionLine(r inventory.Role, h *inventory.Host, inv *inventory.Inventory) string {
	switch r {
	case inventory.RoleNUTServer:
		if h.UPS != nil && h.UPS.Name != "" {
			return fmt.Sprintf("install NUT, configure UPS %q, start upsd + upsmon", h.UPS.Name)
		}
		return "install NUT, write configs, start upsd + upsmon"
	case inventory.RoleNUTClient:
		return "install nut-client + upsmon, point it at the NUT server"
	case inventory.RoleExporter:
		return "install the Prometheus NUT exporter (systemd service)"
	case inventory.RoleShutdownDaemon:
		if d := inv.ShutdownDaemon; d != nil {
			return fmt.Sprintf("install battery-watch daemon (threshold %d%%, poll %ds)", d.Threshold, d.PollInterval)
		}
		return "install the battery-watch shutdown daemon"
	case inventory.RoleShutdownTarget:
		cmd := "~/shutdown.sh"
		if h.Shutdown != nil && h.Shutdown.Command != "" {
			cmd = h.Shutdown.Command
		}
		return fmt.Sprintf("register graceful-shutdown command (%s)", cmd)
	default:
		return ""
	}
}

func writeApplySummary(b *strings.Builder, res *orchestrator.Result, elapsed time.Duration) {
	if res == nil {
		return
	}
	ok, failed := 0, 0
	for _, h := range res.Hosts {
		if h.HasErrors() {
			failed++
		} else {
			ok++
		}
	}
	fmt.Fprintf(b, "elapsed: %s · ok: %d · failed: %d\n\n", elapsed.Round(time.Second), ok, failed)
	for _, h := range res.Hosts {
		mark := "ok"
		if h.HasErrors() {
			mark = "FAIL"
		}
		fmt.Fprintf(b, "  [%s] %s\n", mark, h.Host.Name)
		for _, e := range h.Errors {
			fmt.Fprintf(b, "       %v\n", e)
		}
	}
}
