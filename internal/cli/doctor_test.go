package cli

import (
	"testing"

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
