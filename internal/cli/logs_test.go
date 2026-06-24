package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestJournalctlCmd(t *testing.T) {
	tests := []struct {
		name   string
		unit   string
		lines  int
		follow bool
		want   string
	}{
		{
			name: "defaults", unit: "ups-battery-shutdown", lines: 200, follow: false,
			want: "journalctl --no-pager -u 'ups-battery-shutdown' -n 200",
		},
		{
			name: "follow", unit: "nut-server", lines: 50, follow: true,
			want: "journalctl --no-pager -u 'nut-server' -n 50 -f",
		},
		{
			name: "zero lines omits -n", unit: "nut-monitor", lines: 0, follow: false,
			want: "journalctl --no-pager -u 'nut-monitor'",
		},
		{
			name: "negative lines omits -n", unit: "nut-server", lines: -5, follow: false,
			want: "journalctl --no-pager -u 'nut-server'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := journalctlCmd(tt.unit, tt.lines, tt.follow); got != tt.want {
				t.Errorf("journalctlCmd(%q, %d, %v) = %q, want %q",
					tt.unit, tt.lines, tt.follow, got, tt.want)
			}
		})
	}
}

func TestShellQuoteEscapesSingleQuote(t *testing.T) {
	// A unit name containing a quote must not let the argument break out of
	// its quoting on the remote shell.
	got := shellQuote(`evil';rm -rf /`)
	want := `'evil'\'';rm -rf /'`
	if got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}

func TestRunLogsUnknownHost(t *testing.T) {
	// A host that isn't in the inventory must error out before any SSH.
	path := writeInv(t, oneHostInv)

	var out, errBuf bytes.Buffer
	err := runLogs(context.Background(), &out, &errBuf, path, "ghost", defaultLogUnit, 200, false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown host: want 'not found' error, got %v", err)
	}
}
