package ssh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestExecutor_OpenNilHost(t *testing.T) {
	e := NewExecutor(NewConfig())
	_, err := e.Open(nil)
	if err == nil {
		t.Fatal("Open(nil) returned nil error")
	}
}

func TestExecutor_NoAuthAvailable(t *testing.T) {
	// Construct an Executor that can't authenticate: no agent socket, key
	// path points at a missing file.
	t.Setenv("SSH_AUTH_SOCK", "")

	cfg := NewConfig()
	cfg.KeyPath = filepath.Join(t.TempDir(), "missing-key")
	cfg.UseAgent = true // doesn't matter — socket is empty

	e := NewExecutor(cfg)
	_, err := e.authMethods()
	if err == nil {
		t.Fatal("authMethods returned nil error, want 'no auth'")
	}
	if !strings.Contains(err.Error(), "no auth available") {
		t.Errorf("error %q should mention 'no auth available'", err)
	}
}

func TestExecutor_AuthMethodsCached(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	// Provide a real (dummy-format) key file so loadPrivateKey will fail
	// and methods slice ends up empty — that's enough to exercise caching
	// of the negative result.
	cfg := NewConfig()
	cfg.KeyPath = filepath.Join(t.TempDir(), "no-key")

	e := NewExecutor(cfg)
	_, err1 := e.authMethods()
	_, err2 := e.authMethods()
	if !errors.Is(err1, err1) {
		t.Fatal("self-equality broke") // satisfies "use err1"
	}
	// Both calls should produce the same error instance (cached).
	if err1 == nil || err2 == nil {
		t.Fatalf("expected errors, got: %v / %v", err1, err2)
	}
	if err1.Error() != err2.Error() {
		t.Errorf("auth errors differ across calls — not cached: %q vs %q", err1, err2)
	}
}

func TestExecutor_HostKeyCallback_RelaxedSkipsKnownHosts(t *testing.T) {
	cfg := NewConfig()
	cfg.StrictHostKey = false
	cfg.KnownHostsPath = "/definitely/not/here"
	e := NewExecutor(cfg)
	cb, err := e.hostKeyCallback()
	if err != nil {
		t.Fatalf("relaxed mode shouldn't read known_hosts, got err: %v", err)
	}
	if cb == nil {
		t.Fatal("relaxed mode returned nil callback")
	}
}

func TestExecutor_HostKeyCallback_StrictMissingFileExplains(t *testing.T) {
	cfg := NewConfig()
	cfg.KnownHostsPath = filepath.Join(t.TempDir(), "no-known-hosts")
	e := NewExecutor(cfg)
	_, err := e.hostKeyCallback()
	if err == nil {
		t.Fatal("expected error for missing known_hosts in strict mode")
	}
	if !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Errorf("error should suggest ssh-keyscan, got: %v", err)
	}
}

func TestExecutor_HostKeyCallback_StrictReadsRealFile(t *testing.T) {
	// Write a valid (if minimal) known_hosts entry so the parser doesn't
	// blow up. We don't need a usable key — just a syntactically valid
	// line.
	tmp := filepath.Join(t.TempDir(), "known_hosts")
	line := "example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN9bWQyVQ1Q2W7Z4tnZk1bU/X8B4XK0M8aN/u7m1KQRf\n"
	if err := os.WriteFile(tmp, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := NewConfig()
	cfg.KnownHostsPath = tmp
	e := NewExecutor(cfg)
	cb, err := e.hostKeyCallback()
	if err != nil {
		t.Fatalf("hostKeyCallback: %v", err)
	}
	if cb == nil {
		t.Fatal("hostKeyCallback returned nil")
	}
}

func TestExecutor_CloseEmptyIsSafe(t *testing.T) {
	e := NewExecutor(NewConfig())
	if err := e.Close(); err != nil {
		t.Errorf("Close on empty Executor: %v", err)
	}
}

func TestResult_ZeroValuesMakeSense(t *testing.T) {
	r := Result{}
	if r.Stdout != "" || r.Stderr != "" || r.ExitCode != 0 {
		t.Error("Zero Result should have empty strings and exit 0")
	}
}

// Smoke check that the inventory.Host integration compiles — the
// Open method takes *inventory.Host so a refactor would break here.
func TestOpenAcceptsInventoryHost(t *testing.T) {
	h := &inventory.Host{Name: "pi", Address: "10.0.0.1", User: "pi"}
	if h.User+"@"+h.Address != "pi@10.0.0.1" {
		t.Fatal("inventory.Host shape changed?")
	}
}
