package roles

import (
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
	_, err := r.Plan(nil, nil, h)
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
	d, err := r.Plan(nil, nil, h)
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
	for _, marker := range []string{"OK", "PARTIAL", "MISSING", "upsd", "nut-server", "systemctl"} {
		if !strings.Contains(detectCmd, marker) {
			t.Errorf("detectCmd missing %q:\n%s", marker, detectCmd)
		}
	}
}
