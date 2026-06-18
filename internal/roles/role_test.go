package roles

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

func TestStateString(t *testing.T) {
	cases := map[State]string{
		StateUnknown: "unknown",
		StateMissing: "missing",
		StatePartial: "partial",
		StateOK:      "ok",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestDiff_NoOp(t *testing.T) {
	d := &Diff{
		Host:    &inventory.Host{Name: "pi"},
		Role:    "nut-server",
		Current: StateOK,
		Target:  StateOK,
		Actions: nil,
	}
	if !d.NoOp() {
		t.Error("empty Actions should be NoOp")
	}
	d.Actions = []string{"do thing"}
	if d.NoOp() {
		t.Error("non-empty Actions should not be NoOp")
	}
}

func TestDiff_Format(t *testing.T) {
	d := &Diff{
		Host:    &inventory.Host{Name: "pi"},
		Role:    "nut-server",
		Current: StateMissing,
		Target:  StateOK,
		Actions: []string{"install nut", "configure upsd.users"},
	}
	out := d.Format()
	for _, want := range []string{"nut-server", "pi", "missing", "ok", "install nut", "configure upsd.users"} {
		if !strings.Contains(out, want) {
			t.Errorf("Format() missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestDiff_FormatNoOp(t *testing.T) {
	d := &Diff{
		Host:    &inventory.Host{Name: "pi"},
		Role:    "nut-server",
		Current: StateOK,
		Target:  StateOK,
	}
	if !strings.Contains(d.Format(), "no changes") {
		t.Errorf("NoOp Format() should say 'no changes', got: %s", d.Format())
	}
}

// stubRole is a minimal Role used to exercise the registry.
type stubRole struct {
	name    string
	applies bool
}

func (r stubRole) Name() string                    { return r.name }
func (r stubRole) Applies(*inventory.Host) bool    { return r.applies }
func (stubRole) Detect(context.Context, *ssh.Connection, *inventory.Host) (State, error) {
	return StateUnknown, nil
}
func (stubRole) Plan(context.Context, *ssh.Connection, *inventory.Host) (*Diff, error) {
	return nil, nil
}
func (stubRole) Apply(context.Context, *ssh.Connection, *inventory.Host, io.Writer) error {
	return nil
}

// resetRegistry empties the package-level registry so registry tests
// don't bleed into production data. Used inside test helpers only.
func resetRegistry(t *testing.T) {
	t.Helper()
	saved := registry
	registry = nil
	t.Cleanup(func() { registry = saved })
}

func TestRegistry_RegisterAndByName(t *testing.T) {
	resetRegistry(t)
	a := stubRole{name: "a"}
	b := stubRole{name: "b"}
	Register(a)
	Register(b)

	if got := len(All()); got != 2 {
		t.Errorf("All() len = %d, want 2", got)
	}
	if r, ok := ByName("a"); !ok || r.Name() != "a" {
		t.Errorf("ByName(a) = %v, %v", r, ok)
	}
	if _, ok := ByName("missing"); ok {
		t.Error("ByName(missing) should return false")
	}
}

func TestRegistry_AllReturnsCopy(t *testing.T) {
	resetRegistry(t)
	Register(stubRole{name: "a"})
	got := All()
	got[0] = stubRole{name: "mutated"}
	if All()[0].Name() != "a" {
		t.Error("All() should return a copy, but the registry was mutated")
	}
}

func TestRegistry_ForHost(t *testing.T) {
	resetRegistry(t)
	Register(stubRole{name: "yes", applies: true})
	Register(stubRole{name: "no", applies: false})

	got := ForHost(&inventory.Host{Name: "pi"})
	if len(got) != 1 || got[0].Name() != "yes" {
		t.Errorf("ForHost = %v, want only 'yes'", got)
	}
}
