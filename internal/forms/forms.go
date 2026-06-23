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

	"github.com/charmbracelet/huh"
	"github.com/rtorcato/homelab-nut/internal/inventory"
)

// ErrAborted is returned (wrapped) when the user backs out of a form with
// esc/ctrl+c. Callers that drive forms outside the one-shot `init` flow —
// e.g. the TUI's add/edit/delete-host shortcuts — treat it as a no-op
// rather than a hard error. It aliases huh's sentinel so callers don't
// need to import huh directly.
var ErrAborted = huh.ErrUserAborted

// AskHost runs the guided wizard for a brand-new host: name/address/user/
// roles plus the conditional UPS and shutdown sub-forms when those roles
// are selected. index is the 1-based human-friendly host number shown in
// the form title.
func AskHost(index int) (*inventory.Host, error) {
	host := &inventory.Host{}
	roleStrings := []string{}

	if err := hostForm(fmt.Sprintf("Host #%d", index), host, &roleStrings).Run(); err != nil {
		return nil, err
	}
	return collectRoleDetails(host, roleStrings)
}

// EditHost runs the same guided wizard as AskHost, but seeded with an
// existing host's values so the user edits in place. The returned host is
// a fresh value safe to assign back into the inventory slice; UPS and
// shutdown config is dropped when the corresponding role is unchecked.
func EditHost(existing *inventory.Host) (*inventory.Host, error) {
	host := *existing // value copy — UPS/Shutdown pointers reseeded below
	roleStrings := make([]string, len(existing.Roles))
	for i, r := range existing.Roles {
		roleStrings[i] = string(r)
	}

	if err := hostForm("Edit host: "+existing.Name, &host, &roleStrings).Run(); err != nil {
		return nil, err
	}
	return collectRoleDetails(&host, roleStrings)
}

// hostForm builds the first wizard step (identity + roles) bound to the
// given host and role-selection slice. Shared by AskHost and EditHost so
// the new-host and edit-host flows can never drift apart.
func hostForm(title string, host *inventory.Host, roleStrings *[]string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(title),
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
				Description("What this host does. Pick at least one.").
				Options(roleOptions()...).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return errors.New("at least one role is required")
					}
					return nil
				}).
				Value(roleStrings),
		),
	)
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
func collectRoleDetails(host *inventory.Host, roleStrings []string) (*inventory.Host, error) {
	host.Roles = make([]inventory.Role, len(roleStrings))
	for i, r := range roleStrings {
		host.Roles[i] = inventory.Role(r)
	}

	if host.HasRole(inventory.RoleNUTServer) {
		ups := host.UPS
		if ups == nil {
			ups = &inventory.UPS{}
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewNote().Title(fmt.Sprintf("UPS on %s", host.Name)),
			huh.NewInput().
				Title("UPS name").
				Description("As reported by `upsc -l` — typically `myups`").
				Value(&ups.Name).
				Validate(RequireNonEmpty("ups.name")),
			huh.NewInput().
				Title("Driver").
				Description("e.g. usbhid-ups, blazer_usb, snmp-ups").
				Placeholder("usbhid-ups").
				Value(&ups.Driver).
				Validate(RequireNonEmpty("ups.driver")),
		)).Run(); err != nil {
			return nil, err
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
		if err := huh.NewForm(huh.NewGroup(
			huh.NewNote().Title(fmt.Sprintf("Shutdown command for %s", host.Name)),
			huh.NewInput().
				Title("Command").
				Description("Path → wrapped in nohup over SSH. Bare command (e.g. `poweroff`) → sent inline.").
				Value(&sd.Command).
				Validate(RequireNonEmpty("shutdown.command")),
		)).Run(); err != nil {
			return nil, err
		}
		host.Shutdown = sd
	} else {
		host.Shutdown = nil
	}

	return host, nil
}

// AskShutdownDaemon runs the global daemon-config form.
func AskShutdownDaemon() (*inventory.ShutdownDaemon, error) {
	d := &inventory.ShutdownDaemon{Threshold: 50, PollInterval: 30}
	thresholdStr := strconv.Itoa(d.Threshold)
	pollStr := strconv.Itoa(d.PollInterval)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Shutdown daemon — global config"),
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
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	d.Threshold, _ = strconv.Atoi(thresholdStr)
	d.PollInterval, _ = strconv.Atoi(pollStr)
	return d, nil
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
