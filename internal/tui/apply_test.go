package tui

import (
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestApplyIdleShowsPlanPreview(t *testing.T) {
	m := modelWithInventory(fixtureInventory())
	m.current = screenApply
	m.width = 100 // apply.status defaults to applyIdle

	out := m.View()
	for _, want := range []string{
		"Apply — set up NUT",
		"over SSH",
		"Idempotent",
		// host identity
		"pi", "192.0.2.10",
		"ws", "192.0.2.20",
		// role-specific, inventory-aware descriptions
		`configure UPS "myups"`,
		"threshold 50%, poll 30s",
		"register graceful-shutdown command (~/shutdown.sh)",
		"press 'a' to apply",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("apply preview missing %q\n--- view ---\n%s", want, out)
		}
	}
}

func TestApplyPlanRespectsExecutionOrder(t *testing.T) {
	// fixtureInventory's pi has roles [nut-server, shutdown-daemon]; Apply
	// runs nut-server first, so its line must precede shutdown-daemon's.
	m := modelWithInventory(fixtureInventory())
	m.current = screenApply
	m.width = 100

	out := m.View()
	srv := strings.Index(out, "nut-server")
	daemon := strings.Index(out, "shutdown-daemon")
	if srv < 0 || daemon < 0 {
		t.Fatalf("expected both roles in preview, got nut-server=%d shutdown-daemon=%d", srv, daemon)
	}
	if srv > daemon {
		t.Errorf("nut-server (%d) should appear before shutdown-daemon (%d)", srv, daemon)
	}
}

func TestRoleActionLineVariants(t *testing.T) {
	inv := &inventory.Inventory{
		ShutdownDaemon: &inventory.ShutdownDaemon{Threshold: 20, PollInterval: 15},
	}
	cases := []struct {
		role inventory.Role
		host inventory.Host
		want string
	}{
		{inventory.RoleNUTClient, inventory.Host{}, "point it at the NUT server"},
		{inventory.RoleExporter, inventory.Host{}, "Prometheus NUT exporter"},
		{inventory.RoleShutdownDaemon, inventory.Host{}, "threshold 20%, poll 15s"},
		{inventory.RoleShutdownTarget, inventory.Host{Shutdown: &inventory.Shutdown{Command: "poweroff"}}, "(poweroff)"},
	}
	for _, tc := range cases {
		got := roleActionLine(tc.role, &tc.host, inv)
		if !strings.Contains(got, tc.want) {
			t.Errorf("roleActionLine(%s) = %q, want substring %q", tc.role, got, tc.want)
		}
	}
}
