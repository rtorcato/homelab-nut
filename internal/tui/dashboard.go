package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/upspoll"
)

// Cadences for the live dashboard. Kept here (not user-configurable yet)
// so the View is deterministic for tests and matches the CLI defaults.
const (
	dashboardInterval = 5 * time.Second
	dashboardTimeout  = 2 * time.Second
)

// dashboardState is the live-poll slice of rootModel. lastErr captures
// inventory load problems separately from per-host poll errors, which
// live on each Row.
type dashboardState struct {
	rows    []upspoll.Row
	updated time.Time
	// inFlight is true between a poll cmd dispatch and its result, so
	// the tick doesn't pile up overlapping polls if a host is slow.
	inFlight bool
}

// dashboardTickMsg fires every dashboardInterval to kick off a poll.
type dashboardTickMsg struct{}

// dashboardUpdatedMsg carries fresh poll results into the model.
type dashboardUpdatedMsg struct {
	rows []upspoll.Row
	at   time.Time
}

// dashboardTick returns a tea.Cmd that emits dashboardTickMsg after d.
func dashboardTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return dashboardTickMsg{} })
}

// pollDashboard polls every nut-server host in inv and returns a
// dashboardUpdatedMsg. Designed to run inside a tea.Cmd goroutine —
// the per-poll timeout caps the wait so a dead host doesn't stall
// the whole dashboard.
func pollDashboard(inv *inventory.Inventory) tea.Cmd {
	return func() tea.Msg {
		if inv == nil {
			return dashboardUpdatedMsg{rows: nil, at: time.Now()}
		}
		hosts := inv.HostsWithRole(inventory.RoleNUTServer)
		ctx, cancel := context.WithTimeout(context.Background(), dashboardTimeout+time.Second)
		defer cancel()
		return dashboardUpdatedMsg{
			rows: upspoll.Poll(ctx, hosts, dashboardTimeout),
			at:   time.Now(),
		}
	}
}

// renderDashboardCards lays out one card per row using the available
// terminal width. Cards reflow into multiple per-row columns when the
// terminal is wide enough.
func renderDashboardCards(rows []upspoll.Row, width int) string {
	if len(rows) == 0 {
		return emptyStateStyle.Render(
			"No nut-server hosts in this inventory.\n\n" +
				"Add a host with role 'nut-server' (and a UPS block) to see live state.")
	}

	cards := make([]string, len(rows))
	for i, r := range rows {
		cards[i] = renderCard(r)
	}

	// One card per row when the terminal is narrow; pack columns when wide.
	cardWidth := lipgloss.Width(cards[0])
	usable := width - 4 // margin allowance inside bodyStyle
	cols := usable / (cardWidth + 2)
	if cols < 1 {
		cols = 1
	}
	if cols > len(cards) {
		cols = len(cards)
	}

	var b strings.Builder
	for i := 0; i < len(cards); i += cols {
		end := i + cols
		if end > len(cards) {
			end = len(cards)
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
		b.WriteString("\n")
	}
	return b.String()
}

// renderCard builds one fixed-width card for a single host.
func renderCard(r upspoll.Row) string {
	const innerWidth = 34

	// lipgloss.Width counts visible cells (skips ANSI codes); len() would
	// count escape bytes and collapse the name cell to a sliver.
	badge := statusBadge(r)
	nameWidth := innerWidth - lipgloss.Width(badge) - 1 // -1 keeps a gap so the badge doesn't touch the name
	if nameWidth < 1 {
		nameWidth = 1
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		hostNameStyle.Width(nameWidth).Render(r.Host),
		badge,
	)

	subhead := r.Address
	if r.UPS != "" {
		subhead += " · " + r.UPS
	}

	var body strings.Builder
	body.WriteString(header)
	body.WriteString("\n")
	body.WriteString(hintStyle.Render(subhead))
	body.WriteString("\n\n")

	if r.Error != "" {
		body.WriteString(errorStyle.Render("✗ " + truncateForCard(r.Error, innerWidth-2)))
		body.WriteString("\n")
	} else {
		body.WriteString(gaugeLine("Battery", r.BatteryCharge))
		body.WriteString("\n")
		body.WriteString(gaugeLine("Load", r.Load))
		body.WriteString("\n")
		body.WriteString(runtimeLine(r.BatteryRuntime))
		body.WriteString("\n")
	}

	return cardStyle.Width(innerWidth + 2).Render(body.String())
}

// statusBadge returns the lipgloss-styled, padded status chip for the
// upper right of a card. Color reflects severity, not the raw string.
func statusBadge(r upspoll.Row) string {
	if r.Error != "" {
		return badgeErrorStyle.Render(" ERR ")
	}
	switch {
	case strings.Contains(r.Status, "LB"):
		return badgeErrorStyle.Render(" " + r.Status + " ")
	case strings.HasPrefix(r.Status, "OB"):
		return badgeWarnStyle.Render(" " + r.Status + " ")
	case r.Status == "":
		return badgeMutedStyle.Render(" ?  ")
	default:
		return badgeOKStyle.Render(" " + r.Status + " ")
	}
}

// gaugeLine renders a labeled 10-cell ASCII progress bar:
//
//	Battery [████████░░] 85%
//
// nil values render as "[----------] -" so the card height stays stable
// regardless of which vars the upsd reports.
func gaugeLine(label string, p *float64) string {
	const cells = 10
	if p == nil {
		return fmt.Sprintf("%-8s [%s] -",
			label, strings.Repeat("-", cells))
	}
	fill := int(*p / 10)
	if fill > cells {
		fill = cells
	}
	if fill < 0 {
		fill = 0
	}
	bar := strings.Repeat("█", fill) + strings.Repeat("░", cells-fill)
	return fmt.Sprintf("%-8s [%s] %d%%", label, bar, int(*p))
}

func runtimeLine(seconds *int) string {
	if seconds == nil {
		return fmt.Sprintf("%-8s -", "Runtime")
	}
	d := time.Duration(*seconds) * time.Second
	return fmt.Sprintf("%-8s %s", "Runtime", d.Round(time.Second))
}

func truncateForCard(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 2 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// statusLegend describes every status code the dashboard renders and
// what it means. Single source of truth — viewHelp renders the badges
// inline using statusBadge() so the legend stays accurate as the badge
// rules evolve.
var statusLegend = []struct {
	Row     upspoll.Row
	Meaning string
}{
	{upspoll.Row{Status: "OL"}, "On Line — utility power, healthy"},
	{upspoll.Row{Status: "OL CHRG"}, "On Line, battery is charging"},
	{upspoll.Row{Status: "OB"}, "On Battery — utility power out, UPS sustaining load"},
	{upspoll.Row{Status: "OB LB"}, "On Battery + Low Battery — imminent shutdown"},
	{upspoll.Row{Status: "LB"}, "Low Battery (regardless of OL/OB)"},
	{upspoll.Row{Error: "x"}, "host unreachable or protocol error"},
	{upspoll.Row{}, "no status reported by upsd"},
}

// dashboardFooter is the tagline under the cards: "X hosts · refreshed Hh:mm:ss · r to refresh".
func dashboardFooter(rows []upspoll.Row, updated time.Time) string {
	if updated.IsZero() {
		return hintStyle.Render(fmt.Sprintf("%d host(s) · polling…  ·  r to refresh now",
			len(rows)))
	}
	return hintStyle.Render(fmt.Sprintf("%d host(s) · refreshed %s  ·  r to refresh now",
		len(rows), updated.Format("15:04:05")))
}
