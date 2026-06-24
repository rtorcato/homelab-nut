// Package forms holds the charmbracelet/huh form definitions for the
// inventory bootstrap flow. Lifted out of internal/cli/init.go so the
// TUI can drive the exact same forms without duplicating logic.
//
// The inventory package stays pure data + validation; this package
// adds the huh dependency so consumers that don't need interactive
// prompts (e.g. tests, status pollers) don't drag huh into their dep
// graph.
package forms

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/rtorcato/homelab-nut/internal/inventory"
)

// ErrAborted is returned (wrapped) when the user backs out of a form with
// esc/ctrl+c. Callers that drive forms outside the one-shot `init` flow —
// e.g. the TUI's add/edit/delete-host shortcuts — treat it as a no-op
// rather than a hard error. It aliases huh's sentinel so callers don't
// need to import huh directly.
var ErrAborted = huh.ErrUserAborted

// cancelHint is shown on every wizard step's note so the user always sees
// how to back out. huh's own footer is built from the focused field's
// keybinds and can't carry the form-level cancel binding, so we surface it
// here instead.
const cancelHint = "Esc or Ctrl+C cancels — nothing is saved unless you finish the wizard."

// cancelKeyMap is huh's default keymap with Esc added to the abort binding
// (huh binds only Ctrl+C by default). Applied to every wizard form so Esc
// cancels, matching cancelHint. The form checks this binding before the
// focused field, so Esc reliably aborts from any step.
func cancelKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"))
	return km
}

// DriverDetector is an optional hook the wizard calls to pre-fill the UPS
// driver for a nut-server host before showing the UPS form. It returns the
// detected driver and true on success, or ("", false) to fall back to the
// default. Passing it as a callback keeps this package free of an ssh
// dependency — the cli layer supplies an implementation that scans over
// SSH. nil means "skip detection".
type DriverDetector func(host *inventory.Host) (driver string, ok bool)

// AskHost runs the guided wizard for a brand-new host: name/address/user/
// roles plus the conditional UPS and shutdown sub-forms when those roles
// are selected. index is the 1-based human-friendly host number shown in
// the form title. detect (optional) pre-fills the UPS driver via a scan.
// daemonDefault (optional) seeds the per-host shutdown-daemon form with the
// fleet-wide default when this host has no override yet.
func AskHost(index int, detect DriverDetector, daemonDefault *inventory.ShutdownDaemon) (*inventory.Host, error) {
	host := &inventory.Host{}
	roleStrings := []string{}

	if err := hostForm(fmt.Sprintf("Host #%d", index), host, &roleStrings).Run(); err != nil {
		return nil, err
	}
	return collectRoleDetails(host, roleStrings, detect, daemonDefault)
}

// EditHost runs the same guided wizard as AskHost, but seeded with an
// existing host's values so the user edits in place. The returned host is
// a fresh value safe to assign back into the inventory slice; UPS and
// shutdown config is dropped when the corresponding role is unchecked.
func EditHost(existing *inventory.Host, detect DriverDetector, daemonDefault *inventory.ShutdownDaemon) (*inventory.Host, error) {
	host := *existing // value copy — UPS/Shutdown pointers reseeded below
	roleStrings := make([]string, len(existing.Roles))
	for i, r := range existing.Roles {
		roleStrings[i] = string(r)
	}

	if err := hostForm("Edit host: "+existing.Name, &host, &roleStrings).Run(); err != nil {
		return nil, err
	}
	return collectRoleDetails(&host, roleStrings, detect, daemonDefault)
}

// hostForm builds the first wizard step (identity + roles) bound to the
// given host and role-selection slice. Shared by AskHost and EditHost so
// the new-host and edit-host flows can never drift apart.
func hostForm(title string, host *inventory.Host, roleStrings *[]string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(title).
				Description(cancelHint),
			huh.NewInput().
				Title("Name").
				Description("Short identifier — e.g. pi-rack, workstation, dream-machine").
				Validate(RequireNonEmpty("name")).
				Value(&host.Name),
			huh.NewInput().
				Title("Address").
				Description("IP or hostname reachable via SSH").
				Validate(RequireNonEmpty("address")).
				Value(&host.Address),
			huh.NewInput().
				Title("SSH user").
				Description("Account that can ssh in and (for setup roles) escalate via sudo").
				Validate(RequireNonEmpty("user")).
				Value(&host.User),
			huh.NewMultiSelect[string]().
				Title("Roles").
				Description("What this host does — a host can have several. "+
					"Space toggles, enter confirms. Pick at least one.").
				Options(roleOptions()...).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return errors.New("at least one role is required")
					}
					return nil
				}).
				Value(roleStrings),
		),
	).WithKeyMap(cancelKeyMap())
}

