package roles

import (
	"context"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestShutdownTarget_Name(t *testing.T) {
	if got := (shutdownTarget{}).Name(); got != "shutdown-target" {
		t.Errorf("Name() = %q, want shutdown-target", got)
	}
}

func TestShutdownTarget_Applies(t *testing.T) {
	r := shutdownTarget{}
	cases := []struct {
		name  string
		host  *inventory.Host
		apply bool
	}{
		{"nil host", nil, false},
		{"no roles", &inventory.Host{Name: "h"}, false},
		{"wrong role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer}}, false},
		{"target role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleShutdownTarget}}, true},
		{"target + client", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleShutdownTarget, inventory.RoleNUTClient}}, true},
	}
	for _, tc := range cases {
		if got := r.Applies(tc.host); got != tc.apply {
			t.Errorf("Applies(%s) = %v, want %v", tc.name, got, tc.apply)
		}
	}
}

func TestIsScriptCommand(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},                   // empty defaults to script
		{"~/shutdown.sh", true},      // tilde
		{"./shutdown.sh", true},      // relative path
		{"/usr/local/bin/foo.sh", true}, // absolute
		{"/etc/foo.sh", true},
		{"my-script.sh", true},       // ends in .sh
		{"poweroff", false},          // inline
		{"shutdown -h now", false},   // inline command
		{"reboot", false},
	}
	for _, tc := range cases {
		if got := isScriptCommand(tc.in); got != tc.want {
			t.Errorf("isScriptCommand(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestShutdownTarget_ResolvedMode(t *testing.T) {
	r := shutdownTarget{}
	cases := []struct {
		name string
		host *inventory.Host
		want string
	}{
		{"nil host", nil, "script"},
		{"no shutdown block", &inventory.Host{Name: "h"}, "script"},
		{"script command", &inventory.Host{Name: "h", Shutdown: &inventory.Shutdown{Command: "~/shutdown.sh"}}, "script"},
		{"inline poweroff", &inventory.Host{Name: "h", Shutdown: &inventory.Shutdown{Command: "poweroff"}}, "inline"},
		{"empty command", &inventory.Host{Name: "h", Shutdown: &inventory.Shutdown{Command: ""}}, "script"},
	}
	for _, tc := range cases {
		if got := r.resolvedMode(tc.host); got != tc.want {
			t.Errorf("%s: resolvedMode = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestShutdownTarget_PlanScriptMode(t *testing.T) {
	r := shutdownTarget{}
	h := &inventory.Host{
		Name: "ws", User: "admin",
		Roles:    []inventory.Role{inventory.RoleShutdownTarget},
		Shutdown: &inventory.Shutdown{Command: "~/shutdown.sh"},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "/home/admin/shutdown.sh") {
		t.Errorf("script Plan should mention deploying ~/shutdown.sh, got:\n%s", joined)
	}
	if !strings.Contains(joined, "ups-shutdown") {
		t.Errorf("script Plan should mention configuring sudoers, got:\n%s", joined)
	}
}

func TestShutdownTarget_PlanInlineMode(t *testing.T) {
	r := shutdownTarget{}
	h := &inventory.Host{
		Name: "dream-machine", User: "admin",
		Roles:    []inventory.Role{inventory.RoleShutdownTarget},
		Shutdown: &inventory.Shutdown{Command: "poweroff"},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "inline mode") {
		t.Errorf("inline Plan should mention 'inline mode', got:\n%s", joined)
	}
	if !strings.Contains(joined, "poweroff") {
		t.Errorf("inline Plan should mention the inline command, got:\n%s", joined)
	}
	if strings.Contains(joined, "shutdown.sh") {
		t.Errorf("inline Plan should NOT mention shutdown.sh deployment, got:\n%s", joined)
	}
}

func TestShutdownTarget_ApplyValidatesUser(t *testing.T) {
	// Apply rejects an empty user since the install + sudoers commands
	// interpolate it directly. nil conn fails first; we still want the
	// check to exist.
	r := shutdownTarget{}
	h := &inventory.Host{
		Name: "ws", // no User set
		Roles:    []inventory.Role{inventory.RoleShutdownTarget},
		Shutdown: &inventory.Shutdown{Command: "~/shutdown.sh"},
	}
	err := r.Apply(context.TODO(), nil, h, nil)
	if err == nil {
		t.Fatal("Apply with nil conn should error")
	}
	if !strings.Contains(err.Error(), "nil connection") {
		t.Errorf("expected nil-connection error, got: %v", err)
	}
}

func TestShutdownTarget_RegisteredOnInit(t *testing.T) {
	r, ok := ByName("shutdown-target")
	if !ok {
		t.Fatal("shutdown-target not registered in roles.All()")
	}
	if r.Name() != "shutdown-target" {
		t.Errorf("ByName(shutdown-target).Name() = %q", r.Name())
	}
}

func TestShutdownTarget_EmbeddedShutdownReadable(t *testing.T) {
	b, err := readScript("shutdown.sh")
	if err != nil {
		t.Fatalf("readScript: %v", err)
	}
	if len(b) < 100 {
		t.Errorf("shutdown.sh suspiciously short: %d bytes", len(b))
	}
	if !strings.HasPrefix(string(b), "#!/") {
		t.Errorf("shutdown.sh missing shebang: %q", string(b[:20]))
	}
}

func TestShutdownTarget_DetectCmdShape(t *testing.T) {
	for _, marker := range []string{"OK", "PARTIAL", "MISSING", "ups-shutdown", "shutdown.sh"} {
		if !strings.Contains(shutdownTargetDetectCmd, marker) {
			t.Errorf("shutdownTargetDetectCmd missing %q", marker)
		}
	}
	// Inline variant only needs sudoers presence.
	for _, marker := range []string{"OK", "MISSING", "ups-shutdown"} {
		if !strings.Contains(shutdownTargetDetectInlineCmd, marker) {
			t.Errorf("shutdownTargetDetectInlineCmd missing %q", marker)
		}
	}
}
