// Package tui hosts the Bubble Tea models that drive the interactive UI.
//
// Phase 1 introduces a multi-screen shell: Dashboard, Hosts list, Host
// detail, Help. Each screen is rendered by a method on rootModel. Real
// network-backed status and apply screens land in Phase 2 and 3.
package tui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rtorcato/homelab-nut/internal/inventory"
)

// screen identifies a top-level view in the TUI.
type screen int

const (
	screenDashboard screen = iota
	screenHosts
	screenHost
	screenApply
	screenHelp
)

func (s screen) String() string {
	switch s {
	case screenDashboard:
		return "Dashboard"
	case screenHosts:
		return "Hosts"
	case screenHost:
		return "Host"
	case screenApply:
		return "Apply"
	case screenHelp:
		return "Help"
	}
	return ""
}

// tabOrder is the navigation order for tab / shift+tab and number keys.
// screenHost isn't in the tab bar — you reach it by pressing enter on Hosts.
var tabOrder = []screen{screenDashboard, screenHosts, screenApply, screenHelp}

// New returns the root Bubble Tea model.
//
// inventoryPath is consulted at startup; if the file doesn't exist or
// fails to validate, the loadErr is shown in-place rather than crashing
// the TUI, so users get a clear "run init" prompt.
func New(version, inventoryPath string) tea.Model {
	m := rootModel{
		version:       version,
		inventoryPath: inventoryPath,
		current:       screenDashboard,
	}
	if inventoryPath != "" {
		inv, err := inventory.Load(inventoryPath)
		m.inv = inv
		m.loadErr = err
	}
	return m
}

type rootModel struct {
	version       string
	inventoryPath string
	inv           *inventory.Inventory
	loadErr       error
	current       screen
	selectedHost  int
	apply         applyState
	// exitAction is set by 'i' or 'e' keys before tea.Quit, so the
	// wrapping cobra command can dispatch a follow-up action
	// (run init forms, open $EDITOR) and then relaunch the TUI.
	// "" means a normal quit with no follow-up.
	exitAction    string
	width, height int
}

// ExitAction extracts the action the user requested via key shortcut
// before the TUI exited. Returns "init", "edit", or "" (normal quit).
// Designed for the cobra command that owns the TUI program to dispatch
// follow-up actions without needing to type-assert an unexported model.
func ExitAction(m tea.Model) string {
	if rm, ok := m.(rootModel); ok {
		return rm.exitAction
	}
	return ""
}

func (m rootModel) Init() tea.Cmd { return nil }

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case applyCompleteMsg:
		m.apply.elapsed = time.Since(m.apply.startedAt)
		m.apply.result = msg.result
		if msg.err != nil {
			m.apply.status = applyFailed
			m.apply.err = msg.err
		} else {
			m.apply.status = applyDone
		}
		if m.apply.logBuf != nil {
			m.apply.logBuf.WriteString(msg.logs)
		}
		return m, nil
	}
	return m, nil
}

func (m rootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys always take priority.
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.current == screenHost {
			m.current = screenHosts
		}
		return m, nil
	case "tab":
		m.current = cycle(m.current, +1)
		return m, nil
	case "shift+tab":
		m.current = cycle(m.current, -1)
		return m, nil
	case "1":
		m.current = screenDashboard
		return m, nil
	case "2":
		m.current = screenHosts
		return m, nil
	case "3":
		m.current = screenApply
		return m, nil
	case "4", "?":
		m.current = screenHelp
		return m, nil
	case "a", "A":
		if m.apply.status != applyRunning && m.inv != nil && len(m.inv.Hosts) > 0 {
			m.current = screenApply
			m.apply = applyState{
				status:    applyRunning,
				startedAt: time.Now(),
				logBuf:    new(bytes.Buffer),
			}
			return m, startApply(m.inv)
		}
		return m, nil
	case "i", "I":
		// init flow is only meaningful when there's no usable inventory —
		// otherwise the user should use `e` to edit. Refuse to trigger
		// init over an existing valid inventory to avoid surprises.
		if m.inv == nil || len(m.inv.Hosts) == 0 {
			m.exitAction = "init"
			return m, tea.Quit
		}
		return m, nil
	case "e", "E":
		// edit needs a file on disk to point $EDITOR at.
		if m.inventoryPath != "" {
			m.exitAction = "edit"
			return m, tea.Quit
		}
		return m, nil
	case "o", "O":
		openURL("https://github.com/rtorcato/homelab-nut")
		return m, nil
	}

	// Screen-local navigation.
	switch m.current {
	case screenHosts:
		switch msg.String() {
		case "up", "k":
			if m.selectedHost > 0 {
				m.selectedHost--
			}
		case "down", "j":
			if m.inv != nil && m.selectedHost < len(m.inv.Hosts)-1 {
				m.selectedHost++
			}
		case "enter":
			if m.inv != nil && len(m.inv.Hosts) > 0 {
				m.current = screenHost
			}
		}
	}
	return m, nil
}

// cycle returns the next screen in tabOrder. dir is +1 or -1.
func cycle(cur screen, dir int) screen {
	idx := 0
	for i, s := range tabOrder {
		if s == cur {
			idx = i
			break
		}
	}
	n := len(tabOrder)
	return tabOrder[(idx+dir+n)%n]
}

