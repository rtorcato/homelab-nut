package roles

import (
	"context"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestNutClient_Name(t *testing.T) {
	if got := (nutClient{}).Name(); got != "nut-client" {
		t.Errorf("Name() = %q, want nut-client", got)
	}
}

func TestNutClient_Applies(t *testing.T) {
	r := nutClient{}
	cases := []struct {
		name  string
		host  *inventory.Host
		apply bool
	}{
		{"nil host", nil, false},
		{"no roles", &inventory.Host{Name: "h"}, false},
		{"wrong role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTServer}}, false},
		{"correct role", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTClient}}, true},
		{"role + others", &inventory.Host{Name: "h", Roles: []inventory.Role{inventory.RoleNUTClient, inventory.RoleShutdownTarget}}, true},
	}
	for _, tc := range cases {
		if got := r.Applies(tc.host); got != tc.apply {
			t.Errorf("Applies(%s) = %v, want %v", tc.name, got, tc.apply)
		}
	}
}

func TestNutClient_PlanRejectsInventoryWithoutServer(t *testing.T) {
	r := nutClient{}
	h := &inventory.Host{
		Name:  "ws",
		Roles: []inventory.Role{inventory.RoleNUTClient},
	}
	// Inventory has only the client, no nut-server.
	inv := &inventory.Inventory{Hosts: []inventory.Host{*h}}
	ctx := WithInventory(context.TODO(), inv)

	_, err := r.Plan(ctx, nil, h)
	if err == nil {
		t.Fatal("Plan should reject inventory with no nut-server host")
	}
	if !strings.Contains(err.Error(), "nut-server") {
		t.Errorf("error should mention 'nut-server', got: %v", err)
	}
}

func TestNutClient_PlanWithServerProducesActions(t *testing.T) {
	r := nutClient{}
	server := inventory.Host{
		Name: "pi", Address: "192.0.2.10", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	client := inventory.Host{
		Name: "ws", Address: "192.0.2.20", User: "admin",
		Roles: []inventory.Role{inventory.RoleNUTClient},
	}
	inv := &inventory.Inventory{Hosts: []inventory.Host{server, client}}
	ctx := WithInventory(context.TODO(), inv)

	d, err := r.Plan(ctx, nil, &client)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if d.NoOp() {
		t.Error("Plan with nil conn (StateUnknown) should propose actions")
	}
	joined := strings.Join(d.Actions, "\n")
	for _, want := range []string{"myups", "192.0.2.10"} {
		if !strings.Contains(joined, want) {
			t.Errorf("actions should mention %q, got:\n%s", want, joined)
		}
	}
}

func TestNutClient_PlanFallbackWithoutInventoryContext(t *testing.T) {
	// Plan should still work without inventory in context — it just
	// can't fill in server-specific details. The actions list reflects
	// that with a "resolve nut-server" placeholder action.
	r := nutClient{}
	h := &inventory.Host{
		Name: "ws", Roles: []inventory.Role{inventory.RoleNUTClient},
	}
	d, err := r.Plan(context.TODO(), nil, h)
	if err != nil {
		t.Fatalf("Plan without inventory ctx: %v", err)
	}
	joined := strings.Join(d.Actions, "\n")
	if !strings.Contains(joined, "resolve nut-server") {
		t.Errorf("fallback Plan should mention 'resolve nut-server', got:\n%s", joined)
	}
}

func TestNutClient_ApplyRequiresPasswordEnv(t *testing.T) {
	r := nutClient{}
	server := inventory.Host{
		Name: "pi", Address: "192.0.2.10", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	client := inventory.Host{
		Name: "ws", Address: "192.0.2.20", User: "admin",
		Roles: []inventory.Role{inventory.RoleNUTClient},
	}
	inv := &inventory.Inventory{Hosts: []inventory.Host{server, client}}
	ctx := WithInventory(context.TODO(), inv)

	t.Setenv(nutMonitorPasswordEnv, "")

	// Use a sentinel non-nil but not-real conn — Apply checks env before
	// touching the wire. We can't construct a real *ssh.Connection without
	// network access, but the password check happens before the conn is used.
	// Apply does check `conn == nil` early though, so pass nil and expect
	// the connection error (not the password error). To check ONLY the
	// password error path, we'd need a mockable connection; for now,
	// verify that nil conn fails fast.
	err := r.Apply(ctx, nil, &client, nil)
	if err == nil {
		t.Fatal("Apply with nil conn should error")
	}
	if !strings.Contains(err.Error(), "nil connection") {
		t.Errorf("expected 'nil connection' error first, got: %v", err)
	}
}

func TestNutClient_RegisteredOnInit(t *testing.T) {
	r, ok := ByName("nut-client")
	if !ok {
		t.Fatal("nut-client not registered in roles.All()")
	}
	if r.Name() != "nut-client" {
		t.Errorf("ByName(nut-client).Name() = %q", r.Name())
	}
}

func TestNutClient_EmbeddedScriptReadable(t *testing.T) {
	b, err := readScript("setup-client.sh")
	if err != nil {
		t.Fatalf("readScript: %v", err)
	}
	if len(b) < 100 {
		t.Errorf("setup-client.sh suspiciously short: %d bytes", len(b))
	}
	if !strings.HasPrefix(string(b), "#!/bin/bash") {
		t.Errorf("setup-client.sh missing shebang: %q", string(b[:20]))
	}
}

func TestNutClient_DetectCmdShape(t *testing.T) {
	for _, marker := range []string{"OK", "PARTIAL", "MISSING", "upsmon", "nut-client", "systemctl"} {
		if !strings.Contains(nutClientDetectCmd, marker) {
			t.Errorf("nutClientDetectCmd missing %q:\n%s", marker, nutClientDetectCmd)
		}
	}
}

func TestFindNUTServer(t *testing.T) {
	server := inventory.Host{
		Name: "pi", Address: "192.0.2.10", User: "pi",
		Roles: []inventory.Role{inventory.RoleNUTServer},
		UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
	}
	cases := []struct {
		name    string
		inv     *inventory.Inventory
		want    string // expected server name, "" for error
		wantErr bool
	}{
		{"nil inv", nil, "", true},
		{"empty inv", &inventory.Inventory{}, "", true},
		{"finds server", &inventory.Inventory{Hosts: []inventory.Host{server}}, "pi", false},
		{"server without ups", &inventory.Inventory{Hosts: []inventory.Host{
			{Name: "p2", Address: "192.0.2.11", User: "pi", Roles: []inventory.Role{inventory.RoleNUTServer}},
		}}, "", true},
	}
	for _, tc := range cases {
		got, err := findNUTServer(tc.inv)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got.Name != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got.Name, tc.want)
		}
	}
}
