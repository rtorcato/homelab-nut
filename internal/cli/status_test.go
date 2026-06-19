package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

// fakeNUTServer speaks just enough of the NUT protocol to satisfy
// pollHost: LIST UPS and LIST VAR. Anything else gets "ERR
// UNKNOWN-COMMAND" so test failures are loud rather than silent.
type fakeNUTServer struct {
	ln    net.Listener
	addr  string
	upses []string                     // raw "UPS <name> \"<desc>\"" body lines
	vars  map[string]map[string]string // ups name → var → value
}

func newFakeNUTServer(t *testing.T, upses []string, vars map[string]map[string]string) *fakeNUTServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeNUTServer{ln: ln, addr: ln.Addr().String(), upses: upses, vars: vars}
	go f.accept()
	t.Cleanup(func() { _ = ln.Close() })
	return f
}

func (f *fakeNUTServer) accept() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(c)
	}
}

func (f *fakeNUTServer) handle(c net.Conn) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "LIST UPS":
			fmt.Fprint(c, "BEGIN LIST UPS\n")
			for _, u := range f.upses {
				fmt.Fprint(c, u+"\n")
			}
			fmt.Fprint(c, "END LIST UPS\n")
		case strings.HasPrefix(line, "LIST VAR "):
			ups := strings.TrimPrefix(line, "LIST VAR ")
			vs, ok := f.vars[ups]
			if !ok {
				fmt.Fprint(c, "ERR UNKNOWN-UPS\n")
				continue
			}
			fmt.Fprintf(c, "BEGIN LIST VAR %s\n", ups)
			for k, v := range vs {
				fmt.Fprintf(c, "VAR %s %s %q\n", ups, k, v)
			}
			fmt.Fprintf(c, "END LIST VAR %s\n", ups)
		default:
			fmt.Fprint(c, "ERR UNKNOWN-COMMAND\n")
		}
	}
}

func TestPollHostHappyPath(t *testing.T) {
	f := newFakeNUTServer(t,
		[]string{`UPS myups "Office UPS"`},
		map[string]map[string]string{
			"myups": {
				"ups.status":      "OL",
				"battery.charge":  "85",
				"battery.runtime": "1800",
				"ups.load":        "12",
			},
		},
	)
	h := &inventory.Host{Name: "office", Address: f.addr}
	row := pollHost(context.Background(), h, time.Second)

	if row.Error != "" {
		t.Fatalf("unexpected error: %s", row.Error)
	}
	if row.UPS != "myups" {
		t.Errorf("ups = %q, want myups", row.UPS)
	}
	if row.Status != "OL" {
		t.Errorf("status = %q, want OL", row.Status)
	}
	if row.BatteryCharge == nil || *row.BatteryCharge != 85 {
		t.Errorf("battery_charge = %v, want 85", row.BatteryCharge)
	}
	if row.BatteryRuntime == nil || *row.BatteryRuntime != 1800 {
		t.Errorf("battery_runtime = %v, want 1800", row.BatteryRuntime)
	}
	if row.Load == nil || *row.Load != 12 {
		t.Errorf("load = %v, want 12", row.Load)
	}
}

func TestPollHostNoUPSReported(t *testing.T) {
	f := newFakeNUTServer(t, nil, nil)
	h := &inventory.Host{Name: "empty", Address: f.addr}
	row := pollHost(context.Background(), h, time.Second)
	if !strings.Contains(row.Error, "no UPS reported") {
		t.Errorf("error = %q, want substring 'no UPS reported'", row.Error)
	}
}

func TestPollHostDialFailure(t *testing.T) {
	// Listen + close so the port is guaranteed-not-accepting. Race with
	// the OS reissuing the port is fine — we only need a refused connect.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	h := &inventory.Host{Name: "down", Address: addr}
	row := pollHost(context.Background(), h, 200*time.Millisecond)
	if row.Error == "" {
		t.Fatal("expected error on closed port")
	}
}

func TestPollHostMissingVars(t *testing.T) {
	// Server reports a UPS but only the status — no battery / load.
	// Expected: row populates Status, leaves numeric fields nil.
	f := newFakeNUTServer(t,
		[]string{`UPS myups "Bare UPS"`},
		map[string]map[string]string{
			"myups": {"ups.status": "OB LB"},
		},
	)
	h := &inventory.Host{Name: "bare", Address: f.addr}
	row := pollHost(context.Background(), h, time.Second)
	if row.Status != "OB LB" {
		t.Errorf("status = %q, want %q", row.Status, "OB LB")
	}
	if row.BatteryCharge != nil || row.BatteryRuntime != nil || row.Load != nil {
		t.Errorf("expected nil numerics, got charge=%v runtime=%v load=%v",
			row.BatteryCharge, row.BatteryRuntime, row.Load)
	}
}

func TestStatusRowJSONOmitsUnknownFields(t *testing.T) {
	row := statusRow{Host: "h", Address: "1.2.3.4", Error: "timeout"}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, banned := range []string{"battery_charge", "battery_runtime", "load", "ups", "status"} {
		if strings.Contains(got, banned) {
			t.Errorf("JSON unexpectedly includes %q: %s", banned, got)
		}
	}
	for _, required := range []string{`"host":"h"`, `"address":"1.2.3.4"`, `"error":"timeout"`} {
		if !strings.Contains(got, required) {
			t.Errorf("JSON missing %q: %s", required, got)
		}
	}
}

func TestPrintStatusTableGolden(t *testing.T) {
	charge := 85.0
	load := 12.0
	runtime := 1800
	rows := []statusRow{
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
	f := newFakeNUTServer(t,
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
`, f.addr))

	var out, errBuf bytes.Buffer
	err := runStatus(context.Background(), &out, &errBuf, path, false,
		time.Second, time.Second, outputJSON)
	if err != nil {
		t.Fatalf("runStatus: %v (stderr=%s)", err, errBuf.String())
	}

	var rows []statusRow
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
	// Inventory has a single nut-client host (no nut-server). Status
	// should emit an empty JSON array, not error out.
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

// Compile-time guard that io.Discard satisfies io.Writer — keeps the
// import live when the file is built without a test that uses it.
var _ io.Writer = io.Discard
