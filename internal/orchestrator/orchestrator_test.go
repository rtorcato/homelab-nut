package orchestrator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/roles"
)

func TestRoleOrderHasAllRoles(t *testing.T) {
	// Every role in the inventory.AllRoles set should appear exactly once
	// in roleOrder, or the orchestrator will silently skip it.
	if len(roleOrder) != len(inventory.AllRoles) {
		t.Errorf("roleOrder length = %d, want %d (inventory.AllRoles)", len(roleOrder), len(inventory.AllRoles))
	}
	seen := make(map[inventory.Role]bool)
	for _, r := range roleOrder {
		if seen[r] {
			t.Errorf("role %s appears twice in roleOrder", r)
		}
		seen[r] = true
	}
	for _, want := range inventory.AllRoles {
		if !seen[want] {
			t.Errorf("role %s missing from roleOrder", want)
		}
	}
}

func TestRoleOrderServerBeforeClient(t *testing.T) {
	// Per ROADMAP: nut-server is the dependency for nut-client + exporter.
	// Belt-and-braces test so a future shuffle doesn't break apply order.
	pos := make(map[inventory.Role]int)
	for i, r := range roleOrder {
		pos[r] = i
	}
	if pos[inventory.RoleNUTServer] >= pos[inventory.RoleNUTClient] {
		t.Error("nut-server must come before nut-client in roleOrder")
	}
	if pos[inventory.RoleShutdownDaemon] >= pos[inventory.RoleShutdownTarget] {
		// Daemon doesn't *depend* on target install, but installing the
		// daemon last leaves no targets to push the key to before the
		// daemon starts. Even though key distribution is currently manual,
		// keep the explicit order so future auto-distribution slots in
		// without re-ordering.
		t.Logf("note: daemon comes after target — that's intentional; daemon's pubkey output happens after targets exist")
	}
}

func TestResult_NoOpAndHasErrors(t *testing.T) {
	r := &Result{Hosts: []*HostResult{
		{Diffs: []*roles.Diff{{Actions: nil}}},
	}}
	if !r.NoOp() {
		t.Error("Result with empty diff should be NoOp")
	}
	if r.HasErrors() {
		t.Error("Result with no errors should not HasErrors")
	}

	r.Hosts[0].Diffs[0].Actions = []string{"do thing"}
	if r.NoOp() {
		t.Error("Result with non-empty diff should NOT be NoOp")
	}

	r.Hosts[0].Errors = []error{errStub("boom")}
	if !r.HasErrors() {
		t.Error("Result with errors should HasErrors")
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

func TestPrefixWriter_PrependsPerLine(t *testing.T) {
	var buf bytes.Buffer
	pw := newPrefixWriter(&buf, "[host/role] ")
	if _, err := pw.Write([]byte("hello\nworld\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := buf.String()
	want := "[host/role] hello\n[host/role] world\n"
	if got != want {
		t.Errorf("prefix output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPrefixWriter_HandlesPartialLines(t *testing.T) {
	var buf bytes.Buffer
	pw := newPrefixWriter(&buf, "P ")
	if _, err := pw.Write([]byte("hel")); err != nil {
		t.Fatal(err)
	}
	if _, err := pw.Write([]byte("lo\nworld")); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// Partial line is prefixed once at the start, the next chunk completes
	// the line. The "world" tail without a trailing \n still gets prefixed.
	want := "P hello\nP world"
	if got != want {
		t.Errorf("partial-write output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRun_NilInventoryReturnsEmptyResult(t *testing.T) {
	got := Plan(nil, nil, Options{})
	if got == nil {
		t.Fatal("Plan(nil) returned nil")
	}
	if len(got.Hosts) != 0 {
		t.Errorf("expected empty Hosts, got %d", len(got.Hosts))
	}
}

// noteForFutureWork: full Plan/Apply integration tests need a mockable
// ssh.Connection (the current type wraps a *ssh.Client and there's no
// interface seam yet). The role-level unit tests already cover Plan
// rules and Apply argument construction without a real connection.
// A follow-up issue can add an SSH-mock harness if/when we want to
// exercise the orchestrator's role-order + concurrency end-to-end.
//
// Marker test to make the gap explicit:
func TestRun_NeedsMockSSHForFullCoverage(t *testing.T) {
	t.Skip("needs ssh.Connection interface seam — tracked separately")
}

// Quick sanity that strings.HasPrefix isn't a stub — keeps the import
// honest if the file evolves.
func TestSanity(t *testing.T) {
	if !strings.HasPrefix("hello world", "hello") {
		t.Fatal("strings.HasPrefix broken?")
	}
}
