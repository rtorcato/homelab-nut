package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/doctor"
	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestDoctorTargets(t *testing.T) {
	inv := &inventory.Inventory{Hosts: []inventory.Host{
		{Name: "pi", Roles: []inventory.Role{inventory.RoleNUTServer}},
		{Name: "nas", Roles: []inventory.Role{inventory.RoleNUTClient, inventory.RoleShutdownTarget}},
		{Name: "unifi", Roles: []inventory.Role{inventory.RoleShutdownTarget}},
	}}
	got, err := doctorTargets(inv, "")
	if err != nil || len(got) != 2 || got[0].Name != "pi" || got[1].Name != "nas" {
		t.Fatalf("all: got %v, %v; want pi, nas", got, err)
	}
	if _, err := doctorTargets(inv, "ghost"); err == nil {
		t.Error("unknown host should error")
	}
	if _, err := doctorTargets(inv, "unifi"); err == nil {
		t.Error("shutdown-target-only host has nothing to check")
	}
}

// A hostile NUT server picks the UPS names doctor prints; escape sequences in
// them must not reach the terminal.
func TestPrintDoctorResultsStripsControlChars(t *testing.T) {
	var buf bytes.Buffer
	err := printDoctorResults(&buf, []doctor.Result{
		{Host: "pi", Checks: []doctor.Check{{Name: "monitor ups@pi", Detail: "has: \x1b]0;pwned\x07ok\x1b[2J\r\tx"}}},
		{Host: "nas", Error: "probe nas: exit 1 (stderr: \x1b[31mred)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "\x1b\x07\r") {
		t.Fatalf("control characters leaked: %q", out)
	}
	for _, want := range []string{"]0;pwnedok[2Jx", "[31mred"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}