func (m rootModel) View() string {
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	switch m.current {
	case screenDashboard:
		b.WriteString(m.viewDashboard())
	case screenHosts:
		b.WriteString(m.viewHosts())
	case screenHost:
		b.WriteString(m.viewHost())
	case screenApply:
		b.WriteString(m.viewApply())
	case screenHelp:
		b.WriteString(m.viewHelp())
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	return b.String()
}

func (m rootModel) renderTabBar() string {
	title := titleStyle.Render(fmt.Sprintf("homelab-nut %s", m.version))
	tabs := make([]string, 0, len(tabOrder))
	for i, s := range tabOrder {
		label := fmt.Sprintf("%d %s", i+1, s)
		if s == m.current || (s == screenHosts && m.current == screenHost) {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, tabStyle.Render(label))
		}
	}
	return tabBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, title, strings.Repeat(" ", 2), strings.Join(tabs, " ")))
}

func (m rootModel) renderStatusBar() string {
	hints := []string{"tab cycles", "esc backs out", "? help", "q quits"}
	if m.current == screenHosts {
		hints = append([]string{"↑↓ select", "enter drill in"}, hints...)
	}
	return statusBarStyle.Render(strings.Join(hints, " · "))
}

func (m rootModel) viewDashboard() string {
	if m.inv == nil || m.loadErr != nil || len(m.inv.Hosts) == 0 {
		return bodyStyle.Render(m.emptyDashboard())
	}

	rolesByHost := func(h inventory.Host) string {
		strs := make([]string, len(h.Roles))
		for i, r := range h.Roles {
			strs[i] = r.String()
		}
		return strings.Join(strs, ", ")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %d host(s) configured at %s\n\n", titleStyle.Render("Inventory:"), len(m.inv.Hosts), m.inventoryPath)
	for _, h := range m.inv.Hosts {
		fmt.Fprintf(&b, "  %s · %s · %s\n", h.Name, h.Address, rolesByHost(h))
	}
	if m.inv.ShutdownDaemon != nil {
		d := m.inv.ShutdownDaemon
		fmt.Fprintf(&b, "\n%s threshold=%d%%  poll=%ds", titleStyle.Render("Daemon:"), d.Threshold, d.PollInterval)
	}
	b.WriteString("\n\nReal-time UPS status lands in Phase 3 (#4).")
	return bodyStyle.Render(b.String())
}

func (m rootModel) emptyDashboard() string {
	var msg string
	switch {
	case m.loadErr != nil && errors.Is(m.loadErr, errFileMissing):
		msg = "No inventory found at " + m.inventoryPath + "."
	case m.loadErr != nil:
		msg = "Could not load " + m.inventoryPath + ":\n  " + m.loadErr.Error()
	default:
		msg = "Inventory is empty."
	}
	return emptyStateStyle.Render(msg + "\n\nPress  i  to set up your inventory (or run `homelab-nut init` outside the TUI).")
}

func (m rootModel) viewHosts() string {
	if m.inv == nil || len(m.inv.Hosts) == 0 {
		return bodyStyle.Render(m.emptyDashboard())
	}
	var b strings.Builder
	for i, h := range m.inv.Hosts {
		line := fmt.Sprintf("%s  %s  %s", h.Name, h.Address, h.User)
		if i == m.selectedHost {
			fmt.Fprintf(&b, "▸ %s\n", activeHostItemStyle.Render(line))
		} else {
			fmt.Fprintf(&b, "  %s\n", hostItemStyle.Render(line))
		}
	}
	return bodyStyle.Render(b.String())
}

func (m rootModel) viewHost() string {
	if m.inv == nil || m.selectedHost >= len(m.inv.Hosts) {
		return bodyStyle.Render(emptyStateStyle.Render("No host selected. Press esc to return."))
	}
	h := m.inv.Hosts[m.selectedHost]
	roles := make([]string, len(h.Roles))
	for i, r := range h.Roles {
		roles[i] = r.String()
	}
	var b strings.Builder
	row := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render(label), value)
	}
	row("name", h.Name)
	row("address", h.Address)
	row("user", h.User)
	row("roles", strings.Join(roles, ", "))
	if h.UPS != nil {
		row("ups", fmt.Sprintf("name=%s driver=%s", h.UPS.Name, h.UPS.Driver))
	}
	if h.Shutdown != nil {
		row("shutdown", h.Shutdown.Command)
	}
	return bodyStyle.Render(b.String())
}

func (m rootModel) viewHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Keybindings"))
	b.WriteString("\n\n")
	rows := [][2]string{
		{"tab / shift+tab", "cycle screens"},
		{"1 / 2 / 3 / 4", "jump to Dashboard / Hosts / Apply / Help"},
		{"?", "open this help"},
		{"↑ ↓ / k j", "select host (Hosts screen)"},
		{"enter", "drill into selected host"},
		{"i", "set up inventory (empty-state Dashboard only)"},
		{"e", "edit inventory in $EDITOR (any screen)"},
		{"a / A", "run apply (any screen)"},
		{"o", "open the project page in your browser"},
		{"esc", "go back one screen"},
		{"q / ctrl+c", "quit"},
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s %s\n", labelStyle.Width(20).Render(r[0]), r[1])
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("More screens (apply, live status, logs) land in Phase 2 and 3."))
	return bodyStyle.Render(b.String())
}

// errFileMissing is a sentinel so the Dashboard can distinguish "no
// inventory file" from "file exists but failed to parse/validate".
var errFileMissing = errors.New("no inventory file")
