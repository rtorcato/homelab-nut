package tui

import (
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/upspoll"
)

// piServerRow is a healthy nut-server poll result for the fixture's "pi"
// host, so the State column has live data to render.
func piServerRow(charge float64) []upspoll.Row {
	return []upspoll.Row{
		{Host: "pi", Address: "192.0.2.10", UPS: "myups", Status: "OL", BatteryCharge: &charge},
	}
}

func TestHostsTabWideLayoutShowsRolesAndState(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	m.width = 120
	m.dashboard.rows = piServerRow(100)

	out := m.View()
	for _, want := range []string{
		"NAME", "ADDRESS", "USER", "ROLES", "STATE", // table header
		"pi", "192.0.2.10",
		"nut-server (myups)", "OL · 100%",
		"shutdown-daemon (50%, 30s, 1 tgt)", "configured",
		"ws", "shutdown-target (~/shutdown.sh)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("wide Hosts tab missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestHostsTabNarrowLayoutReflowsWithoutTruncation(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	m.width = 70
	m.dashboard.rows = piServerRow(42)

	out := m.View()
	// No table header in the narrow layout, but every role summary and the
	// state must still render in full.
	for _, want := range []string{
		"pi", "192.0.2.10",
		"nut-server (myups)", "OL · 42%",
		"shutdown-daemon (50%, 30s, 1 tgt)",
		"shutdown-target (~/shutdown.sh)", "configured",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("narrow Hosts tab missing %q\n--- view ---\n%s", want, out)
		}
	}
	if strings.Contains(out, "ADDRESS  ") {
		t.Errorf("narrow layout should not render the wide table header:\n%s", out)
	}
}

func TestHostsTabRendersErrorStateForUnreachableServer(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	m.width = 120
	m.dashboard.rows = []upspoll.Row{{Host: "pi", Address: "192.0.2.10", Error: "timeout"}}

	out := m.View()
	if !strings.Contains(out, "ERR") {
		t.Errorf("expected ERR token for unreachable nut-server host:\n%s", out)
	}
}

func TestHostsTabShowsPollingBeforeFirstResult(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	m.width = 120
	// No dashboard rows yet — the very first tick hasn't returned.
	out := m.View()
	if !strings.Contains(out, "polling…") {
		t.Errorf("expected polling placeholder before first poll result:\n%s", out)
	}
}

func TestHostsTabSelectionCursorFollowsSelectedHost(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHosts
	m.width = 120
	m.selectedHost = 1
	m.dashboard.rows = piServerRow(100)

	out := m.View()
	if !strings.Contains(out, "▸ ") {
		t.Errorf("expected selection cursor ▸ on the Hosts tab:\n%s", out)
	}
	// The cursor must sit on the selected ("ws") row, not the first one.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "▸") && !strings.Contains(line, "ws") {
			t.Errorf("cursor on wrong row: %q", line)
		}
	}
}

func TestHostsTabUsesConfiguredForNonServerRoles(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: []inventory.Host{{
			Name: "unas", Address: "10.0.10.125", User: "root",
			Roles:    []inventory.Role{inventory.RoleShutdownTarget},
			Shutdown: &inventory.Shutdown{Command: "poweroff"},
		}},
	}
	m := modelWithInventory(inv)
	m.current = screenHosts
	m.width = 120

	out := m.View()
	for _, want := range []string{"unas", "shutdown-target (poweroff)", "configured"} {
		if !strings.Contains(out, want) {
			t.Errorf("Hosts tab missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestTruncateCommand(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"poweroff", 24, "poweroff"},
		{"~/shutdown.sh", 24, "~/shutdown.sh"},
		{"~/very/long/path/to/the/shutdown.sh", 24, "~/very/long/path/to/the…"},
	}
	for _, tc := range cases {
		got := truncateCommand(tc.in, tc.n)
		if got != tc.want {
			t.Errorf("truncateCommand(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
		// Result must never exceed the requested cell budget.
		if w := len([]rune(got)); w > tc.n {
			t.Errorf("truncateCommand(%q, %d) width %d exceeds budget", tc.in, tc.n, w)
		}
	}
}
