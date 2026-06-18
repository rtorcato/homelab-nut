package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/spf13/cobra"
)

// defaultInventoryPath is the conventional location: ./homelab-nut.yaml.
const defaultInventoryPath = "homelab-nut.yaml"

func newInventoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Inspect and edit the homelab-nut inventory",
		Long: `The inventory is a YAML file (default: ./homelab-nut.yaml) describing the
hosts in your homelab, their roles, and the UPS attachments.

Subcommands let you list hosts, validate the file, show a single host,
or open the file in your $EDITOR (re-validating on save).`,
	}
	cmd.AddCommand(newInventoryListCmd())
	cmd.AddCommand(newInventoryValidateCmd())
	cmd.AddCommand(newInventoryShowCmd())
	cmd.AddCommand(newInventoryEditCmd())
	return cmd
}

func newInventoryListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print a table of hosts (or JSON with -o json)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			inv, err := loadInventoryOrReport(cmd.ErrOrStderr(), path)
			if err != nil {
				return err
			}
			if getOutputFormat(cmd) == outputJSON {
				return emitJSON(cmd.OutOrStdout(), inv.Hosts)
			}
			return printHostsTable(cmd.OutOrStdout(), inv)
		},
	}
	addOutputFlag(cmd)
	return cmd
}

func newInventoryValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check the inventory against the schema and rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			_, err := inventory.Load(path)
			if getOutputFormat(cmd) == outputJSON {
				return emitValidateJSON(cmd.OutOrStdout(), path, err)
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", err)
				return errSilent
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s is valid\n", path)
			return nil
		},
	}
	addOutputFlag(cmd)
	return cmd
}

// emitValidateJSON renders the validate result in JSON form.
// Success: {"valid": true, "path": "..."}
// Failure: {"valid": false, "path": "...", "errors": [{"field": "...", "message": "..."}]}
func emitValidateJSON(w io.Writer, path string, err error) error {
	type fieldErr struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	type result struct {
		Valid  bool       `json:"valid"`
		Path   string     `json:"path"`
		Errors []fieldErr `json:"errors,omitempty"`
	}
	res := result{Valid: err == nil, Path: path}
	if err != nil {
		var vErr *inventory.ValidationError
		if errors.As(err, &vErr) {
			for _, iss := range vErr.Issues {
				res.Errors = append(res.Errors, fieldErr{Field: iss.Path, Message: iss.Message})
			}
		} else {
			res.Errors = []fieldErr{{Field: "", Message: err.Error()}}
		}
	}
	out := emitJSON(w, res)
	if err != nil {
		// Surface non-zero exit even on the JSON path.
		return errSilent
	}
	return out
}

func newInventoryShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <host>",
		Short: "Show details for one host (or JSON with -o json)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")
			inv, err := loadInventoryOrReport(cmd.ErrOrStderr(), path)
			if err != nil {
				return err
			}
			h := inv.HostByName(args[0])
			if h == nil {
				return fmt.Errorf("host %q not found in %s", args[0], path)
			}
			if getOutputFormat(cmd) == outputJSON {
				return emitJSON(cmd.OutOrStdout(), h)
			}
			return printHostDetail(cmd.OutOrStdout(), h)
		},
	}
	addOutputFlag(cmd)
	return cmd
}

func newInventoryEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the inventory in $EDITOR and re-validate on save",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := cmd.Flags().GetString("inventory")

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			ed := exec.Command(editor, path)
			ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := ed.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}

			if _, err := inventory.Load(path); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", err)
				return errSilent
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s saved and valid\n", path)
			return nil
		},
	}
}

func printHostsTable(w io.Writer, inv *inventory.Inventory) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tADDRESS\tUSER\tROLES")
	for _, h := range inv.Hosts {
		roles := make([]string, len(h.Roles))
		for i, r := range h.Roles {
			roles[i] = r.String()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.Name, h.Address, h.User, strings.Join(roles, ","))
	}
	return tw.Flush()
}

func printHostDetail(w io.Writer, h *inventory.Host) error {
	roles := make([]string, len(h.Roles))
	for i, r := range h.Roles {
		roles[i] = r.String()
	}

	fmt.Fprintf(w, "name:    %s\n", h.Name)
	fmt.Fprintf(w, "address: %s\n", h.Address)
	fmt.Fprintf(w, "user:    %s\n", h.User)
	fmt.Fprintf(w, "roles:   %s\n", strings.Join(roles, ", "))
	if h.UPS != nil {
		fmt.Fprintf(w, "ups:     name=%s driver=%s\n", h.UPS.Name, h.UPS.Driver)
	}
	if h.Shutdown != nil {
		fmt.Fprintf(w, "shutdown command: %s\n", h.Shutdown.Command)
	}
	return nil
}

// loadInventoryOrReport loads + validates path, writing schema-validation
// errors to stderr as-is. Returns errSilent so the CLI exits non-zero
// without printing the error a second time.
func loadInventoryOrReport(stderr io.Writer, path string) (*inventory.Inventory, error) {
	inv, err := inventory.Load(path)
	if err == nil {
		return inv, nil
	}
	var vErr *inventory.ValidationError
	if errors.As(err, &vErr) {
		fmt.Fprintf(stderr, "%s\n", err)
		return nil, errSilent
	}
	// Parse error, missing file, etc — let cobra print it.
	return nil, err
}

// errSilent is returned to indicate "non-zero exit, message already printed".
// cobra honours SilenceUsage on the root command, so the usage block won't
// be repeated either.
var errSilent = silentErr{}

type silentErr struct{}

func (silentErr) Error() string { return "" }
