package inventory

import (
	"strings"
	"testing"
)

func TestEffectiveShutdownDaemon_Precedence(t *testing.T) {
	global := &ShutdownDaemon{Threshold: 50, PollInterval: 30}
	override := &ShutdownDaemon{Threshold: 40, PollInterval: 20}

	hostWith := &Host{Name: "a", Roles: []Role{RoleShutdownDaemon}, ShutdownDaemon: override}
	hostWithout := &Host{Name: "b", Roles: []Role{RoleShutdownDaemon}}

	cases := []struct {
		name string
		inv  *Inventory
		host *Host
		want *ShutdownDaemon
	}{
		{"host override wins over global", &Inventory{ShutdownDaemon: global}, hostWith, override},
		{"falls back to global", &Inventory{ShutdownDaemon: global}, hostWithout, global},
		{"override with no global", &Inventory{}, hostWith, override},
		{"nil when neither set", &Inventory{}, hostWithout, nil},
		{"nil host falls back to global", &Inventory{ShutdownDaemon: global}, nil, global},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.inv.EffectiveShutdownDaemon(tc.host); got != tc.want {
				t.Errorf("EffectiveShutdownDaemon() = %v, want %v", got, tc.want)
			}
		})
	}
}

// A legacy inventory carrying only the root-level shutdown_daemon block must
// keep driving a daemon host — the back-compat guarantee (no hand-edit needed).
func TestEffectiveShutdownDaemon_LegacyGlobalOnly(t *testing.T) {
	yml := `
hosts:
  - name: pi
    address: 192.0.2.10
    user: pi
    roles: [nut-server, shutdown-daemon]
    ups: { name: myups, driver: usbhid-ups }
  - name: ws
    address: 192.0.2.20
    user: admin
    roles: [shutdown-target]
    shutdown: { command: poweroff }
shutdown_daemon:
  threshold: 42
  poll_interval: 17
`
	inv, err := LoadReader(strings.NewReader(yml))
	if err != nil {
		t.Fatalf("legacy inventory should load: %v", err)
	}
	d := inv.EffectiveShutdownDaemon(inv.HostByName("pi"))
	if d == nil || d.Threshold != 42 || d.PollInterval != 17 {
		t.Fatalf("legacy global block should drive the daemon host, got %+v", d)
	}
}
