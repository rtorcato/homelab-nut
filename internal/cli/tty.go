package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
)

// requireTTY returns an error unless f (stdin) is a terminal. Every entry
// point that opens a huh form, the TUI or $EDITOR calls it first: without a
// terminal those can spin on EOF instead of exiting (#173). what names the
// command in the message, e.g. "init". Piped stdin is rejected even when a
// controlling tty exists, since there is no scripted mode to fall back to.
func requireTTY(f *os.File, what string) error {
	fd := f.Fd()
	if isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd) {
		return nil
	}
	return fmt.Errorf("%s needs an interactive terminal (stdin is not a TTY)", what)
}
