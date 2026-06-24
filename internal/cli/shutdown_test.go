package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestShutdownCheckCmd(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantRemote  string
		wantFailHas string
	}{
		{
			name: "home-relative script", command: "~/shutdown.sh",
			wantRemote:  `test -x "$HOME"/'shutdown.sh'`,
			wantFailHas: "not found or not executable",
		},
		{
			name: "absolute script", command: "/opt/nut/shutdown.sh",
			wantRemote:  `test -x '/opt/nut/shutdown.sh'`,
			wantFailHas: "not found or not executable",
		},
		{
			name: "inline binary", command: "poweroff",
			wantRemote:  `command -v 'poweroff'`,
			wantFailHas: "not found in PATH",
		},
		{
			name: "inline with args checks first token", command: "shutdown -h now",
			wantRemote:  `command -v 'shutdown'`,
			wantFailHas: "not found in PATH",
		},
		{
			name: "home-relative script with args", command: "~/shutdown.sh --force",
			wantRemote:  `test -x "$HOME"/'shutdown.sh'`,
			wantFailHas: "not found or not executable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote, failMsg := shutdownCheckCmd(tt.command)
			if remote != tt.wantRemote {
				t.Errorf("remote = %q, want %q", remote, tt.wantRemote)
			}
			if !strings.Contains(failMsg, tt.wantFailHas) {
				t.Errorf("failMsg = %q, want containing %q", failMsg, tt.wantFailHas)
			}
		})
	}
}

func TestPrintShutdownTestResults(t *testing.T) {
	var buf bytes.Buffer
	printShutdownTestResults(&buf, []shutdownTestResult{
		{Host: "workstation", Command: "~/shutdown.sh", OK: true},
		{Host: "dream-machine", Command: "poweroff", OK: false, Error: "command not found in PATH: poweroff"},
	})
	out := buf.String()
	for _, want := range []string{"workstation", "OK", "dream-machine", "command not found in PATH", "2 host(s) checked, 1 failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestPrintShutdownTestResultsEmpty(t *testing.T) {
	var buf bytes.Buffer
	printShutdownTestResults(&buf, nil)
	if !strings.Contains(buf.String(), "No hosts with the shutdown-target role") {
		t.Errorf("empty result: got %q", buf.String())
	}
}

// No shutdown-target hosts → nothing to SSH, clean exit 0.
func TestRunShutdownTestNoTargets(t *testing.T) {
	// oneHostInv has a single nut-client host, no shutdown-target.
	path := writeInv(t, oneHostInv)
	var out, errBuf bytes.Buffer
	if err := runShutdownTest(context.Background(), &out, &errBuf, path, outputText); err != nil {
		t.Fatalf("no targets should exit 0, got %v", err)
	}
	if !strings.Contains(out.String(), "No hosts with the shutdown-target role") {
		t.Errorf("expected empty-target message, got %q", out.String())
	}
}

func TestExitCodeMapping(t *testing.T) {
	if got := ExitCode(nil); got != ExitOK {
		t.Errorf("ExitCode(nil) = %d, want %d", got, ExitOK)
	}
	if got := ExitCode(errSilent); got != 1 {
		t.Errorf("ExitCode(errSilent) = %d, want 1", got)
	}
	if got := ExitCode(errExit(ExitApplyPartial)); got != ExitApplyPartial {
		t.Errorf("ExitCode(errExit(3)) = %d, want %d", got, ExitApplyPartial)
	}
	if got := ExitCode(errExit(ExitNetwork)); got != ExitNetwork {
		t.Errorf("ExitCode(errExit(2)) = %d, want %d", got, ExitNetwork)
	}
}
