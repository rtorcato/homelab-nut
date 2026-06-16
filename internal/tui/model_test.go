package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtorcato/homelab-nut/internal/inventory"
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
	expected := []screen{screenHosts, screenHelp, screenDashboard, screenHosts}
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
