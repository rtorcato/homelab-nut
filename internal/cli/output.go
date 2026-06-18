package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// outputFormat is the value of the --output / -o flag. Subcommands
// branch on it to decide whether to render human-readable text or
// emit machine-readable JSON for AI agents and scripts.
type outputFormat string

const (
	outputText outputFormat = "text"
	outputJSON outputFormat = "json"
)

// addOutputFlag wires --output / -o onto cmd with "text" as the default.
// Subcommands that support structured output should call this in their
// constructor and then read via getOutputFormat.
func addOutputFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", string(outputText),
		"output format: text (human) | json (machine-readable for AI/scripts)")
}

// getOutputFormat returns the format chosen on cmd. Unknown values
// fall back to text with a stderr warning — scripts that mistype get
// readable output rather than a crash.
func getOutputFormat(cmd *cobra.Command) outputFormat {
	v, _ := cmd.Flags().GetString("output")
	switch outputFormat(v) {
	case outputJSON:
		return outputJSON
	case outputText, "":
		return outputText
	default:
		fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: unknown --output value %q, falling back to text\n", v)
		return outputText
	}
}

// emitJSON writes v to w as indented JSON with a trailing newline.
// Used by every --output json branch so output is consistent.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
