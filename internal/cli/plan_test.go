package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/orchestrator"
)

func TestPrintPlanResultCountsFailedHosts(t *testing.T) {
	res := &orchestrator.Result{Hosts: []*orchestrator.HostResult{
		{Host: &inventory.Host{Name: "a"}, Errors: []error{errors.New("ssh: no auth available")}},
		{Host: &inventory.Host{Name: "b"}},
	}}
	var buf bytes.Buffer
	printPlanResult(&buf, res)
	got := buf.String()
	if strings.Contains(got, "no changes") {
		t.Errorf("plan claims no changes with an unreachable host:\n%s", got)
	}
	if !strings.Contains(got, "Plan: 1 of 2 host(s) failed") {
		t.Errorf("summary should count the failed host:\n%s", got)
	}
}