// roleDescriptions is the one-line "what does this role do" blurb shown
// beside each option in the Roles multiselect. Keyed by role so the
// option label and stored value can't drift.
var roleDescriptions = map[inventory.Role]string{
	inventory.RoleNUTServer:      "owns the UPS; serves status over the network",
	inventory.RoleNUTClient:      "monitors the server's UPS; shuts itself down",
	inventory.RoleExporter:       "exposes Prometheus metrics for the UPS",
	inventory.RoleShutdownDaemon: "watches battery %; SSHes shutdowns to targets",
	inventory.RoleShutdownTarget: "gets shut down remotely when battery is low",
}

// roleOptions builds the Roles multiselect options with an aligned
// description beside each role. The option's value stays the bare role
// string (e.g. "nut-server") so the inventory schema is unchanged — only
// the displayed label carries the description.
func roleOptions() []huh.Option[string] {
	// Width the role column to the longest name so the "— desc" parts align.
	w := 0
	for _, r := range inventory.AllRoles {
		if n := len(r.String()); n > w {
			w = n
		}
	}
	opts := make([]huh.Option[string], 0, len(inventory.AllRoles))
	for _, r := range inventory.AllRoles {
		label := fmt.Sprintf("%-*s  — %s", w, r.String(), roleDescriptions[r])
		opts = append(opts, huh.NewOption(label, string(r)))
	}
	return opts
}

// collectRoleDetails finalizes a host after the identity/roles step: it
// converts the selected role strings, then runs the conditional UPS and
// shutdown sub-forms. Pre-existing UPS/shutdown values seed those forms
// (so editing keeps them); deselecting a role clears its config.
func collectRoleDetails(host *inventory.Host, roleStrings []string, detect DriverDetector, daemonDefault *inventory.ShutdownDaemon) (*inventory.Host, error) {
	host.Roles = make([]inventory.Role, len(roleStrings))
	for i, r := range roleStrings {
		host.Roles[i] = inventory.Role(r)
	}

	if host.HasRole(inventory.RoleNUTServer) {
		ups := host.UPS
		if ups == nil {
			ups = &inventory.UPS{}
		}
		// Prefill sensible, editable defaults so a user who doesn't yet
		// know the values can just press enter. The name is a label they
		// choose; the driver defaults to usbhid-ups but is best-effort
		// auto-detected over SSH when a detector is supplied and the host
		// is reachable with nut-scanner installed.
		if ups.Name == "" {
			ups.Name = "myups"
		}
		if ups.Driver == "" {
			ups.Driver = "usbhid-ups"
			if detect != nil {
				if d, ok := detect(host); ok && d != "" {
					ups.Driver = d
				}
			}
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("UPS on %s", host.Name)).
				Description("Optional — both fields are pre-filled with working defaults. "+
					"If you don't know them yet, just press enter; apply auto-detects the\n"+
					"real driver and you can refine these later.\n"+cancelHint),
			huh.NewInput().
				Title("UPS name (optional)").
				Description("A short label you choose — becomes the [section] in ups.conf.").
				Value(&ups.Name),
			huh.NewInput().
				Title("Driver (optional)").
				Description("apply auto-detects this with nut-scanner; usbhid-ups fits most USB UPSes (also blazer_usb, snmp-ups).").
				Value(&ups.Driver),
		)).WithKeyMap(cancelKeyMap()).Run(); err != nil {
			return nil, err
		}
		// Belt-and-suspenders: a nut-server host must carry a UPS name and
		// driver (inventory validation enforces it), so if the user cleared
		// a field, fall back to the default rather than failing the save.
		if strings.TrimSpace(ups.Name) == "" {
			ups.Name = "myups"
		}
		if strings.TrimSpace(ups.Driver) == "" {
			ups.Driver = "usbhid-ups"
		}
		host.UPS = ups
	} else {
		host.UPS = nil
	}

	if host.HasRole(inventory.RoleShutdownTarget) {
		sd := host.Shutdown
		if sd == nil {
			sd = &inventory.Shutdown{Command: "~/shutdown.sh"}
		}
		delayStr := ""
		if sd.Delay > 0 {
			delayStr = strconv.Itoa(sd.Delay)
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("Shutdown command for %s", host.Name)).
				Description(cancelHint),
			huh.NewInput().
				Title("Command").
				Description("Path → wrapped in nohup over SSH. Bare command (e.g. `poweroff`) → sent inline.").
				Value(&sd.Command).
				Validate(RequireNonEmpty("shutdown.command")),
			huh.NewInput().
				Title("Delay before shutdown (seconds, optional)").
				Description("Wait this long before sending the command — e.g. let a NAS finish before the gateway it talks through powers off. Blank or 0 = no wait.").
				Value(&delayStr).
				Validate(NonNegativeIntOrEmpty("shutdown.delay")),
		)).WithKeyMap(cancelKeyMap()).Run(); err != nil {
			return nil, err
		}
		sd.Delay = 0
		if s := strings.TrimSpace(delayStr); s != "" {
			sd.Delay, _ = strconv.Atoi(s)
		}
		host.Shutdown = sd
	} else {
		host.Shutdown = nil
	}

	if host.HasRole(inventory.RoleShutdownDaemon) {
		// Seed from this host's existing override, else the fleet-wide
		// default, else the built-in 50%/30s. Copy so the form never mutates
		// the shared global default in place.
		sd := host.ShutdownDaemon
		switch {
		case sd != nil:
			// edit in place
		case daemonDefault != nil:
			cp := *daemonDefault
			sd = &cp
		default:
			sd = &inventory.ShutdownDaemon{Threshold: 50, PollInterval: 30}
		}
		if err := runDaemonForm(fmt.Sprintf("Shutdown daemon on %s", host.Name), sd); err != nil {
			return nil, err
		}
		host.ShutdownDaemon = sd
	} else {
		host.ShutdownDaemon = nil
	}

	return host, nil
}

