package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rtorcato/homelab-nut/internal/forms"
	"github.com/rtorcato/homelab-nut/internal/inventory"
)

// The host-editing flows below back the TUI's Hosts-screen shortcuts
// ('n' add, 'e' edit, 'd' delete). Because huh forms can't run inside the
// Bubble Tea program, the TUI quits with an exitAction and runTUILoop
// dispatches here, then relaunches the TUI with the reloaded inventory.
// All three reuse forms.AskHost/EditHost and the atomic inventory.Save
// path, so they validate on write exactly like `init` and `inventory edit`.
//
// They print directly to the real terminal (os.Stdout/os.Stderr) — they run
// while the TUI is suspended, same as the huh forms — and runTUILoop pauses
// afterwards so the summary/result is readable before the TUI redraws.

// runAddHost walks the new-host wizard, shows a summary, and appends on
// confirmation.
func runAddHost(path string) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	host, err := forms.AskHost(len(inv.Hosts)+1, wizardDriverDetector, inv.ShutdownDaemon)
	if err != nil {
		return cancelledOrErr(err, "no host added")
	}
	inv.Hosts = append(inv.Hosts, *host)
	return finalizeHostChange(inv, host, path, "added")
}

// runEditHost re-runs the wizard pre-filled with the selected host, then
// shows a summary and saves on confirmation.
func runEditHost(path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	edited, err := forms.EditHost(&inv.Hosts[idx], wizardDriverDetector, inv.ShutdownDaemon)
	if err != nil {
		return cancelledOrErr(err, "no changes made")
	}
	inv.Hosts[idx] = *edited
	return finalizeHostChange(inv, edited, path, "updated")
}

// runDeleteHost confirms, then removes the selected host. It refuses to
// delete the last host — an empty inventory fails validation, and the
// clearer message here beats a raw "at least one host is required".
func runDeleteHost(path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	if len(inv.Hosts) == 1 {
		return errors.New("cannot delete the only host; add another first or run `homelab-nut inventory edit`")
	}
	name := inv.Hosts[idx].Name
	ok, err := forms.ConfirmDeleteHost(name)
	if err != nil {
		return cancelledOrErr(err, "nothing was deleted")
	}
	if !ok {
		fmt.Fprintln(os.Stdout, "Kept — nothing was deleted.")
		return nil
	}
	inv.Hosts = append(inv.Hosts[:idx], inv.Hosts[idx+1:]...)
	if err := saveInventory(inv, path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Saved %s — host %q deleted.\n", path, name)
	return nil
}

// runApplyHost applies a single selected host over SSH, without re-running
// the edit wizard — the TUI's 'a' shortcut to converge a host in place. The
// full inventory is still loaded so cross-host roles resolve; --auto-approve
// since the user explicitly asked to apply this host.
func runApplyHost(path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	return runApply(os.Stdin, os.Stdout, os.Stderr, path, inv.Hosts[idx].Name, true, 0, outputText)
}

// runUninstallHost is the TUI 'u' entry point: uninstall a single host
// interactively. autoApprove is false so the terminal shows the preview and
// a y/N confirm before removing; purge stays off (the upstream NUT package
// is kept — `--purge-nut` is a deliberate CLI-only choice, see uninstall.go).
func runUninstallHost(path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	return runUninstall(os.Stdin, os.Stdout, os.Stderr, path, inv.Hosts[idx].Name, false, "all", false, false, 0, outputText)
}

// runBackupHost is the TUI 'b' entry point: back up a single host to the
// default ./backups dir, secrets excluded (the CLI's --include-secrets is
// the opt-in for a full capture). Read-only on the target.
func runBackupHost(path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	return runBackup(context.Background(), os.Stdout, os.Stderr, path, inv.Hosts[idx].Name, false, "", false, 0, ts, outputText)
}

// finalizeHostChange shows a summary of the affected host and saves only
// after the user confirms. A shutdown-daemon host's battery-watch tuning is
// collected inline in the wizard (forms.collectRoleDetails), so there's no
// separate daemon-config step here.
// action is the past-tense verb shown in the summary ("added"/"updated").
func finalizeHostChange(inv *inventory.Inventory, changed *inventory.Host, path, action string) error {
	// Summary before saving.
	bar := strings.Repeat("─", 52)
	fmt.Fprintf(os.Stdout, "\n%s\nHost to be %s:\n\n", bar, action)
	printHostSummary(os.Stdout, changed)
	fmt.Fprintf(os.Stdout, "%s\n", bar)

	save, err := forms.ConfirmSave(path)
	if err != nil {
		return cancelledOrErr(err, "nothing was saved")
	}
	if !save {
		fmt.Fprintln(os.Stdout, "Discarded — nothing was saved.")
		return nil
	}

	if err := saveInventory(inv, path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Saved %s — host %q %s.\n", path, changed.Name, action)

	// Apply the host now so the change actually takes effect. An unapplied
	// host is just inert YAML — forgetting this step is the most common way
	// a new host looks "broken". Scoped to this host (the full inventory is
	// still loaded so cross-host roles resolve); --auto-approve since the
	// user already confirmed the save.
	fmt.Fprintf(os.Stdout, "\nApplying %s now…\n\n", changed.Name)
	return runApply(os.Stdin, os.Stdout, os.Stderr, path, changed.Name, true, 0, outputText)
}

// printHostSummary renders a host's resolved fields, mirroring the TUI's
// host-detail view so the wizard summary and the detail screen agree.
func printHostSummary(w io.Writer, h *inventory.Host) {
	roles := make([]string, len(h.Roles))
	for i, r := range h.Roles {
		roles[i] = r.String()
	}
	fmt.Fprintf(w, "  name:     %s\n", h.Name)
	fmt.Fprintf(w, "  address:  %s\n", h.Address)
	fmt.Fprintf(w, "  user:     %s\n", h.User)
	fmt.Fprintf(w, "  roles:    %s\n", strings.Join(roles, ", "))
	if h.UPS != nil {
		fmt.Fprintf(w, "  ups:      name=%s driver=%s\n", h.UPS.Name, h.UPS.Driver)
	}
	if h.Shutdown != nil {
		fmt.Fprintf(w, "  shutdown: %s", h.Shutdown.Command)
		if h.Shutdown.Delay > 0 {
			fmt.Fprintf(w, " (after %ds)", h.Shutdown.Delay)
		}
		fmt.Fprintln(w)
	}
	if d := h.ShutdownDaemon; d != nil {
		fmt.Fprintf(w, "  daemon:   threshold=%d%% poll=%ds", d.Threshold, d.PollInterval)
		if d.SlackWebhookEnv != "" {
			fmt.Fprintf(w, " slack=$%s", d.SlackWebhookEnv)
		}
		fmt.Fprintln(w)
	}
}

// saveInventory writes inv to path, formatting a validation failure to
// stderr and returning errSilent (message already shown) so the TUI loop
// doesn't double-report it.
func saveInventory(inv *inventory.Inventory, path string) error {
	if err := inv.Save(path); err != nil {
		var vErr *inventory.ValidationError
		if errors.As(err, &vErr) {
			fmt.Fprintf(os.Stderr, "Could not save: %s\n", err)
			return errSilent
		}
		return err
	}
	return nil
}

// cancelledOrErr maps a user esc/ctrl+c out of a form to a clean no-op with
// a "Cancelled — <what>" note, so the user gets feedback instead of silence.
// Any other error is returned unchanged.
func cancelledOrErr(err error, what string) error {
	if errors.Is(err, forms.ErrAborted) {
		fmt.Fprintf(os.Stdout, "Cancelled — %s.\n", what)
		return nil
	}
	return err
}
