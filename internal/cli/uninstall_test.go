package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/orchestrator"
	"github.com/rtorcato/homelab-nut/internal/roles"
)

// All of these exercise the guards that run *before* any SSH, so they're
// safe headless. The actual removal path needs a live host.

func TestRunUninstall_RefusesFleetWithoutAll(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runUninstall(strings.NewReader(""), &out, &errb, p, "", false, "all", false, false, 0, outputText)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "--all") {
		t.Errorf("stderr should explain --all, got: %q", errb.String())
	}
}

func TestRunUninstall_HostNotFound(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runUninstall(strings.NewReader(""), &out, &errb, p, "ghost", false, "all", false, false, 0, outputText)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "not found") {
		t.Errorf("stderr should say not found, got: %q", errb.String())
	}
}

func TestRunUninstall_BadRole(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runUninstall(strings.NewReader(""), &out, &errb, p, "", true, "scheduler", false, false, 0, outputText)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "unknown --role") {
		t.Errorf("stderr should reject the role, got: %q", errb.String())
	}
}

func TestRunUninstall_JSONRequiresAutoApprove(t *testing.T) {
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runUninstall(strings.NewReader(""), &out, &errb, p, "", true, "all", false, false, 0, outputJSON)
	if err != errSilent {
		t.Fatalf("want errSilent, got %v", err)
	}
	if !strings.Contains(errb.String(), "--auto-approve") {
		t.Errorf("stderr should require --auto-approve, got: %q", errb.String())
	}
}

func TestRunUninstall_TextPreviewThenAbort(t *testing.T) {
	// Decline at the confirm prompt — nothing runs, exits cleanly.
	p := writeInv(t, twoHostInv)
	var out, errb bytes.Buffer
	err := runUninstall(strings.NewReader("n\n"), &out, &errb, p, "a", false, "all", false, false, 0, outputText)
	if err != nil {
		t.Fatalf("aborting should be a clean exit, got %v", err)
	}
	if !strings.Contains(out.String(), "About to uninstall:") {
		t.Errorf("expected a preview, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected an abort message, got: %q", out.String())
	}
}

func TestPrintUninstallPreview_RoleFilterAndPurgeNote(t *testing.T) {
	inv := &inventory.Inventory{Hosts: []inventory.Host{
		{Name: "pi", Roles: []inventory.Role{inventory.RoleNUTServer, inventory.RoleExporter}},
		{Name: "ws", Roles: []inventory.Role{inventory.RoleNUTClient}},
	}}
	targets := selectUninstallTargets(inv, "")
	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(targets))
	}

	var b bytes.Buffer
	printUninstallPreview(&b, targets, inventory.RoleExporter, false)
	got := b.String()
	// --role exporter: pi shows only exporter; ws (no exporter) shows none.
	if !strings.Contains(got, "pi — exporter") {
		t.Errorf("pi should list only exporter, got:\n%s", got)
	}
	if !strings.Contains(got, "ws — (no matching roles)") {
		t.Errorf("ws should match nothing under --role exporter, got:\n%s", got)
	}
	if !strings.Contains(got, "left in place") {
		t.Errorf("non-purge run should note NUT is left in place, got:\n%s", got)
	}

	b.Reset()
	printUninstallPreview(&b, targets, "", true)
	if !strings.Contains(b.String(), "--purge-nut") {
		t.Errorf("purge run should warn about --purge-nut, got:\n%s", b.String())
	}
}

func TestFlattenAndSummarise(t *testing.T) {
	res := &orchestrator.Result{Hosts: []*orchestrator.HostResult{
		{
			Host: &inventory.Host{Name: "pi"},
			Removals: []*roles.Removal{
				{Role: "exporter", Removed: []string{"unit nut-exporter.service", "/usr/local/bin/nut_exporter"}},
				{Role: "shutdown-daemon", Removed: []string{"unit ups-battery-shutdown.service"}, Skipped: []string{"/etc/ups-battery-shutdown.conf"}},
			},
		},
		{
			Host:     &inventory.Host{Name: "ws"},
			Removals: []*roles.Removal{{Role: "nut-client", Skipped: []string{"upstream nut-client package (pass --purge-nut to remove)"}}},
		},
	}}

	removed, failed := summariseUninstall(res)
	if removed != 3 {
		t.Errorf("removed count = %d, want 3", removed)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
	if res.NothingRemoved() {
		t.Error("NothingRemoved() should be false — pi removed items")
	}

	sum := buildUninstallSummary(res, 2*time.Second, removed, failed)
	if len(sum.Results) != 2 || sum.Removed != 3 || sum.Elapsed != "2s" {
		t.Errorf("summary mismatch: %+v", sum)
	}
	if len(sum.Results[0].Removed) != 3 || len(sum.Results[1].Skipped) != 1 {
		t.Errorf("flattened results wrong: %+v", sum.Results)
	}
}

func TestNothingRemoved_AllAbsent(t *testing.T) {
	res := &orchestrator.Result{Hosts: []*orchestrator.HostResult{
		{Host: &inventory.Host{Name: "pi"}, Removals: []*roles.Removal{
			{Role: "exporter", Skipped: []string{"unit nut-exporter.service"}},
		}},
	}}
	if !res.NothingRemoved() {
		t.Error("NothingRemoved() should be true when every artifact was absent")
	}
}