// runDaemonForm runs the battery-watch daemon config form bound to d,
// mutating it in place. Shared by the per-host shutdown-daemon wizard step
// (collectRoleDetails); the title names the host the daemon runs on.
func runDaemonForm(title string, d *inventory.ShutdownDaemon) error {
	thresholdStr := strconv.Itoa(d.Threshold)
	pollStr := strconv.Itoa(d.PollInterval)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(title).
				Description("Battery-watch tuning for this host's daemon. "+cancelHint),
			huh.NewInput().
				Title("Battery threshold (%)").
				Description("Trigger shutdown when battery drops below this on battery.").
				Value(&thresholdStr).
				Validate(IntInRange("threshold", 1, 99)),
			huh.NewInput().
				Title("Poll interval (seconds)").
				Description("How often the daemon checks the UPS.").
				Value(&pollStr).
				Validate(IntMin("poll_interval", 1)),
			huh.NewInput().
				Title("Slack webhook env var").
				Description("Name of an env var holding the webhook URL. Leave blank to skip Slack.").
				Placeholder("SLACK_WEBHOOK").
				Value(&d.SlackWebhookEnv),
		),
	).WithKeyMap(cancelKeyMap())
	if err := form.Run(); err != nil {
		return err
	}
	d.Threshold, _ = strconv.Atoi(thresholdStr)
	d.PollInterval, _ = strconv.Atoi(pollStr)
	return nil
}

// ConfirmOverwrite is the y/N prompt shown when init would clobber an
// existing inventory.
func ConfirmOverwrite(path string) (bool, error) {
	var overwrite bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("%s already exists. Overwrite?", path)).
		Affirmative("Overwrite").
		Negative("Cancel").
		Value(&overwrite).
		Run()
	return overwrite, err
}

// ConfirmDeleteHost is the destructive-action guard shown before removing
// a host from the inventory.
func ConfirmDeleteHost(name string) (bool, error) {
	var del bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Delete host %q?", name)).
		Description("Removes it from the inventory. This cannot be undone.").
		Affirmative("Delete").
		Negative("Keep").
		Value(&del).
		Run()
	return del, err
}

// ConfirmAddAnother is the loop-control prompt between host entries.
func ConfirmAddAnother() (bool, error) {
	var addAnother bool
	err := huh.NewConfirm().
		Title("Add another host?").
		Value(&addAnother).
		Run()
	return addAnother, err
}

// ConfirmSave is the final write confirmation after the YAML preview.
func ConfirmSave(path string) (bool, error) {
	var save bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Write %s?", path)).
		Affirmative("Write").
		Negative("Discard").
		Value(&save).
		Run()
	return save, err
}

// RequireNonEmpty returns a Validate function that rejects empty/whitespace.
func RequireNonEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

// IntInRange returns a Validate function for integer strings within [lo, hi].
func IntInRange(field string, lo, hi int) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("%s must be a number", field)
		}
		if n < lo || n > hi {
			return fmt.Errorf("%s must be between %d and %d", field, lo, hi)
		}
		return nil
	}
}

// IntMin returns a Validate function for integer strings >= lo.
func IntMin(field string, lo int) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("%s must be a number", field)
		}
		if n < lo {
			return fmt.Errorf("%s must be at least %d", field, lo)
		}
		return nil
	}
}

// NonNegativeIntOrEmpty validates an optional whole-second field: empty is
// allowed (treated as 0 by the caller), otherwise it must be an integer >= 0.
func NonNegativeIntOrEmpty(field string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return fmt.Errorf("%s must be a non-negative number of seconds", field)
		}
		return nil
	}
}
