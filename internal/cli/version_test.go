package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmdPrintsAllFields(t *testing.T) {
	info := BuildInfo{Version: "v1.2.3", Commit: "deadbeef", Date: "2026-06-16T00:00:00Z"}
	cmd := newVersionCmd(info)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{"v1.2.3", "deadbeef", "2026-06-16T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("version output missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestRootCmdBuilds(t *testing.T) {
	cmd := newRootCmd(BuildInfo{Version: "test"})
	if cmd.Use != "homelab-nut" {
		t.Errorf("root cmd Use = %q, want homelab-nut", cmd.Use)
	}
	if _, _, err := cmd.Find([]string{"version"}); err != nil {
		t.Errorf("expected version subcommand to be registered: %v", err)
	}
}
