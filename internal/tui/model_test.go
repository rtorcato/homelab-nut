package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/upspoll"
)

// fixtureInventory returns a small but role-diverse inventory for tests.
func fixtureInventory() *inventory.Inventory {
	return &inventory.Inventory{
		Hosts: []inventory.Host{
			{
				Name: "pi", Address: "192.0.2.10", User: "pi",
				Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleShutdownDaemon},
				UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
			},
			{
				Name: "ws", Address: "192.0.2.20", User: "admin",
				Roles:    []inventory.Role{inventory.RoleShutdownTarget},
				Shutdown: &inventory.Shutdown{Command: "~/shutdown.sh"},
			},
		},
		ShutdownDaemon: &inventory.ShutdownDaemon{Threshold: 50, PollInterval: 30},
	}
}

// modelWithInventory returns a rootModel pre-loaded with the given inventory.
func modelWithInventory(inv *inventory.Inventory) rootModel {
	return rootModel{
		version: "test",
		current: screenDashboard,
		inv:     inv,
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(t *testing.T, m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	t.Helper()
	return m.Update(msg)
}

func TestInitialScreenIsDashboard(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	if m.current != screenDashboard {
		t.Errorf("current = %v, want %v", m.current, screenDashboard)
	}
}

func TestTabCyclesScreens(t *testing.T) {
	m := tea.Model(modelWithInventory(fixtureInventory()))

	var cmd tea.Cmd
	expected := []screen{screenHosts, screenHelp, screenDashboard, screenHosts, screenHelp}
	for i, want := range expected {
		m, cmd = send(t, m, key("tab"))
		_ = cmd
		if got := m.(rootModel).current; got != want {
			t.Errorf("after tab #%d: current = %v, want %v", i+1, got, want)
		}
	}
}

func TestShiftTabCyclesBackward(t *testing.T) {
	m := tea.Model(modelWithInventory(fixtureInventory()))
	m, _ = send(t, m, key("shift+tab"))
	if got := m.(rootModel).current; got != screenHelp {
		t.Errorf("shift+tab from Dashboard: current = %v, want Help", got)
	}
}

func TestNumberKeysJumpToScreens(t *testing.T) {
	cases := []struct {
		key  string
		want screen
	}{
		{"1", screenDashboard},
		{"2", screenHosts},
		{"3", screenHelp},
		{"?", screenHelp},
	}
	for _, tc := range cases {
		m := tea.Model(modelWithInventory(fixtureInventory()))
		m, _ = send(t, m, key(tc.key))
		if got := m.(rootModel).current; got != tc.want {
			t.Errorf("key %q -> %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestHostsScreenSelectionAndDrillIn(t *testing.T) {
	m := tea.Model(modelWithInventory(fixtureInventory()))
	// jump to Hosts
	m, _ = send(t, m, key("2"))
	if got := m.(rootModel).current; got != screenHosts {
		t.Fatalf("expected Hosts, got %v", got)
	}
	// down once -> selectedHost == 1
	m, _ = send(t, m, key("down"))
	if got := m.(rootModel).selectedHost; got != 1 {
		t.Errorf("after down: selectedHost = %d, want 1", got)
	}
	// down at end of list -> no movement
	m, _ = send(t, m, key("down"))
	if got := m.(rootModel).selectedHost; got != 1 {
		t.Errorf("clamp: selectedHost = %d, want 1", got)
	}
	// up -> 0
	m, _ = send(t, m, key("up"))
	if got := m.(rootModel).selectedHost; got != 0 {
		t.Errorf("after up: selectedHost = %d, want 0", got)
	}
	// enter -> drill in to Host detail
	m, _ = send(t, m, key("enter"))
	if got := m.(rootModel).current; got != screenHost {
		t.Errorf("after enter: current = %v, want Host", got)
	}
	// esc -> back to Hosts
	m, _ = send(t, m, key("esc"))
	if got := m.(rootModel).current; got != screenHosts {
		t.Errorf("after esc: current = %v, want Hosts", got)
	}
}

func TestEscBacksOutToDashboard(t *testing.T) {
	m := tea.Model(modelWithInventory(fixtureInventory()))
	// Navigate to Help, then esc should return to the Dashboard rather than
	// being a no-op.
	m, _ = send(t, m, key("3"))
	if got := m.(rootModel).current; got != screenHelp {
		t.Fatalf("after '3': current = %v, want Help", got)
	}
	m, _ = send(t, m, key("esc"))
	if got := m.(rootModel).current; got != screenDashboard {
		t.Errorf("after esc on Help: current = %v, want Dashboard", got)
	}
}

func TestApplyKeyOnHostScreensAppliesSelectedHost(t *testing.T) {
	// 'a' on the Hosts list or a host's detail screen converges that one host
	// over SSH (suspends the TUI, like 'e'/'s') — not a whole-fleet apply and
	// not the removed Apply screen.
	for _, start := range []screen{screenHosts, screenHost} {
		m := modelWithInventory(fixtureInventory())
		m.current = start
		m.selectedHost = 1
		next, cmd := tea.Model(m).Update(key("a"))
		rm := next.(rootModel)
		if rm.exitAction != "apply-host" {
			t.Errorf("'a' on %v: exitAction = %q, want apply-host", start, rm.exitAction)
		}
		if rm.exitHostIdx != 1 {
			t.Errorf("'a' on %v: exitHostIdx = %d, want 1", start, rm.exitHostIdx)
		}
		if cmd == nil {
			t.Errorf("'a' on %v should return tea.Quit", start)
		}
		if got := ExitHostIndex(rm); got != 1 {
			t.Errorf("'a' on %v: ExitHostIndex() = %d, want 1", start, got)
		}
	}
}

func TestApplyKeyIgnoredOffHostScreens(t *testing.T) {
	// Apply is host-scoped now — on the Dashboard (no unambiguous selected
	// host) 'a' is a no-op, mirroring 's' and 'd'.
	m := modelWithInventory(fixtureInventory()) // current == Dashboard
	next, cmd := tea.Model(m).Update(key("a"))
	rm := next.(rootModel)
	if rm.exitAction != "" {
		t.Errorf("'a' on Dashboard set exitAction = %q, want empty", rm.exitAction)
	}
	if cmd != nil {
		t.Error("'a' on Dashboard should be a no-op (nil cmd)")
	}
}

func TestIKeyOnEmptyInventorySetsInitExitAction(t *testing.T) {
	m := rootModel{version: "test", current: screenDashboard, inv: nil, inventoryPath: "homelab-nut.yaml"}
	newModel, cmd := tea.Model(m).Update(key("i"))
	rm := newModel.(rootModel)
	if rm.exitAction != "init" {
		t.Errorf("'i' on empty Dashboard: exitAction = %q, want %q", rm.exitAction, "init")
	}
	if cmd == nil {
		t.Error("'i' should return tea.Quit")
	}
	if got := ExitAction(rm); got != "init" {
		t.Errorf("ExitAction() = %q, want init", got)
	}
}

func TestIKeyIgnoredWithPopulatedInventory(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	newModel, _ := tea.Model(m).Update(key("i"))
	if got := newModel.(rootModel).exitAction; got != "" {
		t.Errorf("'i' over populated inventory should be ignored, exitAction = %q", got)
	}
}

func TestEKeySetsEditExitAction(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.inventoryPath = "homelab-nut.yaml"
	newModel, cmd := tea.Model(m).Update(key("e"))
	rm := newModel.(rootModel)
	if rm.exitAction != "edit" {
		t.Errorf("'e' Hosts screen: exitAction = %q, want edit", rm.exitAction)
	}
	if cmd == nil {
		t.Error("'e' should return tea.Quit")
	}
}

func TestEKeyIgnoredWhenNoInventoryPath(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.inventoryPath = "" // unusual but possible
	newModel, _ := tea.Model(m).Update(key("e"))
	if got := newModel.(rootModel).exitAction; got != "" {
		t.Errorf("'e' with no inventoryPath should be ignored, exitAction = %q", got)
	}
}

func TestNKeyOnHostsScreenSetsAddHostAction(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	newModel, cmd := tea.Model(m).Update(key("n"))
	rm := newModel.(rootModel)
	if rm.exitAction != "add-host" {
		t.Errorf("'n' on Hosts: exitAction = %q, want add-host", rm.exitAction)
	}
	if cmd == nil {
		t.Error("'n' on Hosts should return tea.Quit")
	}
}

func TestNKeyWorksFromAnyScreen(t *testing.T) {
	// 'n' is a global shortcut now — adding a host shouldn't require
	// hunting for the Hosts tab first. On the Dashboard it should still
	// trigger the add-host flow.
	m := modelWithInventory(fixtureInventory()) // current == Dashboard
	newModel, cmd := tea.Model(m).Update(key("n"))
	rm := newModel.(rootModel)
	if rm.exitAction != "add-host" {
		t.Errorf("'n' on Dashboard: exitAction = %q, want add-host", rm.exitAction)
	}
	if cmd == nil {
		t.Error("'n' on Dashboard should return tea.Quit")
	}
}

func TestNKeyIgnoredWithEmptyInventory(t *testing.T) {
	// With no hosts to append to, the empty-state Dashboard points at 'i'
	// (init) instead, so 'n' is a no-op.
	m := rootModel{version: "test", current: screenDashboard, inv: &inventory.Inventory{}}
	newModel, _ := tea.Model(m).Update(key("n"))
	if got := newModel.(rootModel).exitAction; got != "" {
		t.Errorf("'n' with empty inventory should be ignored, exitAction = %q", got)
	}
}

func TestDashboardFooterAdvertisesAddHost(t *testing.T) {
	// The Dashboard footer must surface how to add a host so the user
	// doesn't have to discover the Hosts tab first.
	m := modelWithInventory(fixtureInventory())
	m.dashboard.rows = []upspoll.Row{{}} // non-empty so host hints show
	if bar := m.renderStatusBar(); !strings.Contains(bar, "n add") {
		t.Errorf("Dashboard footer should advertise 'n add host', got %q", bar)
	}
}

func TestEKeyOnHostsScreenEditsSelectedHost(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.inventoryPath = "homelab-nut.yaml"
	m.current = screenHosts
	m.selectedHost = 1
	newModel, cmd := tea.Model(m).Update(key("e"))
	rm := newModel.(rootModel)
	if rm.exitAction != "edit-host" {
		t.Errorf("'e' on Hosts: exitAction = %q, want edit-host", rm.exitAction)
	}
	if rm.exitHostIdx != 1 {
		t.Errorf("'e' on Hosts: exitHostIdx = %d, want 1", rm.exitHostIdx)
	}
	if cmd == nil {
		t.Error("'e' on Hosts should return tea.Quit")
	}
	if got := ExitHostIndex(rm); got != 1 {
		t.Errorf("ExitHostIndex() = %d, want 1", got)
	}
}

func TestEKeyOnDashboardStillOpensEditor(t *testing.T) {
	m := modelWithInventory(fixtureInventory()) // current == Dashboard
	m.inventoryPath = "homelab-nut.yaml"
	newModel, _ := tea.Model(m).Update(key("e"))
	if got := newModel.(rootModel).exitAction; got != "edit" {
		t.Errorf("'e' on Dashboard: exitAction = %q, want edit", got)
	}
}

func TestDKeyOnHostsScreenSetsDeleteHostAction(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	m.selectedHost = 1
	newModel, cmd := tea.Model(m).Update(key("d"))
	rm := newModel.(rootModel)
	if rm.exitAction != "delete-host" {
		t.Errorf("'d' on Hosts: exitAction = %q, want delete-host", rm.exitAction)
	}
	if rm.exitHostIdx != 1 {
		t.Errorf("'d' on Hosts: exitHostIdx = %d, want 1", rm.exitHostIdx)
	}
	if cmd == nil {
		t.Error("'d' on Hosts should return tea.Quit")
	}
}

func TestDKeyIgnoredOffHostsScreen(t *testing.T) {
	m := modelWithInventory(fixtureInventory()) // current == Dashboard
	newModel, _ := tea.Model(m).Update(key("d"))
	if got := newModel.(rootModel).exitAction; got != "" {
		t.Errorf("'d' off the Hosts screen should be ignored, exitAction = %q", got)
	}
}

func TestExitHostIndexHelper_NilSafe(t *testing.T) {
	var notMe tea.Model
	if got := ExitHostIndex(notMe); got != 0 {
		t.Errorf("ExitHostIndex(nil) = %d, want 0", got)
	}
}

func TestSKeyOnNUTServerHostSetsDetectAction(t *testing.T) {
	m := modelWithInventory(fixtureInventory()) // host[0]=pi has nut-server
	m.current = screenHosts
	m.selectedHost = 0
	newModel, cmd := tea.Model(m).Update(key("s"))
	rm := newModel.(rootModel)
	if rm.exitAction != "detect-host" {
		t.Errorf("'s' on nut-server host: exitAction = %q, want detect-host", rm.exitAction)
	}
	if rm.exitHostIdx != 0 {
		t.Errorf("'s': exitHostIdx = %d, want 0", rm.exitHostIdx)
	}
	if cmd == nil {
		t.Error("'s' on nut-server host should return tea.Quit")
	}
}

func TestSKeyIgnoredOnNonNUTServerHost(t *testing.T) {
	m := modelWithInventory(fixtureInventory()) // host[1]=ws has no nut-server
	m.current = screenHosts
	m.selectedHost = 1
	newModel, _ := tea.Model(m).Update(key("s"))
	if got := newModel.(rootModel).exitAction; got != "" {
		t.Errorf("'s' on non-nut-server host should be ignored, exitAction = %q", got)
	}
}

func TestSKeyIgnoredOffHostScreens(t *testing.T) {
	m := modelWithInventory(fixtureInventory()) // current == Dashboard
	newModel, _ := tea.Model(m).Update(key("s"))
	if got := newModel.(rootModel).exitAction; got != "" {
		t.Errorf("'s' off the Hosts/detail screens should be ignored, exitAction = %q", got)
	}
}

func TestExitActionHelper_NilSafe(t *testing.T) {
	// ExitAction on a non-rootModel returns "" without panic.
	var notMe tea.Model
	if got := ExitAction(notMe); got != "" {
		t.Errorf("ExitAction(nil) = %q, want \"\"", got)
	}
}

func TestQuitKeysReturnTeaQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m := tea.Model(modelWithInventory(fixtureInventory()))
		_, cmd := m.Update(key(k))
		if cmd == nil {
			t.Errorf("key %q returned nil cmd, want tea.Quit", k)
			continue
		}
		// tea.Quit is a function returning tea.QuitMsg. Easier to check
		// via the result.
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("key %q cmd did not produce QuitMsg", k)
		}
	}
}

func TestEmptyInventoryShowsInitHint(t *testing.T) {
	m := rootModel{
		version: "test",
		current: screenDashboard,
		inv:     nil,
	}
	out := m.View()
	if !strings.Contains(out, "homelab-nut init") {
		t.Errorf("empty inventory view missing init hint:\n%s", out)
	}
}

func TestActiveTabHighlightsHostsWhenDrilledIn(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHost
	// Tab bar should still show Hosts as active when current is Host.
	bar := m.renderTabBar()
	// Tab labels appear with index + name; the active style differs.
	if !strings.Contains(bar, "2 Hosts") {
		t.Errorf("tab bar missing Hosts tab:\n%s", bar)
	}
}
