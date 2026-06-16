package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate homelab-nut.yaml interactively",
		Long: `Walks through a guided form, then writes the resulting inventory
to ./homelab-nut.yaml (or the path given via --inventory).

If an inventory already exists, you'll be asked whether to overwrite it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			return runInit(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), path)
		},
	}
}

func runInit(_ io.Reader, stdout, stderr io.Writer, path string) error {
	// 1. Handle existing file: confirm before clobbering.
	if _, err := os.Stat(path); err == nil {
		var overwrite bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("%s already exists. Overwrite?", path)).
			Affirmative("Overwrite").
			Negative("Cancel").
			Value(&overwrite).
			Run()
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintln(stdout, "Cancelled. Existing inventory left untouched.")
			return nil
		}
	}

	inv := &inventory.Inventory{}

	// 2. Add hosts in a loop. At least one is required.
	for {
		host, err := askHost(len(inv.Hosts) + 1)
		if err != nil {
			return err
		}
		inv.Hosts = append(inv.Hosts, *host)

		var addAnother bool
		if err := huh.NewConfirm().
			Title("Add another host?").
			Value(&addAnother).
			Run(); err != nil {
			return err
		}
		if !addAnother {
			break
		}
	}

	// 3. If any host has shutdown-daemon, collect daemon config.
	if len(inv.HostsWithRole(inventory.RoleShutdownDaemon)) > 0 {
		d, err := askShutdownDaemon()
		if err != nil {
			return err
		}
		inv.ShutdownDaemon = d
	}

	// 4. Preview + confirm.
	var preview bytes.Buffer
	if err := inv.Render(&preview); err != nil {
		return fmt.Errorf("render preview: %w", err)
	}
	fmt.Fprintln(stdout, strings.Repeat("─", 60))
	fmt.Fprintln(stdout, preview.String())
	fmt.Fprintln(stdout, strings.Repeat("─", 60))

	var save bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Write %s?", path)).
		Affirmative("Write").
		Negative("Discard").
		Value(&save).
		Run(); err != nil {
		return err
	}
	if !save {
		fmt.Fprintln(stdout, "Discarded. Nothing was written.")
		return nil
	}

	if err := inv.Save(path); err != nil {
		// Validation can fail here if the user produced an inconsistent
		// inventory (e.g. shutdown_daemon block but no daemon host).
		// huh's flow guards against the simple cases but we re-check.
		var vErr *inventory.ValidationError
		if errors.As(err, &vErr) {
			fmt.Fprintf(stderr, "%s\n", err)
			return errSilent
		}
		return err
	}
	fmt.Fprintf(stdout, "Wrote %s — run `homelab-nut inventory list` to confirm.\n", path)
	return nil
}

func askHost(index int) (*inventory.Host, error) {
	host := &inventory.Host{}
	// Bind a temp []string to use the standard MultiSelect, then convert
	// to []Role on submit.
	roleStrings := []string{}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(fmt.Sprintf("Host #%d", index)),
			huh.NewInput().
				Title("Name").
				Description("Short identifier — e.g. pi-rack, workstation, dream-machine").
				Validate(requireNonEmpty("name")).
				Value(&host.Name),
			huh.NewInput().
				Title("Address").
				Description("IP or hostname reachable via SSH").
				Validate(requireNonEmpty("address")).
				Value(&host.Address),
			huh.NewInput().
				Title("SSH user").
				Description("Account that can ssh in and (for setup roles) escalate via sudo").
				Validate(requireNonEmpty("user")).
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

	// Conditional sub-forms. Run them only when relevant — keeps the
	// guided flow tight.
	if host.HasRole(inventory.RoleNUTServer) {
		ups := &inventory.UPS{}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewNote().Title(fmt.Sprintf("UPS on %s", host.Name)),
			huh.NewInput().
				Title("UPS name").
				Description("As reported by `upsc -l` — typically `myups`").
				Value(&ups.Name).
				Validate(requireNonEmpty("ups.name")),
			huh.NewInput().
				Title("Driver").
				Description("e.g. usbhid-ups, blazer_usb, snmp-ups").
				Placeholder("usbhid-ups").
				Value(&ups.Driver).
				Validate(requireNonEmpty("ups.driver")),
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
				Validate(requireNonEmpty("shutdown.command")),
		)).Run(); err != nil {
			return nil, err
		}
		host.Shutdown = sd
	}

	return host, nil
}

func askShutdownDaemon() (*inventory.ShutdownDaemon, error) {
	d := &inventory.ShutdownDaemon{Threshold: 50, PollInterval: 30}
	var thresholdStr = strconv.Itoa(d.Threshold)
	var pollStr = strconv.Itoa(d.PollInterval)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Shutdown daemon — global config"),
			huh.NewInput().
				Title("Battery threshold (%)").
				Description("Trigger shutdown when battery drops below this on battery.").
				Value(&thresholdStr).
				Validate(intInRange("threshold", 1, 99)),
			huh.NewInput().
				Title("Poll interval (seconds)").
				Description("How often the daemon checks the UPS.").
				Value(&pollStr).
				Validate(intMin("poll_interval", 1)),
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
	// Inputs were validated; ignore conversion errors.
	d.Threshold, _ = strconv.Atoi(thresholdStr)
	d.PollInterval, _ = strconv.Atoi(pollStr)
	return d, nil
}

// requireNonEmpty returns a Validate function that rejects empty/whitespace.
func requireNonEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

// intInRange returns a Validate function for integer strings within [lo, hi].
func intInRange(field string, lo, hi int) func(string) error {
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

// intMin returns a Validate function for integer strings >= lo.
func intMin(field string, lo int) func(string) error {
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
