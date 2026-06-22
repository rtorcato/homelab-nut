package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/upspoll"
)

// maxRoleCommandLen caps a shutdown-target command in the Roles column so
// long script paths don't blow out the table width.
const maxRoleCommandLen = 24

// roleLine is one rendered role for a host: a compact, inventory-derived
// summary plus the State-column value that belongs to that role.
//
// nut-server roles carry live UPS state (reused from the Dashboard poll);
// every other role is simply "configured" — we know it's declared in the
// inventory but don't yet probe whether it's healthy. Live daemon/target
// health checks are a deliberate follow-up (see issue #64 "Out of scope").
type roleLine struct {
	summary string
	state   string
}

// viewHosts renders the Hosts tab. Wide terminals (>= 100 cols) get an
// aligned table; narrower ones reflow to a stacked block per host so role
// info is never truncated. Both layouts reuse the same role/state helpers.
func (m rootModel) viewHosts() string {
	if m.inv == nil || len(m.inv.Hosts) == 0 {
		return bodyStyle.Render(m.emptyDashboard())
	}

	width := m.width
	if width <= 0 {
		width = 80
	}

	rowByHost := indexRows(m.dashboard.rows)
	if width >= 100 {
		return bodyStyle.Render(m.renderHostsTable(rowByHost))
	}
	return bodyStyle.Render(m.renderHostsBlocks(rowByHost))
}

// indexRows keys the Dashboard's poll results by host name for O(1) lookup
// while rendering rows.
func indexRows(rows []upspoll.Row) map[string]upspoll.Row {
	out := make(map[string]upspoll.Row, len(rows))
	for _, r := range rows {
		out[r.Host] = r
	}
	return out
}

// hostRoleLines builds the per-role summary + state for a single host,
// preserving the inventory's role order.
func (m rootModel) hostRoleLines(h inventory.Host, rowByHost map[string]upspoll.Row) []roleLine {
	lines := make([]roleLine, 0, len(h.Roles))
	for _, r := range h.Roles {
		switch r {
		case inventory.RoleNUTServer:
			summary := "nut-server"
			if h.UPS != nil && h.UPS.Name != "" {
				summary = fmt.Sprintf("nut-server (%s)", h.UPS.Name)
			}
			lines = append(lines, roleLine{summary: summary, state: m.nutServerState(h, rowByHost)})
		case inventory.RoleShutdownDaemon:
			summary := "shutdown-daemon"
			if d := m.inv.ShutdownDaemon; d != nil {
				tgt := len(m.inv.HostsWithRole(inventory.RoleShutdownTarget))
				summary = fmt.Sprintf("shutdown-daemon (%d%%, %ds, %d tgt)", d.Threshold, d.PollInterval, tgt)
			}
			lines = append(lines, roleLine{summary: summary, state: "configured"})
		case inventory.RoleShutdownTarget:
			summary := "shutdown-target"
			if h.Shutdown != nil && h.Shutdown.Command != "" {
				summary = fmt.Sprintf("shutdown-target (%s)", truncateCommand(h.Shutdown.Command, maxRoleCommandLen))
			}
			lines = append(lines, roleLine{summary: summary, state: "configured"})
		default:
			lines = append(lines, roleLine{summary: r.String(), state: "configured"})
		}
	}
	if len(lines) == 0 {
		lines = append(lines, roleLine{summary: "—", state: "—"})
	}
	return lines
}

// nutServerState turns the Dashboard's poll Row into a compact State-column
// string ("OL · 100%", "ERR", "polling…"). It never issues new SSH calls —
// it only reads what the Dashboard tick already gathered.
func (m rootModel) nutServerState(h inventory.Host, rowByHost map[string]upspoll.Row) string {
	r, ok := rowByHost[h.Name]
	if !ok {
		return hintStyle.Render("polling…")
	}
	if r.Error != "" {
		return errorStyle.Render("ERR")
	}
	status := r.Status
	if status == "" {
		status = "?"
	}
	if r.BatteryCharge != nil {
		return fmt.Sprintf("%s · %d%%", status, int(*r.BatteryCharge))
	}
	return status
}

// renderHostsTable lays out an aligned table for wide terminals. Each host
// owns one primary line (name/address/user + its first role) plus a
// continuation line per additional role, indented under the Roles column.
func (m rootModel) renderHostsTable(rowByHost map[string]upspoll.Row) string {
	hosts := m.inv.Hosts

	// Pre-compute role lines and column widths in a single pass.
	perHost := make([][]roleLine, len(hosts))
	nameW := lipgloss.Width("NAME")
	addrW := lipgloss.Width("ADDRESS")
	userW := lipgloss.Width("USER")
	roleW := lipgloss.Width("ROLES")
	for i, h := range hosts {
		perHost[i] = m.hostRoleLines(h, rowByHost)
		nameW = max(nameW, lipgloss.Width(h.Name))
		addrW = max(addrW, lipgloss.Width(h.Address))
		userW = max(userW, lipgloss.Width(h.User))
		for _, rl := range perHost[i] {
			roleW = max(roleW, lipgloss.Width(rl.summary))
		}
	}

	var b strings.Builder
	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %s",
		nameW, "NAME", addrW, "ADDRESS", userW, "USER", roleW, "ROLES", "STATE")
	b.WriteString(hintStyle.Render(header))
	b.WriteString("\n")

	// Continuation lines sit under the Roles column: cursor + the three
	// preceding cells + their inter-column gaps.
	contIndent := strings.Repeat(" ", 2+nameW+2+addrW+2+userW+2)

	for i, h := range hosts {
		lines := perHost[i]
		selected := i == m.selectedHost
		cursor := "  "
		if selected {
			cursor = "▸ "
		}

		left := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s",
			nameW, h.Name, addrW, h.Address, userW, h.User, roleW, lines[0].summary)
		if selected {
			left = selectedRowStyle.Render(left)
		}
		fmt.Fprintf(&b, "%s%s  %s\n", cursor, left, lines[0].state)

		for _, rl := range lines[1:] {
			fmt.Fprintf(&b, "%s%-*s  %s\n", contIndent, roleW, rl.summary, rl.state)
		}
	}
	return b.String()
}

// renderHostsBlocks is the narrow-terminal fallback: a stacked mini-block
// per host so role summaries are never truncated to fit a column.
func (m rootModel) renderHostsBlocks(rowByHost map[string]upspoll.Row) string {
	hosts := m.inv.Hosts

	perHost := make([][]roleLine, len(hosts))
	roleW := 0
	for i, h := range hosts {
		perHost[i] = m.hostRoleLines(h, rowByHost)
		for _, rl := range perHost[i] {
			roleW = max(roleW, lipgloss.Width(rl.summary))
		}
	}

	var b strings.Builder
	for i, h := range hosts {
		selected := i == m.selectedHost
		cursor := "  "
		name := h.Name
		if selected {
			cursor = "▸ "
			name = selectedRowStyle.Render(name)
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, name)
		fmt.Fprintf(&b, "    %s\n", hintStyle.Render(h.Address+" · "+h.User))
		for _, rl := range perHost[i] {
			fmt.Fprintf(&b, "    %-*s  %s\n", roleW, rl.summary, rl.state)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// truncateCommand shortens a shutdown command to n display cells, appending
// an ellipsis. Rune-aware so multi-byte commands don't get cut mid-glyph.
func truncateCommand(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		return string(r[:0])
	}
	return string(r[:n-1]) + "…"
}
