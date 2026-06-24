package inventory

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWriteTo_RoundTrip(t *testing.T) {
	original := &Inventory{
		Hosts: []Host{
			{
				Name: "pi", Address: "192.0.2.10", User: "pi",
				Roles: []Role{RoleNUTServer, RoleExporter, RoleShutdownDaemon},
				UPS:   &UPS{Name: "myups", Driver: "usbhid-ups"},
			},
			{
				Name: "ws", Address: "192.0.2.20", User: "admin",
				Roles:    []Role{RoleNUTClient, RoleShutdownTarget},
				Shutdown: &Shutdown{Command: "~/shutdown.sh"},
			},
		},
		ShutdownDaemon: &ShutdownDaemon{Threshold: 50, PollInterval: 30, SlackWebhookEnv: "SLACK_WEBHOOK"},
	}

	var buf bytes.Buffer
	if err := original.Render(&buf); err != nil {
		t.Fatalf("Render: %v", err)
	}

	roundtripped, err := LoadReader(&buf)
	if err != nil {
		t.Fatalf("load after Render: %v\nyaml was:\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(original, roundtripped) {
		t.Errorf("round-trip mismatch\noriginal:     %+v\nroundtripped: %+v", original, roundtripped)
	}
}

func TestSave_AtomicAndReloadable(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	inv := &Inventory{
		Hosts: []Host{{
			Name: "h1", Address: "192.0.2.1", User: "u",
			Roles: []Role{RoleNUTClient},
		}},
	}

	path := filepath.Join(dir, "homelab-nut.yaml")
	if err := inv.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if got.Hosts[0].Name != "h1" {
		t.Errorf("Hosts[0].Name = %q, want h1", got.Hosts[0].Name)
	}

	// Verify the header comment landed.
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "homelab-nut init") {
		t.Errorf("file missing header comment, got:\n%s", raw)
	}
}

// TestSave_TempFileNextToDestination guards against the cross-device
// rename bug: the atomic temp file must be created in the destination's
// directory, not the CWD, so os.Rename never has to cross filesystems.
// We prove it by making the CWD unwritable — Save must still succeed
// because it writes the temp next to the (writable) destination.
func TestSave_TempFileNextToDestination(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root ignores directory write permissions")
	}

	cwd := t.TempDir()
	dest := t.TempDir()

	prevWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prevWd) })
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	// Make the CWD unwritable so a temp file created in "." would fail.
	if err := os.Chmod(cwd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cwd, 0o755) }) // so TempDir cleanup can rm it

	inv := &Inventory{
		Hosts: []Host{{
			Name: "h1", Address: "192.0.2.1", User: "u",
			Roles: []Role{RoleNUTClient},
		}},
	}

	path := filepath.Join(dest, "homelab-nut.yaml")
	if err := inv.Save(path); err != nil {
		t.Fatalf("Save with unwritable CWD should still succeed (temp goes next to dest), got: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
}

func TestSave_RejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	inv := &Inventory{} // empty -> validation will fail
	err := inv.Save(filepath.Join(dir, "homelab-nut.yaml"))
	if err == nil {
		t.Fatal("expected Save to reject invalid inventory")
	}
}
