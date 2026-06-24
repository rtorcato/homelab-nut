package cli

import (
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func detectInv() *inventory.Inventory {
	return &inventory.Inventory{
		Hosts: []inventory.Host{
			{
				Name: "pi", Address: "10.0.0.1", User: "root",
				Roles: []inventory.Role{inventory.RoleNUTServer},
				UPS:   &inventory.UPS{Name: "myups", Driver: "usbhid-ups"},
			},
			{
				Name: "nas", Address: "10.0.0.2", User: "root",
				Roles:    []inventory.Role{inventory.RoleShutdownTarget},
				Shutdown: &inventory.Shutdown{Command: "poweroff"},
			},
		},
	}
}

func TestDetectTargetsAllNUTServers(t *testing.T) {
	got, err := detectTargets(detectInv(), "")
	if err != nil {
		t.Fatalf("detectTargets(all) error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "pi" {
		t.Fatalf("detectTargets(all) = %v, want [pi]", names(got))
	}
}

func TestDetectTargetsNamed(t *testing.T) {
	got, err := detectTargets(detectInv(), "pi")
	if err != nil {
		t.Fatalf("detectTargets(pi) error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "pi" {
		t.Fatalf("detectTargets(pi) = %v, want [pi]", names(got))
	}
}

func TestDetectTargetsErrors(t *testing.T) {
	if _, err := detectTargets(detectInv(), "ghost"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown host: want 'not found' error, got %v", err)
	}
	if _, err := detectTargets(detectInv(), "nas"); err == nil || !strings.Contains(err.Error(), "nut-server") {
		t.Errorf("non-nut-server host: want 'nut-server' error, got %v", err)
	}
}

func TestApplyDetectedDriver(t *testing.T) {
	// no UPS block → create with default name.
	h := &inventory.Host{Name: "x"}
	if !applyDetectedDriver(h, "blazer_usb") {
		t.Fatal("expected change when host has no UPS block")
	}
	if h.UPS == nil || h.UPS.Name != "myups" || h.UPS.Driver != "blazer_usb" {
		t.Errorf("UPS = %+v, want {myups blazer_usb}", h.UPS)
	}
	// same driver → no change.
	if applyDetectedDriver(h, "blazer_usb") {
		t.Error("expected no change when driver matches")
	}
	// different driver → update.
	if !applyDetectedDriver(h, "usbhid-ups") || h.UPS.Driver != "usbhid-ups" {
		t.Errorf("expected driver updated to usbhid-ups, got %q", h.UPS.Driver)
	}
	// empty driver → no change.
	if applyDetectedDriver(h, "") {
		t.Error("empty driver should never count as a change")
	}
}

func names(hosts []*inventory.Host) []string {
	out := make([]string, len(hosts))
	for i, h := range hosts {
		out[i] = h.Name
	}
	return out
}
