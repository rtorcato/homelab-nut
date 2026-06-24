package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestRunBackup_RefusesFleetWithoutAll(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runBackup(context.Background(), &out, &errb, p, "", false, "", false, 0, "ts", outputText)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "--all") {
		t.Errorf("stderr should mention --all, got: %q", errb.String())
	}
}

func TestRunBackup_HostNotFound(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runBackup(context.Background(), &out, &errb, p, "ghost", false, "", false, 0, "ts", outputText)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "not found") {
		t.Errorf("stderr should say not found, got: %q", errb.String())
	}
}

func TestRunBackup_FileOutputWithAllRejected(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runBackup(context.Background(), &out, &errb, p, "", true, "/tmp/x.tar.gz", false, 0, "ts", outputText)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "single host") {
		t.Errorf("stderr should reject .tar.gz with --all, got: %q", errb.String())
	}
}

func TestNormaliseConcurrency(t *testing.T) {
	cases := []struct{ requested, n, want int }{
		{0, 5, 5},  // 0 = unlimited → all
		{3, 5, 3},  // honoured
		{9, 5, 5},  // clamped to n
		{-1, 5, 5}, // negative = unlimited
		{2, 0, 1},  // no hosts → floor of 1
	}
	for _, c := range cases {
		if got := normaliseConcurrency(c.requested, c.n); got != c.want {
			t.Errorf("normaliseConcurrency(%d, %d) = %d, want %d", c.requested, c.n, got, c.want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0:       "0 B",
		512:     "512 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		1048576: "1.0 MiB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCountingWriter(t *testing.T) {
	c := &countingWriter{}
	n, err := c.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("Write returned (%d, %v), want (5, nil)", n, err)
	}
	_, _ = c.Write([]byte(" world"))
	if c.n != 11 {
		t.Errorf("counted %d bytes, want 11", c.n)
	}
}

func TestHostInventoryB64_RoundTrips(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: []inventory.Host{
			{Name: "pi", Address: "10.0.0.1", User: "pi", Roles: []inventory.Role{inventory.RoleNUTServer}, UPS: &inventory.UPS{Name: "myups", Driver: "usbhid-ups"}},
			{Name: "ws", Address: "10.0.0.2", User: "admin", Roles: []inventory.Role{inventory.RoleShutdownTarget}, Shutdown: &inventory.Shutdown{Command: "poweroff"}},
		},
		ShutdownDaemon: &inventory.ShutdownDaemon{Threshold: 50, PollInterval: 30},
	}

	enc := hostInventoryB64(inv, &inv.Hosts[0])
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("not valid base64: %v", err)
	}
	// The snapshot must hold only the target host (pi), not the other host.
	yaml := string(raw)
	if !strings.Contains(yaml, "pi") || !strings.Contains(yaml, "myups") {
		t.Errorf("snapshot missing the target host's data:\n%s", yaml)
	}
	if strings.Contains(yaml, "ws") {
		t.Errorf("snapshot leaked another host into pi's backup:\n%s", yaml)
	}

	// It should re-parse as a valid inventory.
	got, err := inventory.LoadReader(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("snapshot should re-parse as inventory, got: %v", err)
	}
	if len(got.Hosts) != 1 || got.Hosts[0].Name != "pi" {
		t.Errorf("re-parsed snapshot = %+v, want single host pi", got.Hosts)
	}
}

func TestBackupScript_StagesExpectedArtifacts(t *testing.T) {
	for _, want := range []string{
		"tar czf - -C \"$STAGE\" .",    // archive to stdout
		"/etc/nut/upsd.users",          // secrets-gated file referenced
		`INCLUDE_SECRETS:-0`,           // secrets gate
		"nut-exporter.service",         // custom unit
		"ups-battery-shutdown.service", // custom unit
		"homelab-nut.yaml",             // inventory snapshot
		"MANIFEST",                     // manifest
		"nut_version:",                 // version capture
	} {
		if !strings.Contains(backupScript, want) {
			t.Errorf("backupScript missing %q", want)
		}
	}
}
