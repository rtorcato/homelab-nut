package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version, commit, and build date (or JSON with -o json)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if getOutputFormat(cmd) == outputJSON {
				return emitJSON(cmd.OutOrStdout(), versionPayload{
					Version: info.Version,
					Commit:  info.Commit,
					Date:    info.Date,
				})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"homelab-nut %s\ncommit:  %s\nbuilt:   %s\n",
				info.Version, info.Commit, info.Date,
			)
			return err
		},
	}
	addOutputFlag(cmd)
	return cmd
}

// versionPayload is the JSON shape for `version -o json`. Same fields
// as BuildInfo but with json tags so the output stays stable.
type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}
