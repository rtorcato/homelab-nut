package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rtorcato/homelab-nut/internal/upspoll"
	"github.com/rtorcato/homelab-nut/internal/upspoll/upspolltest"
)

func TestPrintStatusTableGolden(t *testing.T) {
	charge := 85.0
	load := 12.0
	runtime := 1800
	rows := []upspoll.Row{
		{Host: "office", Address: "10.0.0.1", UPS: "myups", Status: "OL",
			BatteryCharge: &charge, BatteryRuntime: &runtime, Load: &load},
		{Host: "rack", Address: "10.0.0.2", Error: "timeout"},
	}
	var buf bytes.Buffer
	if err := printStatusTable(&buf, rows); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"HOST", "ADDRESS", "UPS", "STATUS", "BATTERY", "LOAD", "RUNTIME", "ERROR",
		"office", "10.0.0.1", "myups", "OL", "85%", "12%", "30m0s",
		"rack", "10.0.0.2", "timeout",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing %q\n---\n%s", want, got)
		}
	}
}

func TestPrintStatusTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := printStatusTable(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No hosts with the nut-server role") {
		t.Errorf("empty table output = %q", buf.String())
	}
}

func TestRunStatusJSONOneShot(t *testing.T) {
	s := upspolltest.New(t,
		[]string{`UPS myups "Test UPS"`},
		map[string]map[string]string{
			"myups": {
				"ups.status":      "OL",
				"battery.charge":  "100",
				"battery.runtime": "3600",
				"ups.load":        "5",
			},
		},
	)
	path := writeTempInventory(t, fmt.Sprintf(`hosts:
  - name: office
    address: %s
    user: pi
    roles: [nut-server]
    ups:
      name: myups
      driver: usbhid-ups
`, s.Addr()))

	var out, errBuf bytes.Buffer
	err := runStatus(context.Background(), &out, &errBuf, path, false,
		time.Second, time.Second, outputJSON)
	if err != nil {
		t.Fatalf("runStatus: %v (stderr=%s)", err, errBuf.String())
	}

	var rows []upspoll.Row
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, out.String())
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].Status != "OL" {
		t.Errorf("status = %q, want OL", rows[0].Status)
	}
	if rows[0].Error != "" {
		t.Errorf("unexpected error: %s", rows[0].Error)
	}
}

func TestRunStatusSkipsNonServerHosts(t *testing.T) {
	path := writeTempInventory(t, `hosts:
  - name: workstation
    address: 10.0.0.5
    user: admin
    roles: [nut-client]
`)
	var out, errBuf bytes.Buffer
	err := runStatus(context.Background(), &out, &errBuf, path, false,
		time.Second, time.Second, outputJSON)
	if err != nil {
		t.Fatalf("runStatus: %v (stderr=%s)", err, errBuf.String())
	}
	trimmed := strings.TrimSpace(out.String())
	if trimmed != "null" && trimmed != "[]" {
		t.Errorf("expected empty JSON array/null, got %q", trimmed)
	}
}

func writeTempInventory(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "homelab-nut.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	return path
}

// Compile-time guard so the io import stays live even if rendering tests
// don't directly use a Writer interface assertion.
var _ io.Writer = io.Discard
