package cli

import (
	"os"
	"strings"
	"testing"
)

func TestRequireTTYRejectsPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	err = requireTTY(r, "init")
	if err == nil {
		t.Fatal("requireTTY accepted a pipe; want an error")
	}
	if want := "init needs an interactive terminal (stdin is not a TTY)"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
}
