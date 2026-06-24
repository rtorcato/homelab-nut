// Package inventory parses and validates homelab-nut.yaml — the declarative
// description of the hosts in a homelab, their roles, UPS attachments, and
// shutdown rules.
//
// A single Load(path) call returns either a fully-validated *Inventory or
// a ValidationError listing every problem found, so users see all issues
// at once rather than one-at-a-time.
package inventory

// Inventory is the root document. Loaded from homelab-nut.yaml.
//
// JSON tags mirror the YAML field names so machine-readable output
// (e.g. `homelab-nut inventory list -o json`) is stable for AI agents
// and scripts to consume.
type Inventory struct {
	Hosts          []Host          `yaml:"hosts"                     json:"hosts"`
	ShutdownDaemon *ShutdownDaemon `yaml:"shutdown_daemon,omitempty" json:"shutdown_daemon,omitempty"`
}

// Host describes a single machine in the fleet.
type Host struct {
	Name     string    `yaml:"name"               json:"name"`
	Address  string    `yaml:"address"            json:"address"`
	User     string    `yaml:"user"               json:"user"`
	Roles    []Role    `yaml:"roles"              json:"roles"`
	UPS      *UPS      `yaml:"ups,omitempty"      json:"ups,omitempty"`
	Shutdown *Shutdown `yaml:"shutdown,omitempty" json:"shutdown,omitempty"`
	// ShutdownDaemon overrides the fleet-wide Inventory.ShutdownDaemon for
	// this host's battery-watch daemon. Only meaningful on a host with role
	// shutdown-daemon. nil = inherit the global default (or built-in 50/30).
	ShutdownDaemon *ShutdownDaemon `yaml:"shutdown_daemon,omitempty" json:"shutdown_daemon,omitempty"`
}

// Role enumerates what a host does.
type Role string

const (
	RoleNUTServer       Role = "nut-server"
	RoleNUTClient       Role = "nut-client"
	RoleExporter        Role = "exporter"
	RoleShutdownDaemon  Role = "shutdown-daemon"
	RoleShutdownTarget  Role = "shutdown-target"
)

// AllRoles is the canonical list (also used for validation error messages).
var AllRoles = []Role{
	RoleNUTServer,
	RoleNUTClient,
	RoleExporter,
	RoleShutdownDaemon,
	RoleShutdownTarget,
}

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	for _, ar := range AllRoles {
		if r == ar {
			return true
		}
	}
	return false
}

// UPS describes a UPS attached to a nut-server host.
type UPS struct {
	Name   string `yaml:"name"   json:"name"`
	Driver string `yaml:"driver" json:"driver"`
}

// Shutdown is per-host shutdown configuration for shutdown-target hosts.
// Command can be either a script path (`~/shutdown.sh` — wrapped in nohup
// over SSH) or an inline command (`poweroff` — sent directly).
//
// Delay is how many seconds the daemon waits before sending this target's
// shutdown — used to sequence dependent devices, e.g. give a NAS time to
// finish before powering off the gateway it talks through. 0 = no wait.
//
// Threshold is the per-target battery % at which the daemon fires this
// target's shutdown, enabling staged shutdown across the fleet (NAS sheds
// early at 60%, the router last at 20%). 0 = inherit the daemon's threshold
// (per-host override → fleet default → built-in 50%).
type Shutdown struct {
	Command   string `yaml:"command"             json:"command"`
	Delay     int    `yaml:"delay,omitempty"     json:"delay,omitempty"`
	Threshold int    `yaml:"threshold,omitempty" json:"threshold,omitempty"`
}

// ShutdownDaemon configures the battery-shutdown daemon. It can be set
// per-host (Host.ShutdownDaemon) and/or fleet-wide (Inventory.ShutdownDaemon):
// a host's own block overrides the global default, which in turn overrides the
// built-in fallback (50% / 30s). See Inventory.EffectiveShutdownDaemon.
type ShutdownDaemon struct {
	Threshold       int    `yaml:"threshold"                   json:"threshold"`
	PollInterval    int    `yaml:"poll_interval"               json:"poll_interval"`
	SlackWebhookEnv string `yaml:"slack_webhook_env,omitempty" json:"slack_webhook_env,omitempty"`
}

// HostByName returns the first host matching name, or nil.
func (inv *Inventory) HostByName(name string) *Host {
	for i := range inv.Hosts {
		if inv.Hosts[i].Name == name {
			return &inv.Hosts[i]
		}
	}
	return nil
}

// HasRole reports whether the host's roles include r.
func (h *Host) HasRole(r Role) bool {
	for _, hr := range h.Roles {
		if hr == r {
			return true
		}
	}
	return false
}

// String formats a Role for display (just the underlying string).
func (r Role) String() string { return string(r) }

// EffectiveShutdownDaemon returns the daemon config in effect for h: the
// host's own override if set, otherwise the fleet-wide default, otherwise nil
// (the caller applies the built-in 50% / 30s fallback). Centralises the
// per-host → global precedence so the role and the TUI agree.
func (inv *Inventory) EffectiveShutdownDaemon(h *Host) *ShutdownDaemon {
	if h != nil && h.ShutdownDaemon != nil {
		return h.ShutdownDaemon
	}
	return inv.ShutdownDaemon
}

// HostsWithRole returns the subset of hosts that include role r.
func (inv *Inventory) HostsWithRole(r Role) []*Host {
	out := make([]*Host, 0, len(inv.Hosts))
	for i := range inv.Hosts {
		if inv.Hosts[i].HasRole(r) {
			out = append(out, &inv.Hosts[i])
		}
	}
	return out
}

