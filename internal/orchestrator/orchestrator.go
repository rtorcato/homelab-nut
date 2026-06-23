// Package orchestrator walks the inventory, opens SSH connections,
// and runs roles per host. It backs both `homelab-nut plan` (read-only
// detect+plan) and `homelab-nut apply` (the same plus Apply).
//
// Ordering inside a single host is deterministic — see roleOrder.
// Across hosts, the orchestrator runs concurrently with a configurable
// max parallelism so a 10-machine fleet doesn't serially SSH into each
// host one at a time.
package orchestrator

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/roles"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

// roleOrder is the per-host execution order for Apply. Matches the
// dependency arrows in ROADMAP.md — set up the server before clients,
// install the exporter before pointing the daemon at it, etc.
var roleOrder = []inventory.Role{
	inventory.RoleNUTServer,
	inventory.RoleNUTClient,
	inventory.RoleExporter,
	inventory.RoleShutdownDaemon,
	inventory.RoleShutdownTarget,
}

// RoleOrder returns the per-host execution order Apply uses, as a copy.
// Exposed so UIs can preview the roles a host will run in the same order
// Apply actually runs them.
func RoleOrder() []inventory.Role {
	out := make([]inventory.Role, len(roleOrder))
	copy(out, roleOrder)
	return out
}

// Options tunes the orchestrator's behaviour.
type Options struct {
	// MaxConcurrency caps how many hosts the orchestrator works on
	// in parallel. Zero or negative → unlimited (one goroutine per host).
	MaxConcurrency int
	// SSHConfig is passed straight through to the SSH executor.
	SSHConfig ssh.Config
}

// HostResult records what happened on one host.
//
// JSON tags so `-o json` output has a stable shape. Errors marshal as
// strings (their .Error() text) via the ErrorStrings() helper since
// the error interface itself isn't JSON-marshalable.
type HostResult struct {
	Host    *inventory.Host `json:"host"`
	Diffs   []*roles.Diff   `json:"diffs"`           // populated by Plan + Apply
	Errors  []error         `json:"-"`               // marshalled via ErrorStrings
	Skipped []string        `json:"skipped,omitempty"` // role names skipped (inapplicable)
}

// ErrorStrings returns the host's errors as plain strings for JSON
// serialization. Empty when there are no errors.
func (r *HostResult) ErrorStrings() []string {
	if len(r.Errors) == 0 {
		return nil
	}
	out := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		out[i] = e.Error()
	}
	return out
}

// MarshalJSON adds the "errors" key built from ErrorStrings() so JSON
// output includes failure detail without the un-marshalable error type.
func (r *HostResult) MarshalJSON() ([]byte, error) {
	type alias HostResult
	return jsonMarshalWithErrors(struct {
		*alias
		Errors []string `json:"errors,omitempty"`
	}{
		alias:  (*alias)(r),
		Errors: r.ErrorStrings(),
	})
}

// HasErrors reports whether any role on the host errored.
func (r *HostResult) HasErrors() bool { return len(r.Errors) > 0 }

// Result aggregates across hosts.
type Result struct {
	Hosts []*HostResult `json:"hosts"`
}

// HasErrors reports whether any host had errors.
func (r *Result) HasErrors() bool {
	for _, h := range r.Hosts {
		if h.HasErrors() {
			return true
		}
	}
	return false
}

// NoOp reports whether the result has zero proposed actions across
// all hosts and all roles. Used by Plan to decide whether to print
// "nothing to do".
func (r *Result) NoOp() bool {
	for _, h := range r.Hosts {
		for _, d := range h.Diffs {
			if !d.NoOp() {
				return false
			}
		}
	}
	return true
}

// Plan walks the inventory and computes per-host diffs without making
// any changes. It opens SSH connections so Detect can run against the
// remote state.
func Plan(ctx context.Context, inv *inventory.Inventory, opts Options) *Result {
	return run(ctx, inv, opts, planOnly, nil)
}

// Apply walks the inventory and executes Apply for each role with a
// non-empty diff, streaming each role's output to out with a per-role
// prefix so concurrent host output stays attributable.
func Apply(ctx context.Context, inv *inventory.Inventory, opts Options, out io.Writer) *Result {
	if out == nil {
		out = io.Discard
	}
	return run(ctx, inv, opts, applyAfterPlan, out)
}

type mode int

const (
	planOnly mode = iota
	applyAfterPlan
)

func run(ctx context.Context, inv *inventory.Inventory, opts Options, m mode, out io.Writer) *Result {
	if inv == nil {
		return &Result{}
	}
	// Inventory has to ride along on ctx so roles like nut-client and
	// shutdown-daemon can resolve cross-host data.
	ctx = roles.WithInventory(ctx, inv)

	executor := ssh.NewExecutor(opts.SSHConfig)
	defer func() { _ = executor.Close() }()

	result := &Result{Hosts: make([]*HostResult, len(inv.Hosts))}

	concurrency := opts.MaxConcurrency
	if concurrency <= 0 || concurrency > len(inv.Hosts) {
		concurrency = len(inv.Hosts)
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := range inv.Hosts {
		host := &inv.Hosts[i]
		hr := &HostResult{Host: host}
		result.Hosts[i] = hr

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			runHost(ctx, executor, host, hr, m, out)
		}()
	}
	wg.Wait()
	return result
}

func runHost(ctx context.Context, executor *ssh.Executor, host *inventory.Host, hr *HostResult, m mode, out io.Writer) {
	conn, err := executor.Open(host)
	if err != nil {
		hr.Errors = append(hr.Errors, fmt.Errorf("ssh %s: %w", host.Name, err))
		return
	}

	for _, want := range roleOrder {
		role, ok := roles.ByName(string(want))
		if !ok {
			continue
		}
		if !role.Applies(host) {
			hr.Skipped = append(hr.Skipped, string(want))
			continue
		}

		diff, err := role.Plan(ctx, conn, host)
		if err != nil {
			hr.Errors = append(hr.Errors, fmt.Errorf("%s plan: %w", role.Name(), err))
			continue
		}
		hr.Diffs = append(hr.Diffs, diff)

		if m == planOnly {
			continue
		}
		if diff.NoOp() {
			continue
		}

		// Wrap out with a per-role prefix so interleaved host output is readable.
		pfx := newPrefixWriter(out, fmt.Sprintf("[%s/%s] ", host.Name, role.Name()))
		if err := role.Apply(ctx, conn, host, pfx); err != nil {
			hr.Errors = append(hr.Errors, fmt.Errorf("%s apply: %w", role.Name(), err))
			// Don't continue to subsequent roles on the same host once one
			// fails — later roles often depend on earlier ones.
			return
		}
	}
}

// prefixWriter is a tiny io.Writer that adds a fixed prefix to every
// line written. Imperfect (assumes whole-line writes) but good enough
// for streaming bash output.
type prefixWriter struct {
	w       io.Writer
	prefix  string
	pending bool
}

func newPrefixWriter(w io.Writer, prefix string) *prefixWriter {
	return &prefixWriter{w: w, prefix: prefix, pending: true}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	wrote := 0
	for _, c := range b {
		if p.pending {
			if _, err := io.WriteString(p.w, p.prefix); err != nil {
				return wrote, err
			}
			p.pending = false
		}
		if _, err := p.w.Write([]byte{c}); err != nil {
			return wrote, err
		}
		wrote++
		if c == '\n' {
			p.pending = true
		}
	}
	return wrote, nil
}
