package inventory

import (
	"strings"
	"testing"
)

const goodYAML = `
hosts:
  - name: pi-rack
    address: 192.0.2.10
    user: pi
    roles: [nut-server, exporter, shutdown-daemon]
    ups: { name: myups, driver: usbhid-ups }
  - name: workstation
    address: 192.0.2.20
    user: admin
    roles: [shutdown-target]
    shutdown: { command: ~/shutdown.sh }
shutdown_daemon:
  threshold: 50
  poll_interval: 30
`

func TestLoadReader_HappyPath(t *testing.T) {
	inv, err := LoadReader(strings.NewReader(goodYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(inv.Hosts), 2; got != want {
		t.Fatalf("len(Hosts) = %d, want %d", got, want)
	}
	if got := inv.HostByName("pi-rack"); got == nil {
		t.Errorf("HostByName(pi-rack) = nil")
	}
	if got := inv.ShutdownDaemon; got == nil || got.Threshold != 50 {
		t.Errorf("ShutdownDaemon.Threshold = %+v, want 50", got)
	}
}

func TestLoadReader_RejectsUnknownFields(t *testing.T) {
	yml := `
hosts:
  - name: a
    address: 192.0.2.1
    user: u
    roles: [nut-client]
    bogus_field: nope
`
	_, err := LoadReader(strings.NewReader(yml))
	if err == nil {
		t.Fatal("expected strict-mode error, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_field") {
		t.Errorf("error should mention unknown field, got: %v", err)
	}
}

func TestLoadReader_EmptyDocument(t *testing.T) {
	_, err := LoadReader(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error on empty document")
	}
}

func TestLoadReader_MultiDoc(t *testing.T) {
	yml := goodYAML + "\n---\nhosts: []\n"
	_, err := LoadReader(strings.NewReader(yml))
	if err == nil {
		t.Fatal("expected multi-doc error")
	}
}
