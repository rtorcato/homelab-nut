package cli

import (
	"errors"
	"fmt"

	"github.com/rtorcato/homelab-nut/internal/forms"
	"github.com/rtorcato/homelab-nut/internal/inventory"
)

// The host-editing flows below back the TUI's Hosts-screen shortcuts
// ('n' add, 'e' edit, 'd' delete). Because huh forms can't run inside the
// Bubble Tea program, the TUI quits with an exitAction and runTUILoop
// dispatches here, then relaunches the TUI with the reloaded inventory.
// All three reuse forms.AskHost/EditHost and the atomic inventory.Save
// path, so they validate on write exactly like `init` and `inventory edit`.

// runAddHost walks the new-host wizard and appends the result.
func runAddHost(path string) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	host, err := forms.AskHost(len(inv.Hosts) + 1)
	if err != nil {
		return ignoreAbort(err)
	}
	inv.Hosts = append(inv.Hosts, *host)
	return saveAfterHostChange(inv, path)
}

// runEditHost re-runs the wizard pre-filled with the selected host.
func runEditHost(path string, idx int) error {
	inv, err := inventory.Load(path)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(inv.Hosts) {
		return fmt.Errorf("host index %d out of range (have %d hosts)", idx, len(inv.Hosts))
	}
	edited, err := forms.EditHost(&inv.Hosts[idx])
	if err != nil {
		return ignoreAbort(err)
	}
	inv.Hosts[idx] = *edited
	return saveAfterHostChange(inv, path)
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
	ok, err := forms.ConfirmDeleteHost(inv.Hosts[idx].Name)
	if err != nil {
		return ignoreAbort(err)
	}
	if !ok {
		return nil
	}
	inv.Hosts = append(inv.Hosts[:idx], inv.Hosts[idx+1:]...)
	return saveAfterHostChange(inv, path)
}

// saveAfterHostChange mirrors init's step 3: when a shutdown-daemon host
// now exists but no daemon config does, collect it before saving. Then
// persist through the atomic, re-validating Save path.
func saveAfterHostChange(inv *inventory.Inventory, path string) error {
	if len(inv.HostsWithRole(inventory.RoleShutdownDaemon)) > 0 && inv.ShutdownDaemon == nil {
		d, err := forms.AskShutdownDaemon()
		if err != nil {
			return ignoreAbort(err)
		}
		inv.ShutdownDaemon = d
	}
	return inv.Save(path)
}

// ignoreAbort maps a user esc/ctrl+c out of a form to a clean no-op so the
// TUI just relaunches unchanged instead of surfacing a scary error.
func ignoreAbort(err error) error {
	if errors.Is(err, forms.ErrAborted) {
		return nil
	}
	return err
}
