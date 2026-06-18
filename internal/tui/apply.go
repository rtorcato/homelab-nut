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
}

// applyCompleteMsg is delivered when the background apply finishes.
type applyCompleteMsg struct {
	result *orchestrator.Result
	err    error
	logs   string
}

// startApply runs the orchestrator in a background goroutine and
// returns a tea.Cmd that delivers an applyCompleteMsg when done.
// We don't try to stream progress to the TUI yet — see applyState.
func startApply(inv *inventory.Inventory) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		res := orchestrator.Apply(context.Background(), inv, orchestrator.Options{
			SSHConfig: hssh.NewConfig(),
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
		fmt.Fprintln(&b, titleStyle.Render("Ready to apply"))
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "%d host(s) in inventory:\n", len(m.inv.Hosts))
		for _, h := range m.inv.Hosts {
			fmt.Fprintf(&b, "  %s · %s\n", h.Name, summariseHostRoles(&h))
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, hintStyle.Render("press 'a' to start apply — runs the same orchestrator as `homelab-nut apply`"))

	case applyRunning:
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[int(time.Since(m.apply.startedAt).Milliseconds()/100)%10]
		fmt.Fprintln(&b, titleStyle.Render(fmt.Sprintf("%s Apply running…", spinner)))
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "Started %s ago\n", time.Since(m.apply.startedAt).Round(time.Second))
		fmt.Fprintln(&b, hintStyle.Render("Mid-flight progress streaming lands in a follow-up — see TODOS.md."))

	case applyDone:
		fmt.Fprintln(&b, lipgloss.NewStyle().Foreground(palette.primary).Bold(true).Render("✓ Apply complete"))
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

func summariseHostRoles(h *inventory.Host) string {
	parts := make([]string, len(h.Roles))
	for i, r := range h.Roles {
		parts[i] = r.String()
	}
	return strings.Join(parts, ", ")
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
