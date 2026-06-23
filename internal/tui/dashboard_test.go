package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/upspoll"
)

// fakeRows returns a happy-path row alongside an error row so the same
// fixture exercises both rendering branches.
func fakeRows() []upspoll.Row {
	charge := 85.0
	load := 12.0
	runtime := 7400
	return []upspoll.Row{
		{
			Host: "pi", Address: "10.0.10.158", UPS: "myups", Status: "OL",
			BatteryCharge: &charge, BatteryRuntime: &runtime, Load: &load,
		},
		{Host: "office", Address: "10.0.0.5", Error: "timeout"},
	}
}

func TestDashboardRendersHappyCard(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.dashboard.rows = fakeRows()
	m.dashboard.updated = time.Date(2026, 6, 19, 12, 30, 45, 0, time.UTC)
	m.width = 120

	out := m.View()
	for _, want := range []string{
		"pi", "10.0.10.158", "myups", "OL",
		"Battery", "85%", "Load", "12%", "Runtime", "2h3m20s",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestDashboardRendersErrorCard(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.dashboard.rows = fakeRows()
	m.width = 80

	out := m.View()
	for _, want := range []string{"office", "10.0.0.5", "timeout", "ERR"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestDashboardEmptyStateWhenNoNUTServerHosts(t *testing.T) {
	// fixtureInventory's first host has nut-server; strip it so we can
	// verify the empty-state path.
	inv := fixtureInventory()
	inv.Hosts = inv.Hosts[1:]
	m := modelWithInventory(inv)
	m.width = 80
	out := m.View()
	if !strings.Contains(out, "No nut-server hosts") {
		t.Errorf("expected empty-state message, got:\n%s", out)
	}
}

func TestDashboardFooterShowsRefreshedTimestamp(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.dashboard.rows = fakeRows()
	m.dashboard.updated = time.Date(2026, 6, 19, 9, 5, 1, 0, time.UTC)
	m.width = 100

	out := m.View()
	if !strings.Contains(out, "refreshed 09:05:01") {
		t.Errorf("footer missing timestamp, got:\n%s", out)
	}
	if !strings.Contains(out, "r to refresh") {
		t.Errorf("footer missing refresh hint, got:\n%s", out)
	}
}

func TestDashboardCardsReflowOnWidth(t *testing.T) {
	rows := []upspoll.Row{
		{Host: "a", Address: "1.1.1.1", UPS: "u", Status: "OL"},
		{Host: "b", Address: "2.2.2.2", UPS: "u", Status: "OL"},
		{Host: "c", Address: "3.3.3.3", UPS: "u", Status: "OL"},
	}
	narrow := renderDashboardCards(rows, 40, -1)
	wide := renderDashboardCards(rows, 200, -1)

	// Narrow: each card on its own line — at least as many lines as
	// cards (cards are multi-line themselves so a generous lower bound
	// is fine).
	if got := strings.Count(narrow, "a"); got == 0 {
		t.Errorf("narrow layout dropped host 'a':\n%s", narrow)
	}
	// Wide: cards pack into fewer rows. Verify all three names are
	// present and that the wide rendering takes fewer total lines.
	if !strings.Contains(wide, "a") || !strings.Contains(wide, "c") {
		t.Errorf("wide layout missing hosts:\n%s", wide)
	}
	if narrowLines, wideLines := strings.Count(narrow, "\n"), strings.Count(wide, "\n"); wideLines >= narrowLines {
		t.Errorf("expected wide layout (lines=%d) to be more compact than narrow (lines=%d)",
			wideLines, narrowLines)
	}
}

func TestDashboardUpdatedMsgPopulatesRowsAndClearsInFlight(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.dashboard.inFlight = true
	rows := fakeRows()
	now := time.Now()

	newM, _ := tea.Model(m).Update(dashboardUpdatedMsg{rows: rows, at: now})
	got := newM.(rootModel).dashboard
	if got.inFlight {
		t.Error("inFlight should clear after dashboardUpdatedMsg")
	}
	if len(got.rows) != 2 {
		t.Errorf("rows = %d, want 2", len(got.rows))
	}
	if !got.updated.Equal(now) {
		t.Errorf("updated = %v, want %v", got.updated, now)
	}
}

func TestDashboardArrowsMoveSelectionAndSyncHost(t *testing.T) {
	// fixtureInventory: host[0]=pi (nut-server), host[1]=ws (no nut-server).
	// Dashboard rows are nut-server hosts only, so a second card needs a
	// second nut-server host. Build a 2-card row set whose names map back
	// to real inventory hosts.
	inv := fixtureInventory()
	inv.Hosts = append(inv.Hosts, inventory.Host{
		Name: "pi2", Address: "192.0.2.30", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		UPS:   &inventory.UPS{Name: "u2", Driver: "usbhid-ups"},
	})
	m := modelWithInventory(inv)
	m.current = screenDashboard
	m.dashboard.rows = []upspoll.Row{{Host: "pi"}, {Host: "pi2"}}

	// down -> select second card, selectedHost syncs to inventory index 2.
	nm, _ := tea.Model(m).Update(key("down"))
	rm := nm.(rootModel)
	if rm.dashSelected != 1 {
		t.Errorf("after down: dashSelected = %d, want 1", rm.dashSelected)
	}
	if rm.selectedHost != 2 {
		t.Errorf("after down: selectedHost = %d, want 2 (pi2)", rm.selectedHost)
	}

	// enter -> drill into host detail for the synced host.
	nm2, _ := tea.Model(rm).Update(key("enter"))
	if got := nm2.(rootModel).current; got != screenHost {
		t.Errorf("after enter: current = %v, want Host", got)
	}

	// up at top clamps at 0.
	rm.dashSelected = 0
	nm3, _ := tea.Model(rm).Update(key("up"))
	if got := nm3.(rootModel).dashSelected; got != 0 {
		t.Errorf("up at top: dashSelected = %d, want 0", got)
	}
}

func TestDashboardSelectionClampsWhenRowsShrink(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.dashSelected = 3
	newM, _ := tea.Model(m).Update(dashboardUpdatedMsg{rows: []upspoll.Row{{Host: "pi"}}, at: time.Now()})
	if got := newM.(rootModel).dashSelected; got != 0 {
		t.Errorf("dashSelected should clamp to 0 when rows shrink, got %d", got)
	}
}

func TestRKeyTriggersImmediatePollWhenIdle(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	newM, cmd := tea.Model(m).Update(key("r"))
	if !newM.(rootModel).dashboard.inFlight {
		t.Error("r key should set inFlight = true")
	}
	if cmd == nil {
		t.Error("r key should return a poll cmd")
	}
}

func TestRKeyIgnoredWhenInFlight(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.dashboard.inFlight = true
	_, cmd := tea.Model(m).Update(key("r"))
	if cmd != nil {
		t.Error("r key should be ignored when poll is already in flight")
	}
}

func TestGaugeLineFallbacks(t *testing.T) {
	if got := gaugeLine("X", nil); !strings.Contains(got, "----------") || !strings.Contains(got, "-") {
		t.Errorf("nil gauge = %q, want placeholder bar + dash value", got)
	}
	v := 73.0
	if got := gaugeLine("X", &v); !strings.Contains(got, "73%") {
		t.Errorf("gauge = %q, want 73%%", got)
	}
}

func TestHelpScreenShowsStatusLegend(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenHelp
	out := m.View()
	for _, want := range []string{
		"UPS status codes",
		"On Line",
		"On Battery",
		"Low Battery",
		"unreachable",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing legend entry %q\n--- view ---\n%s", want, out)
		}
	}
	// Make sure each legend row's badge actually renders (token from
	// the upspoll.Row passed in is in the output).
	for _, want := range []string{"OL", "OB", "LB", "ERR"} {
		if !strings.Contains(out, want) {
			t.Errorf("help legend missing badge token %q", want)
		}
	}
}

func TestStatusBadgeColorsBySeverity(t *testing.T) {
	// Severity classification is the load-bearing behavior — we don't
	// check ANSI codes, just that the badge text contains the right token.
	cases := map[string]upspoll.Row{
		"OL":    {Status: "OL"},
		"OB":    {Status: "OB"},
		"OB LB": {Status: "OB LB"},
	}
	for want, row := range cases {
		got := statusBadge(row)
		if !strings.Contains(got, want) {
			t.Errorf("badge for %q = %q, missing token %q", want, got, want)
		}
	}
	if got := statusBadge(upspoll.Row{Error: "boom"}); !strings.Contains(got, "ERR") {
		t.Errorf("error badge = %q, want ERR", got)
	}
}
