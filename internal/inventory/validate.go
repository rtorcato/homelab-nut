package inventory

import (
	"fmt"
	"strings"
)

// ValidationError collects every problem found during validation so the
// user sees them all at once rather than one-per-run.
type ValidationError struct {
	Issues []FieldError
}

// FieldError pairs a dotted field path with a human-readable message.
type FieldError struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "invalid inventory (%d issue", len(e.Issues))
	if len(e.Issues) != 1 {
		b.WriteByte('s')
	}
	b.WriteString("):")
	for _, iss := range e.Issues {
		fmt.Fprintf(&b, "\n  %s: %s", iss.Path, iss.Message)
	}
	return b.String()
}

func (e *ValidationError) add(path, msg string) {
	e.Issues = append(e.Issues, FieldError{Path: path, Message: msg})
}

// Validate checks every rule and returns either nil or a *ValidationError
// listing all problems.
func (inv *Inventory) Validate() error {
	v := &ValidationError{}

	if len(inv.Hosts) == 0 {
		v.add("hosts", "at least one host is required")
	}

	names := make(map[string]int, len(inv.Hosts))
	for i := range inv.Hosts {
		validateHost(v, &inv.Hosts[i], i)

		// Duplicate-name detection: report on the second occurrence.
		if first, seen := names[inv.Hosts[i].Name]; seen && inv.Hosts[i].Name != "" {
			v.add(
				fmt.Sprintf("hosts[%d].name", i),
				fmt.Sprintf("duplicate name %q (also at hosts[%d])", inv.Hosts[i].Name, first),
			)
		} else if inv.Hosts[i].Name != "" {
			names[inv.Hosts[i].Name] = i
		}
	}

	if inv.ShutdownDaemon != nil {
		validateShutdownDaemon(v, "shutdown_daemon", inv.ShutdownDaemon)

		// If a shutdown_daemon block is configured, somebody must run it.
		if len(inv.HostsWithRole(RoleShutdownDaemon)) == 0 {
			v.add("shutdown_daemon", "configured but no host has role 'shutdown-daemon'")
		}
	}

	if len(v.Issues) > 0 {
		return v
	}
	return nil
}

func validateHost(v *ValidationError, h *Host, idx int) {
	p := func(suffix string) string { return fmt.Sprintf("hosts[%d].%s", idx, suffix) }

	if h.Name == "" {
		v.add(p("name"), "required")
	} else if strings.ContainsAny(h.Name, " \t\n") {
		v.add(p("name"), "must not contain whitespace")
	}

	if h.Address == "" {
		v.add(p("address"), "required")
	} else if strings.ContainsAny(h.Address, " \t\n") {
		v.add(p("address"), "must not contain whitespace")
	}

	if h.User == "" {
		v.add(p("user"), "required")
	}

	if len(h.Roles) == 0 {
		v.add(p("roles"), "at least one role is required")
	}
	for ri, r := range h.Roles {
		if !r.Valid() {
			v.add(
				fmt.Sprintf("hosts[%d].roles[%d]", idx, ri),
				fmt.Sprintf("unknown role %q (valid: %s)", r, joinRoles(AllRoles)),
			)
		}
	}

	// nut-server requires a UPS block.
	if h.HasRole(RoleNUTServer) {
		if h.UPS == nil {
			v.add(p("ups"), "required when host has role 'nut-server'")
		} else {
			if h.UPS.Name == "" {
				v.add(p("ups.name"), "required when host has role 'nut-server'")
			}
			if h.UPS.Driver == "" {
				v.add(p("ups.driver"), "required when host has role 'nut-server' (e.g. 'usbhid-ups')")
			}
		}
	}

	// shutdown-target needs a command (or fall back to default at apply time —
	// we only flag if the block exists but command is empty).
	if h.Shutdown != nil && h.Shutdown.Command == "" {
		v.add(p("shutdown.command"), "must be a script path or inline command")
	}

	// Per-host daemon override: validate its values, and flag it on a host
	// that doesn't actually run the daemon (mirrors the global orphan check).
	if h.ShutdownDaemon != nil {
		validateShutdownDaemon(v, p("shutdown_daemon"), h.ShutdownDaemon)
		if !h.HasRole(RoleShutdownDaemon) {
			v.add(p("shutdown_daemon"), "set but host lacks role 'shutdown-daemon'")
		}
	}
}

func validateShutdownDaemon(v *ValidationError, prefix string, d *ShutdownDaemon) {
	if d.Threshold < 1 || d.Threshold > 99 {
		v.add(prefix+".threshold", fmt.Sprintf("%d is out of range (1-99)", d.Threshold))
	}
	if d.PollInterval <= 0 {
		v.add(prefix+".poll_interval", fmt.Sprintf("%d must be positive", d.PollInterval))
	}
	if d.SlackWebhookEnv != "" && strings.ContainsAny(d.SlackWebhookEnv, " \t\n") {
		v.add(prefix+".slack_webhook_env", "must be an env var name, not a URL")
	}
}

func joinRoles(rs []Role) string {
	strs := make([]string, len(rs))
	for i, r := range rs {
		strs[i] = string(r)
	}
	return strings.Join(strs, ", ")
}
