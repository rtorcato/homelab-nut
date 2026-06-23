package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const twoHostInv = `hosts:
  - name: a
    address: 10.0.0.1
    user: root
    roles: [nut-client]
  - name: b
    address: 10.0.0.2
    user: root
    roles: [nut-client]
`

const oneHostInv = `hosts:
  - name: solo
    address: 10.0.0.1
    user: root
    roles: [nut-client]
`

// writeInv drops a valid inventory in a temp dir and returns its path.
func writeInv(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "homelab-nut.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The form-driven happy paths need a TTY; these cover the guards that run
// before any form, so they're safe headless.

func TestRunDeleteHostRefusesLastHost(t *testing.T) {
	p := writeInv(t, oneHostInv)
	err := runDeleteHost(p, 0)
	if err == nil || !strings.Contains(err.Error(), "only host") {
		t.Fatalf("deleting the only host: want 'only host' error, got %v", err)
	}
	// Inventory must be untouched.
	if got, _ := os.ReadFile(p); string(got) != oneHostInv {
		t.Errorf("inventory was modified despite refused delete")
	}
}

func TestRunDeleteHostOutOfRange(t *testing.T) {
	p := writeInv(t, twoHostInv)
	if err := runDeleteHost(p, 5); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("want out-of-range error, got %v", err)
	}
}

func TestRunEditHostOutOfRange(t *testing.T) {
	p := writeInv(t, twoHostInv)
	if err := runEditHost(p, -1); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("want out-of-range error, got %v", err)
	}
}

func TestRunEditHostMissingInventory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	if err := runEditHost(missing, 0); err == nil {
		t.Fatal("editing a host in a missing inventory should error")
	}
}
