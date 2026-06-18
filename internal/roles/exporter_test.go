package roles

import (
	"context"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestExporter_Name(t *testing.T) {
	if got := (exporter{}).Name(); got != "exporter" {
		t.Errorf("Name() = %q, want exporter", got)
	}
}

func TestExporter_Applies(t *testing.T) {
	r := exporter{}
	cases := []struct {
		name  string
		host  *inventory.Host
		apply bool
	}{
		{"nil host", nil, false},
		{"no roles", &inventory.Host{Name: "h"}, false},
		{"wrong role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTClient}}, false},
		{"exporter only", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleExporter}}, true},
		{"co-located", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleExporter}}, true},
	}
	for _, tc := range cases {
		if got := r.Applies(tc.host); got != tc.apply {
			t.Errorf("Applies(%s) = %v, want %v", tc.name, got, tc.apply)
		}
	}
}

func TestExporter_IsCoLocated(t *testing.T) {
	r := exporter{}
	coLocated := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleExporter},
	}
	standalone := &inventory.Host{
		Name:  "metrics",
		Roles: []inventory.Role{inventory.RoleExporter},
	}
	if !r.isCoLocated(coLocated) {
		t.Error("expected co-located when host has both roles")
	}
	if r.isCoLocated(standalone) {
		t.Error("expected NOT co-located when host has only exporter role")
	}
	if r.isCoLocated(nil) {
		t.Error("nil host should not be co-located")
	}
}

func TestExporter_PlanCoLocatedNoInventoryNeeded(t *testing.T) {
	// Co-located exporters don't need to look up another host —
	// Plan should succeed without inventory in context.
	r := exporter{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleExporter},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "co-located") {
		t.Errorf("co-located Plan should mention 'co-located', got:\n%s", joined)
	}
}

func TestExporter_PlanStandaloneNeedsServerInInventory(t *testing.T) {
	r := exporter{}
	h := &inventory.Host{
		Name:  "metrics",
		Roles: []inventory.Role{inventory.RoleExporter},
	}
	// Inventory has only the exporter, no nut-server.
	inv := &inventory.Inventory{Hosts: []inventory.Host{*h}}
	ctx := WithInventory(context.TODO(), inv)

	_, err := r.Plan(ctx, nil, h)
	if err == nil {
		t.Fatal("Plan for standalone exporter should reject inventory with no nut-server")
	}
	if !strings.Contains(err.Error(), "nut-server") {
		t.Errorf("error should mention 'nut-server', got: %v", err)
	}
}

func TestExporter_PlanStandaloneWithServer(t *testing.T) {
	r := exporter{}
	server := inventory.Host{
		Name: "pi", Address: "192.0.2.10", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	standalone := inventory.Host{
		Name: "metrics", Address: "192.0.2.30", User: "metrics",
		Roles: []inventory.Role{inventory.RoleExporter},
	}
	inv := &inventory.Inventory{Hosts: []inventory.Host{server, standalone}}
	ctx := WithInventory(context.TODO(), inv)

	d, err := r.Plan(ctx, nil, &standalone)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if d.NoOp() {
		t.Error("Plan with nil conn (StateUnknown) should propose actions")
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "192.0.2.10") {
		t.Errorf("standalone Plan should mention nut-server address, got:\n%s", joined)
	}
}

func TestExporter_PlanStandaloneWithoutInventoryFallsBack(t *testing.T) {
	// Without inventory in ctx, standalone Plan should still produce
	// a generic preview with a placeholder rather than erroring.
	// Apply is where the strict check happens.
	r := exporter{}
	h := &inventory.Host{
		Name:  "metrics",
		Roles: []inventory.Role{inventory.RoleExporter},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("fallback Plan: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "resolved at apply time") {
		t.Errorf("fallback Plan should mention resolution deferral, got:\n%s", joined)
	}
}

func TestExporter_ApplyNilConnFailsFast(t *testing.T) {
	r := exporter{}
	h := &inventory.Host{
		Name:  "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleExporter},
	}
	err := r.Apply(context.TODO(), nil, h, nil)
	if err == nil || !strings.Contains(err.Error(), "nil connection") {
		t.Errorf("expected nil-connection error, got: %v", err)
	}
}

func TestExporter_RegisteredOnInit(t *testing.T) {
	r, ok := ByName("exporter")
	if !ok {
		t.Fatal("exporter not registered in roles.All()")
	}
	if r.Name() != "exporter" {
		t.Errorf("ByName(exporter).Name() = %q", r.Name())
	}
}

func TestExporter_EmbeddedScriptReadable(t *testing.T) {
	b, err := readScript("setup-exporter.sh")
	if err != nil {
		t.Fatalf("readScript: %v", err)
	}
	if len(b) < 100 {
		t.Errorf("setup-exporter.sh suspiciously short: %d bytes", len(b))
	}
	if !strings.HasPrefix(string(b), "#!/bin/bash") {
		t.Errorf("setup-exporter.sh missing shebang: %q", string(b[:20]))
	}
}

func TestExporter_DetectCmdShape(t *testing.T) {
	for _, marker := range []string{"OK", "PARTIAL", "MISSING", "nut_exporter", "nut-exporter", "systemctl"} {
		if !strings.Contains(exporterDetectCmd, marker) {
			t.Errorf("exporterDetectCmd missing %q:\n%s", marker, exporterDetectCmd)
		}
	}
}
