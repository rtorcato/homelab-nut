// Package inventory parses and validates homelab-nut.yaml — the declarative
// description of the hosts in a homelab, their roles, UPS attachments, and
// shutdown rules.
//
// A single Load(path) call returns either a fully-validated *Inventory or
// a ValidationError listing every problem found, so users see all issues
// at once rather than one-at-a-time.
package inventory

// Inventory is the root document. Loaded from homelab-nut.yaml.
type Inventory struct {
	Hosts           []Host          `yaml:"hosts"`
	ShutdownDaemon  *ShutdownDaemon `yaml:"shutdown_daemon,omitempty"`
}

// Host describes a single machine in the fleet.
type Host struct {
	Name     string    `yaml:"name"`
	Address  string    `yaml:"address"`
	User     string    `yaml:"user"`
	Roles    []Role    `yaml:"roles"`
	UPS      *UPS      `yaml:"ups,omitempty"`
	Shutdown *Shutdown `yaml:"shutdown,omitempty"`
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
	Name   string `yaml:"name"`
	Driver string `yaml:"driver"`
}

// Shutdown is per-host shutdown configuration for shutdown-target hosts.
// Command can be either a script path (`~/shutdown.sh` — wrapped in nohup
// over SSH) or an inline command (`poweroff` — sent directly).
type Shutdown struct {
	Command string `yaml:"command"`
}

// ShutdownDaemon is the global configuration for the battery-shutdown daemon.
// Lives on the host(s) with role shutdown-daemon.
type ShutdownDaemon struct {
	Threshold       int    `yaml:"threshold"`
	PollInterval    int    `yaml:"poll_interval"`
	SlackWebhookEnv string `yaml:"slack_webhook_env,omitempty"`
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

