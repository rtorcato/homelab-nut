package roles

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestNutServer_Name(t *testing.T) {
	if got := (nutServer{}).Name(); got != "nut-server" {
		t.Errorf("Name() = %q, want nut-server", got)
	}
}

func TestNutServer_Applies(t *testing.T) {
	r := nutServer{}
	cases := []struct {
		name  string
		host  *inventory.Host
		apply bool
	}{
		{"nil host", nil, false},
		{"no roles", &inventory.Host{Name: "h"}, false},
		{"wrong role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTClient}}, false},
		{"correct role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer}}, true},
		{"role + others", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleExporter}}, true},
	}
	for _, tc := range cases {
		if got := r.Applies(tc.host); got != tc.apply {
			t.Errorf("Applies(%s) = %v, want %v", tc.name, got, tc.apply)
		}
	}
}

func TestNutServer_PlanRejectsMissingUPS(t *testing.T) {
	r := nutServer{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		// no UPS block
	}
	_, err := r.Plan(context.TODO(), nil, h)
	if err == nil {
		t.Fatal("Plan should reject host without ups.name/ups.driver")
	}
	if !strings.Contains(err.Error(), "ups.name") || !strings.Contains(err.Error(), "ups.driver") {
		t.Errorf("error should mention both required fields, got: %v", err)
	}
}

func TestNutServer_PlanReturnsActions(t *testing.T) {
	r := nutServer{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if d.NoOp() {
		t.Error("Plan with nil conn (StateUnknown) should propose actions")
	}
	if d.Role != "nut-server" || d.Host != h || d.Target != StateOK {
		t.Errorf("Diff fields wrong: %+v", d)
	}
	// One of the actions should mention the UPS name + driver so the
	// preview is informative.
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "myups") || !strings.Contains(joined, "usbhid-ups") {
		t.Errorf("actions should mention ups name + driver, got:\n%s", joined)
	}
}

func TestNutServer_RegisteredOnInit(t *testing.T) {
	r, ok := ByName("nut-server")
	if !ok {
		t.Fatal("nut-server not registered in roles.All()")
	}
	if r.Name() != "nut-server" {
		t.Errorf("ByName(nut-server).Name() = %q", r.Name())
	}
}

func TestNutServer_EmbeddedScriptReadable(t *testing.T) {
	// The whole point of embed is that the script lives in the binary.
	// Verify the bytes are non-empty and look like the original.
	b, err := readScript("setup-server.sh")
	if err != nil {
		t.Fatalf("readScript: %v", err)
	}
	if len(b) < 100 {
		t.Errorf("setup-server.sh suspiciously short: %d bytes", len(b))
	}
	if !strings.HasPrefix(string(b), "#!/bin/bash") {
		t.Errorf("setup-server.sh missing shebang: %q", string(b[:20]))
	}
}

func TestNutServer_DetectCmdShape(t *testing.T) {
	// The detect script is run on the remote — verify the structure
	// without actually executing it. Catches regressions where the
	// command no longer covers all three states.
	cmd := detectCmd("server-rack-ups")
	for _, marker := range []string{"OK", "PARTIAL", "MISSING", "upsd", "nut-server", "systemctl", "upsc 'server-rack-ups@localhost'"} {
		if !strings.Contains(cmd, marker) {
			t.Errorf("detectCmd missing %q:\n%s", marker, cmd)
		}
	}
}

// TestNutServer_DetectCmdRuns executes detectCmd against stub upsd,
// systemctl and upsc binaries. A running nut-server unit whose upsd
// cannot serve the inventory's UPS is the #132 failure: it must read
// PARTIAL so plan shows drift, never OK.
func TestNutServer_DetectCmdRuns(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	cases := []struct {
		name               string
		upsd, active, upsc bool
		want               string
	}{
		{"not installed", false, false, false, "MISSING"},
		{"unit inactive (MODE=netclient)", true, false, false, "PARTIAL"},
		{"unit active, upsc fails", true, true, false, "PARTIAL"},
		{"unit active, upsc reads", true, true, true, "OK"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stub := func(name string, ok bool) {
				code := "1"
				if ok {
					code = "0"
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit "+code+"\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.upsd {
				stub("upsd", true)
			}
			stub("systemctl", tc.active)
			stub("upsc", tc.upsc)
			c := exec.Command(bash, "-c", detectCmd("server-rack-ups"))
			c.Env = []string{"PATH=" + dir}
			out, err := c.Output()
			if err != nil {
				t.Fatalf("detectCmd: %v", err)
			}
			if got := strings.TrimSpace(string(out)); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
