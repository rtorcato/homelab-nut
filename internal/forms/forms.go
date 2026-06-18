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

// AskHost runs the guided form for a single host: name/address/user/
// roles plus the conditional UPS and shutdown sub-forms when those
// roles are selected. index is the 1-based human-friendly host number
// shown in the form title.
func AskHost(index int) (*inventory.Host, error) {
	host := &inventory.Host{}
	roleStrings := []string{}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(fmt.Sprintf("Host #%d", index)),
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
				Options(huh.NewOptions(
					string(inventory.RoleNUTServer),
					string(inventory.RoleNUTClient),
					string(inventory.RoleExporter),
					string(inventory.RoleShutdownDaemon),
					string(inventory.RoleShutdownTarget),
				)...).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return errors.New("at least one role is required")
					}
					return nil
				}).
				Value(&roleStrings),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	host.Roles = make([]inventory.Role, len(roleStrings))
	for i, r := range roleStrings {
		host.Roles[i] = inventory.Role(r)
	}

	if host.HasRole(inventory.RoleNUTServer) {
		ups := &inventory.UPS{}
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
	}

	if host.HasRole(inventory.RoleShutdownTarget) {
		sd := &inventory.Shutdown{Command: "~/shutdown.sh"}
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
