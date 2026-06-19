package upspoll_test

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/upspoll"
	"github.com/rtorcato/homelab-nut/internal/upspoll/upspolltest"
)

func TestPollHostHappyPath(t *testing.T) {
	s := upspolltest.New(t,
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
	h := &inventory.Host{Name: "office", Address: s.Addr()}
	row := upspoll.PollHost(context.Background(), h, time.Second)

	if row.Error != "" {
		t.Fatalf("unexpected error: %s", row.Error)
	}
	if row.UPS != "myups" || row.Status != "OL" {
		t.Errorf("ups=%q status=%q, want myups/OL", row.UPS, row.Status)
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
	s := upspolltest.New(t, nil, nil)
	row := upspoll.PollHost(context.Background(),
		&inventory.Host{Name: "empty", Address: s.Addr()}, time.Second)
	if !strings.Contains(row.Error, "no UPS reported") {
		t.Errorf("error = %q, want substring 'no UPS reported'", row.Error)
	}
}

func TestPollHostDialFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	row := upspoll.PollHost(context.Background(),
		&inventory.Host{Name: "down", Address: addr}, 200*time.Millisecond)
	if row.Error == "" {
		t.Fatal("expected error on closed port")
	}
}

func TestPollHostMissingVars(t *testing.T) {
	s := upspolltest.New(t,
		[]string{`UPS myups "Bare UPS"`},
		map[string]map[string]string{"myups": {"ups.status": "OB LB"}},
	)
	row := upspoll.PollHost(context.Background(),
		&inventory.Host{Name: "bare", Address: s.Addr()}, time.Second)
	if row.Status != "OB LB" {
		t.Errorf("status = %q, want OB LB", row.Status)
	}
	if row.BatteryCharge != nil || row.BatteryRuntime != nil || row.Load != nil {
		t.Errorf("expected nil numerics, got charge=%v runtime=%v load=%v",
			row.BatteryCharge, row.BatteryRuntime, row.Load)
	}
}

func TestRowJSONOmitsUnknownFields(t *testing.T) {
	row := upspoll.Row{Host: "h", Address: "1.2.3.4", Error: "timeout"}
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

func TestPollPreservesHostOrder(t *testing.T) {
	// Two servers so we can detect ordering. Both happy.
	s1 := upspolltest.New(t,
		[]string{`UPS u "u1"`},
		map[string]map[string]string{"u": {"ups.status": "OL"}})
	s2 := upspolltest.New(t,
		[]string{`UPS u "u2"`},
		map[string]map[string]string{"u": {"ups.status": "OB"}})

	hosts := []*inventory.Host{
		{Name: "a", Address: s1.Addr()},
		{Name: "b", Address: s2.Addr()},
	}
	rows := upspoll.Poll(context.Background(), hosts, time.Second)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Host != "a" || rows[1].Host != "b" {
		t.Errorf("order = [%s %s], want [a b]", rows[0].Host, rows[1].Host)
	}
}
