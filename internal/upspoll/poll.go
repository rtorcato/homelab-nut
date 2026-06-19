// Package upspoll polls NUT servers across an inventory and returns a
// uniform per-host snapshot consumed by both the CLI `status` command
// and the TUI Dashboard. Keeping it here (rather than in internal/cli)
// keeps the dependency direction sane: tui imports upspoll, cli imports
// upspoll — neither imports the other.
package upspoll

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ups"
)

// Row is one polled host's state. Fields without a reading are left at
// zero / nil so consumers can distinguish "unknown" from "zero" — the
// JSON encoder relies on `omitempty` to drop missing values.
type Row struct {
	Host           string   `json:"host"`
	Address        string   `json:"address"`
	UPS            string   `json:"ups,omitempty"`
	Status         string   `json:"status,omitempty"`
	BatteryCharge  *float64 `json:"battery_charge,omitempty"`
	BatteryRuntime *int     `json:"battery_runtime,omitempty"`
	Load           *float64 `json:"load,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// Poll concurrently polls every host and returns the results in the
// same order as the input slice. A nil context is treated as
// context.Background.
func Poll(ctx context.Context, hosts []*inventory.Host, timeout time.Duration) []Row {
	if ctx == nil {
		ctx = context.Background()
	}
	out := make([]Row, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(i int, h *inventory.Host) {
			defer wg.Done()
			out[i] = PollHost(ctx, h, timeout)
		}(i, h)
	}
	wg.Wait()
	return out
}

// PollHost connects to one host, lists its UPSes, reads vars for the
// first one, and returns a Row. Failures populate Row.Error and leave
// numeric fields nil.
func PollHost(ctx context.Context, h *inventory.Host, timeout time.Duration) Row {
	row := Row{Host: h.Name, Address: h.Address}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client, err := ups.Dial(dialCtx, h.Address, ups.DialOptions{
		Timeout:  timeout,
		Deadline: timeout,
	})
	if err != nil {
		row.Error = sanitizeErr(err)
		return row
	}
	defer func() { _ = client.Close() }()

	list, err := client.ListUPS()
	if err != nil {
		row.Error = sanitizeErr(err)
		return row
	}
	if len(list) == 0 {
		row.Error = "no UPS reported by server"
		return row
	}
	// Multi-UPS hosts pick the first; future work expands to one Row per UPS.
	upsName := list[0].Name
	row.UPS = upsName

	vars, err := client.ListVar(upsName)
	if err != nil {
		row.Error = sanitizeErr(err)
		return row
	}
	row.Status = vars["ups.status"]
	row.BatteryCharge = parseFloatPtr(vars["battery.charge"])
	row.BatteryRuntime = parseIntPtr(vars["battery.runtime"])
	row.Load = parseFloatPtr(vars["ups.load"])
	return row
}

// sanitizeErr collapses a deadline-exceeded into a short token the
// table view can render in narrow columns.
func sanitizeErr(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return err.Error()
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseIntPtr(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}
