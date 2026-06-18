// Package roles defines the abstraction every NUT-setup concern
// implements — nut-server, nut-client, exporter, shutdown-daemon,
// shutdown-target — and the registry the apply orchestrator iterates.
//
// In Phase 2 each role wraps an existing tested bash script over SSH.
// Phase 6 replaces those wrappers with native Go (apt installer + config
// templating + systemd unit writer).
package roles

import (
	"context"
	"fmt"
	"io"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

// State is a coarse classification of how a role looks on a host.
// Implementations choose how to distinguish Missing from Partial — many
// roles will only ever return Missing/OK.
type State int

const (
	StateUnknown State = iota
	StateMissing       // role not configured on this host
	StatePartial       // started but not fully wired up
	StateOK            // fully configured and healthy
)

// String returns a short label for use in diff output.
func (s State) String() string {
	switch s {
	case StateMissing:
		return "missing"
	case StatePartial:
		return "partial"
	case StateOK:
		return "ok"
	default:
		return "unknown"
	}
}

// Diff is what Plan returns: a per-host preview of what Apply would do.
type Diff struct {
	Host    *inventory.Host
	Role    string
	Current State
	Target  State
	// Actions is a list of human-readable steps that Apply would take.
	// Empty when Current == Target (nothing to do).
	Actions []string
}

// NoOp reports whether the diff would do nothing.
func (d *Diff) NoOp() bool { return len(d.Actions) == 0 }

// Format returns a multi-line summary suitable for `homelab-nut plan`.
func (d *Diff) Format() string {
	if d == nil {
		return ""
	}
	header := fmt.Sprintf("[%s] %s  %s → %s",
		d.Role, d.Host.Name, d.Current, d.Target)
	if d.NoOp() {
		return header + "  (no changes)"
	}
	out := header
	for _, a := range d.Actions {
		out += "\n  - " + a
	}
	return out
}

// Role is the interface every setup concern implements. Methods are
// called by the apply orchestrator in the order Applies → Detect →
// Plan → (user confirmation) → Apply.
//
// Implementations must be safe to call multiple times — Apply should
// be idempotent so re-running on a half-applied host converges.
type Role interface {
	// Name returns the role's canonical name (e.g. "nut-server").
	// Must match the role string used in inventory.yaml.
	Name() string

	// Applies reports whether this role applies to the given host.
	// Typically just `h.HasRole(inventory.RoleX)`.
	Applies(h *inventory.Host) bool

	// Detect inspects the remote host and reports the current state
	// of this role. A nil conn means "no connectivity" — implementations
	// may return StateUnknown without erroring in that case.
	Detect(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (State, error)

	// Plan calls Detect and builds a Diff describing what Apply would
	// change. Errors here are user-facing validation problems (e.g.
	// missing required inventory fields) — they should be caught before
	// Apply ever runs.
	Plan(ctx context.Context, conn *ssh.Connection, h *inventory.Host) (*Diff, error)

	// Apply makes the changes described by Plan, streaming any
	// command output to out as it arrives.
	Apply(ctx context.Context, conn *ssh.Connection, h *inventory.Host, out io.Writer) error
}

// registry holds the canonical role list. Concrete role packages
// register themselves via init() so the CLI only needs to import
// this package to see all of them.
var registry []Role

// Register adds r to the global role list. Intended for role package
// init() functions — not for runtime use.
func Register(r Role) {
	registry = append(registry, r)
}

// All returns every registered role in registration order. The
// returned slice is a copy — callers can sort or filter freely.
func All() []Role {
	out := make([]Role, len(registry))
	copy(out, registry)
	return out
}

// ByName looks up a role by Name(). Returns false when not found.
func ByName(name string) (Role, bool) {
	for _, r := range registry {
		if r.Name() == name {
			return r, true
		}
	}
	return nil, false
}

// ForHost returns the subset of registered roles whose Applies(h)
// returns true. Useful when iterating "what will run on this host".
func ForHost(h *inventory.Host) []Role {
	out := make([]Role, 0, len(registry))
	for _, r := range registry {
		if r.Applies(h) {
			out = append(out, r)
		}
	}
	return out
}
