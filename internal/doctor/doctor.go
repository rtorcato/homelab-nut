// Package doctor answers "is my shutdown protection actually working right
// now?" for the hosts that carry it. Unlike plan (diffs the host against the
// inventory) and status (polls nut-server hosts only), it probes the world
// from each host's own point of view: is the configured MONITOR target
// reachable, does it have the UPS the host expects, can the shutdown daemon
// read its UPS.
package doctor

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

// Check is one assertion about one host.
type Check struct {
	Name   string `json:"check"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// Result is every check for one host. Error is set (and Checks empty) when
// the host could not be probed at all.
type Result struct {
	Host   string  `json:"host"`
	Checks []Check `json:"checks,omitempty"`
	Error  string  `json:"error,omitempty"`
}

// Healthy reports whether the host was probed and every check passed.
func (r Result) Healthy() bool {
	if r.Error != "" {
		return false
	}
	for _, c := range r.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

// Applies reports whether doctor has anything to assert on h.
func Applies(h *inventory.Host) bool {
	return h.HasRole(inventory.RoleNUTServer) || h.HasRole(inventory.RoleNUTClient) || h.HasRole(inventory.RoleShutdownDaemon)
}

// probeCmd prints one `KIND|field|...` fact per line. The UPS-name lookups
// and the daemon read run on the host itself (`upsc`), so they test what
// that host can actually reach, not what this machine can.
const probeCmd = `rd() { cat "$1" 2>/dev/null || sudo -n cat "$1" 2>/dev/null; }
echo "MODE|$(rd /etc/nut/nut.conf | sed -n 's/^[[:space:]]*MODE[[:space:]]*=[[:space:]]*//p' | tail -1 | tr -d "\"' ")"
if systemctl is-active --quiet nut-monitor || systemctl is-active --quiet nut-client; then echo "UPSMON|active"; else echo "UPSMON|inactive"; fi
conf=$(rd /etc/nut/upsmon.conf)
if [ -z "$conf" ]; then echo "UPSMONCONF|unreadable"; fi
printf '%s\n' "$conf" | awk '$1=="MONITOR"{print $2}' | sort -u | while read -r sys; do
  [ -n "$sys" ] || continue
  out=$(timeout 5 upsc -l -- "${sys#*@}" 2>/dev/null); rc=$?
  echo "LIST|$sys|$rc"
  printf '%s\n' "$out" | while read -r name; do [ -n "$name" ] && echo "UPS|$sys|$name"; done
done
if [ -f /etc/ups-battery-shutdown.conf ]; then
  ref=$(rd /etc/ups-battery-shutdown.conf | sed -n 's/^UPS=//p' | tail -1 | tr -d "\"'")
  ref=${ref:-myups@localhost}
  if systemctl is-active --quiet ups-battery-shutdown; then echo "DAEMON|active"; else echo "DAEMON|inactive"; fi
  st=$(timeout 5 upsc -- "$ref" ups.status 2>/dev/null); rc=$?
  echo "DAEMONREAD|$ref|$rc|$st"
else
  echo "DAEMON|noconf"
fi
true`

type monitor struct {
	target string // "ups@host[:port]"
	rc     int    // exit code of `upsc -l host`
	names  []string
}

// facts is the parsed probe output.
type facts struct {
	mode         string
	upsmonActive bool
	confMissing  bool
	monitors     []*monitor
	daemon       string // active | inactive | noconf | "" (not reported)
	daemonRef    string
	daemonRC     int
	daemonStatus string
}

// Run probes h over conn and evaluates the checks its roles call for.
func Run(ctx context.Context, conn *ssh.Connection, h *inventory.Host) ([]Check, error) {
	res, err := conn.Run(ctx, probeCmd)
	if err != nil {
		return nil, fmt.Errorf("probe %s: %w", h.Name, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("probe %s: exit %d (stderr: %s)", h.Name, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return evaluate(h, parseFacts(res.Stdout)), nil
}

func parseFacts(out string) facts {
	var f facts
	byTarget := map[string]*monitor{}
	for _, line := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(line), "|", 4)
		switch p[0] {
		case "MODE":
			f.mode = field(p, 1)
		case "UPSMON":
			f.upsmonActive = field(p, 1) == "active"
		case "UPSMONCONF":
			f.confMissing = true
		case "LIST":
			rc, _ := strconv.Atoi(field(p, 2))
			m := &monitor{target: field(p, 1), rc: rc}
			byTarget[m.target] = m
			f.monitors = append(f.monitors, m)
		case "UPS":
			if m := byTarget[field(p, 1)]; m != nil {
				m.names = append(m.names, field(p, 2))
			}
		case "DAEMON":
			f.daemon = field(p, 1)
		case "DAEMONREAD":
			f.daemonRef = field(p, 1)
			f.daemonRC, _ = strconv.Atoi(field(p, 2))
			f.daemonStatus = field(p, 3)
		}
	}
	return f
}

func field(p []string, i int) string {
	if i < len(p) {
		return p[i]
	}
	return ""
}

// evaluate turns probe facts into checks, keyed off h's inventory roles.
func evaluate(h *inventory.Host, f facts) []Check {
	var cs []Check
	add := func(name string, ok bool, format string, a ...any) {
		cs = append(cs, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, a...)})
	}

	server := h.HasRole(inventory.RoleNUTServer)
	if server || h.HasRole(inventory.RoleNUTClient) {
		if server {
			ok := f.mode == "netserver" || f.mode == "standalone"
			add("nut.conf mode", ok, "MODE=%q (host has the nut-server role; want netserver or standalone)", f.mode)
		} else {
			add("nut.conf mode", f.mode == "netclient", "MODE=%q (client-only host; want netclient)", f.mode)
		}
		add("upsmon running", f.upsmonActive, "nut-monitor / nut-client service")
		switch {
		case f.confMissing:
			add("monitor targets", false, "/etc/nut/upsmon.conf missing or unreadable")
		case len(f.monitors) == 0:
			add("monitor targets", false, "upsmon.conf has no MONITOR lines")
		}
		for _, m := range f.monitors {
			name, _, _ := strings.Cut(m.target, "@")
			switch {
			case m.rc != 0:
				add("monitor "+m.target, false, "no answer from the NUT server (upsc -l exited %d)", m.rc)
			case !contains(m.names, name):
				add("monitor "+m.target, false, "server answers but has no UPS named %q (has: %s)", name, joinOrNone(m.names))
			default:
				add("monitor "+m.target, true, "reachable, UPS %q exists", name)
			}
		}
	}

	if h.HasRole(inventory.RoleShutdownDaemon) {
		switch f.daemon {
		case "noconf":
			add("shutdown daemon", false, "/etc/ups-battery-shutdown.conf missing")
		default:
			add("shutdown daemon", f.daemon == "active", "ups-battery-shutdown service is %s", orUnknown(f.daemon))
			if f.daemonRC == 0 && f.daemonStatus != "" {
				add("daemon UPS read", true, "%s ups.status=%s", f.daemonRef, f.daemonStatus)
			} else {
				add("daemon UPS read", false, "cannot read ups.status from %s (upsc exited %d)", f.daemonRef, f.daemonRC)
			}
		}
	}
	return cs
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func joinOrNone(ss []string) string {
	if len(ss) == 0 {
		return "none"
	}
	return strings.Join(ss, ", ")
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
