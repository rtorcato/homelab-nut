package roles

import (
	"context"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestShutdownDaemon_Name(t *testing.T) {
	if got := (shutdownDaemon{}).Name(); got != "shutdown-daemon" {
		t.Errorf("Name() = %q, want shutdown-daemon", got)
	}
}

func TestShutdownDaemon_Applies(t *testing.T) {
	r := shutdownDaemon{}
	cases := []struct {
		name  string
		host  *inventory.Host
		apply bool
	}{
		{"nil host", nil, false},
		{"no roles", &inventory.Host{Name: "h"}, false},
		{"wrong role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer}}, false},
		{"daemon role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleShutdownDaemon}}, true},
		{"daemon + server (typical)", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleShutdownDaemon}}, true},
	}
	for _, tc := range cases {
		if got := r.Applies(tc.host); got != tc.apply {
			t.Errorf("Applies(%s) = %v, want %v", tc.name, got, tc.apply)
		}
	}
}

func TestRemoteNodesFromInventory(t *testing.T) {
	cases := []struct {
		name string
		inv  *inventory.Inventory
		want string
	}{
		{"nil", nil, ""},
		{"empty", &inventory.Inventory{}, ""},
		{
			"two targets",
			&inventory.Inventory{Hosts: []inventory.Host{
				{Name: "a", User: "alice", Address: "192.0.2.1", Roles: []inventory.Role{inventory.RoleShutdownTarget}},
				{Name: "b", User: "bob", Address: "192.0.2.2", Roles: []inventory.Role{inventory.RoleShutdownTarget}},
			}},
			"alice@192.0.2.1 bob@192.0.2.2",
		},
		{
			"mixed roles — skips non-targets",
			&inventory.Inventory{Hosts: []inventory.Host{
				{Name: "pi", User: "pi", Address: "192.0.2.10", Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleShutdownDaemon}},
				{Name: "ws", User: "admin", Address: "192.0.2.20", Roles: []inventory.Role{inventory.RoleShutdownTarget}},
			}},
			"admin@192.0.2.20",
		},
	}
	for _, tc := range cases {
		if got := remoteNodesFromInventory(tc.inv); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeNodeHost(t *testing.T) {
	cases := map[string]string{
		"10.0.10.125":   "10_0_10_125",
		"dream-machine": "dream_machine",
		"nas.local":     "nas_local",
		"unas-pro.lan":  "unas_pro_lan",
		"192.0.2.1":     "192_0_2_1",
		"plainhost":     "plainhost",
	}
	for in, want := range cases {
		if got := sanitizeNodeHost(in); got != want {
			t.Errorf("sanitizeNodeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoteCmdsFromInventory(t *testing.T) {
	cases := []struct {
		name string
		inv  *inventory.Inventory
		want string
	}{
		{"nil", nil, ""},
		{"empty", &inventory.Inventory{}, ""},
		{
			"target without command — no override",
			&inventory.Inventory{Hosts: []inventory.Host{
				{Name: "ws", User: "admin", Address: "192.0.2.20", Roles: []inventory.Role{inventory.RoleShutdownTarget}},
			}},
			"",
		},
		{
			"inline command target (the UniFi case)",
			&inventory.Inventory{Hosts: []inventory.Host{
				{Name: "nas", User: "root", Address: "10.0.10.125", Roles: []inventory.Role{inventory.RoleShutdownTarget}, Shutdown: &inventory.Shutdown{Command: "poweroff"}},
			}},
			"CMD_10_0_10_125=poweroff",
		},
		{
			"multiple targets, inventory order, non-targets skipped",
			&inventory.Inventory{Hosts: []inventory.Host{
				{Name: "pi", User: "pi", Address: "192.0.2.10", Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleShutdownDaemon}},
				{Name: "ws", User: "admin", Address: "192.0.2.20", Roles: []inventory.Role{inventory.RoleShutdownTarget}, Shutdown: &inventory.Shutdown{Command: "~/shutdown.sh"}},
				{Name: "nas", User: "root", Address: "10.0.10.125", Roles: []inventory.Role{inventory.RoleShutdownTarget}, Shutdown: &inventory.Shutdown{Command: "poweroff"}},
			}},
			"CMD_192_0_2_20=~/shutdown.sh\nCMD_10_0_10_125=poweroff",
		},
	}
	for _, tc := range cases {
		if got := remoteCmdsFromInventory(tc.inv); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestUPSRefFromInventory(t *testing.T) {
	cases := []struct {
		name string
		host *inventory.Host
		want string
	}{
		{"nil", nil, "myups@localhost"},
		{"no server role", &inventory.Host{Name: "h"}, "myups@localhost"},
		{
			"server + ups",
			&inventory.Host{
				Name:  "pi",
				Roles: []inventory.Role{inventory.RoleNUTServer},
				UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
			},
			"myups@localhost",
		},
		{
			"server + named ups",
			&inventory.Host{
				Name:  "pi",
				Roles: []inventory.Role{inventory.RoleNUTServer},
				UPS:   &inventory.UPS{Name: "rack-ups"},
			},
			"rack-ups@localhost",
		},
		{
			"server role but no ups block — falls back",
			&inventory.Host{Name: "pi", Roles: []inventory.Role{inventory.RoleNUTServer}},
			"myups@localhost",
		},
	}
	for _, tc := range cases {
		if got := upsRefFromInventory(tc.host); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestShutdownDaemon_PlanRejectsInventoryWithoutTargets(t *testing.T) {
	r := shutdownDaemon{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleShutdownDaemon},
	}
	// Inventory has the daemon host but no shutdown-target.
	inv := &inventory.Inventory{Hosts: []inventory.Host{*h}}
	ctx := WithInventory(context.TODO(), inv)
	_, err := r.Plan(ctx, nil, h)
	if err == nil {
		t.Fatal("Plan should reject inventory with no shutdown-target hosts")
	}
	if !strings.Contains(err.Error(), "shutdown-target") {
		t.Errorf("error should mention shutdown-target, got: %v", err)
	}
}

func TestShutdownDaemon_PlanProducesActions(t *testing.T) {
	r := shutdownDaemon{}
	daemonHost := inventory.Host{
		Name: "pi", Address: "192.0.2.10", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleShutdownDaemon},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	target := inventory.Host{
		Name: "ws", Address: "192.0.2.20", User: "admin",
		Roles: []inventory.Role{inventory.RoleShutdownTarget},
	}
	inv := &inventory.Inventory{
		Hosts:          []inventory.Host{daemonHost, target},
		ShutdownDaemon: &inventory.ShutdownDaemon{Threshold: 40, PollInterval: 20},
	}
	ctx := WithInventory(context.TODO(), inv)

	d, err := r.Plan(ctx, nil, &daemonHost)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	for _, want := range []string{"myups@localhost", "40%", "20s", "admin@192.0.2.20"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Plan actions missing %q\n%s", want, joined)
		}
	}
}

func TestShutdownDaemon_PlanSurfacesPerTargetCommands(t *testing.T) {
	r := shutdownDaemon{}
	daemonHost := inventory.Host{
		Name: "pi", Address: "192.0.2.10", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleShutdownDaemon},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	nas := inventory.Host{
		Name: "nas", Address: "10.0.10.125", User: "root",
		Roles:    []inventory.Role{inventory.RoleShutdownTarget},
		Shutdown: &inventory.Shutdown{Command: "poweroff"},
	}
	inv := &inventory.Inventory{
		Hosts:          []inventory.Host{daemonHost, nas},
		ShutdownDaemon: &inventory.ShutdownDaemon{Threshold: 50, PollInterval: 30},
	}
	ctx := WithInventory(context.TODO(), inv)

	d, err := r.Plan(ctx, nil, &daemonHost)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "CMD_10_0_10_125=poweroff") {
		t.Errorf("Plan should surface the per-target command override, got:\n%s", joined)
	}
}

func TestShutdownDaemon_PlanFallbackWithoutInventory(t *testing.T) {
	// No inventory in ctx — Plan should still produce a generic preview
	// (defers cross-host resolution to Apply where it'll error cleanly).
	r := shutdownDaemon{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleShutdownDaemon},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "resolved at apply time") {
		t.Errorf("fallback Plan should mention 'resolved at apply time', got:\n%s", joined)
	}
}

func TestShutdownDaemon_ApplyNilConnFailsFast(t *testing.T) {
	r := shutdownDaemon{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleShutdownDaemon},
	}
	err := r.Apply(context.TODO(), nil, h, nil)
	if err == nil || !strings.Contains(err.Error(), "nil connection") {
		t.Errorf("expected nil-connection error, got: %v", err)
	}
}

func TestShutdownDaemon_RegisteredOnInit(t *testing.T) {
	r, ok := ByName("shutdown-daemon")
	if !ok {
		t.Fatal("shutdown-daemon not registered in roles.All()")
	}
	if r.Name() != "shutdown-daemon" {
		t.Errorf("ByName(shutdown-daemon).Name() = %q", r.Name())
	}
}

func TestShutdownDaemon_EmbeddedScriptsReadable(t *testing.T) {
	for _, name := range []string{"setup-shutdown-daemon.sh", "battery-shutdown.sh"} {
		b, err := readScript(name)
		if err != nil {
			t.Errorf("readScript(%s): %v", name, err)
			continue
		}
		if len(b) < 100 {
			t.Errorf("%s suspiciously short: %d bytes", name, len(b))
		}
		if !strings.HasPrefix(string(b), "#!/") {
			t.Errorf("%s missing shebang: %q", name, string(b[:20]))
		}
	}
}
