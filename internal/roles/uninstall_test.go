package roles

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/etc/nut":         "'/etc/nut'",
		"plain":            "'plain'",
		"with space":       "'with space'",
		"it's":             `'it'\''s'`,
		"/home/admin/x.sh": "'/home/admin/x.sh'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildRemovalScript(t *testing.T) {
	script := buildRemovalScript(
		[]string{"nut-exporter.service"},
		[]string{"/usr/local/bin/nut_exporter", "/etc/default/nut-exporter"},
		[]string{"nut"},
	)
	// Units: stop + disable + rm the unit file + a daemon-reload at the end.
	for _, want := range []string{
		"systemctl stop 'nut-exporter.service'",
		"systemctl disable 'nut-exporter.service'",
		"rm -f '/etc/systemd/system/nut-exporter.service'",
		`echo "removed: unit nut-exporter.service"`,
		"systemctl daemon-reload",
		// Files: existence-gated rm with removed/absent markers.
		"if [ -e '/usr/local/bin/nut_exporter' ]; then rm -rf '/usr/local/bin/nut_exporter'",
		`echo "removed: /etc/default/nut-exporter"`,
		// Packages: dpkg-query gate + apt-get purge.
		"dpkg-query -W -f='${Status}' 'nut'",
		"apt-get purge -y 'nut'",
		`echo "removed: package nut"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q\n---\n%s", want, script)
		}
	}
}

func TestBuildRemovalScript_NoUnitsNoDaemonReload(t *testing.T) {
	// With no units, there should be no daemon-reload tail.
	script := buildRemovalScript(nil, []string{"/etc/sudoers.d/ups-shutdown"}, nil)
	if strings.Contains(script, "daemon-reload") {
		t.Errorf("no units listed, but script has daemon-reload:\n%s", script)
	}
}

func TestParseRemoval(t *testing.T) {
	output := strings.Join([]string{
		"some apt chatter that should be ignored",
		"removed: unit nut-exporter.service",
		"removed: /usr/local/bin/nut_exporter",
		"absent: /etc/default/nut-exporter",
		"error: package nut purge failed",
		"Removing nut (2.7.4) ...", // capital R — not a marker
	}, "\n")

	rem, errs := parseRemoval("exporter", output)
	if rem.Role != "exporter" {
		t.Errorf("role = %q, want exporter", rem.Role)
	}
	wantRemoved := []string{"unit nut-exporter.service", "/usr/local/bin/nut_exporter"}
	if strings.Join(rem.Removed, "|") != strings.Join(wantRemoved, "|") {
		t.Errorf("removed = %v, want %v", rem.Removed, wantRemoved)
	}
	if strings.Join(rem.Skipped, "|") != "/etc/default/nut-exporter" {
		t.Errorf("skipped = %v", rem.Skipped)
	}
	if len(errs) != 1 || errs[0] != "package nut purge failed" {
		t.Errorf("errs = %v, want one purge failure", errs)
	}
}

func TestRemoveArtifacts_NilConn(t *testing.T) {
	_, err := removeArtifacts(context.TODO(), nil, "exporter", io.Discard, nil, []string{"/x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "nil connection") {
		t.Errorf("want nil-connection error, got: %v", err)
	}
}

func TestNutServerUninstall_PurgeGate(t *testing.T) {
	h := &inventory.Host{Name: "pi", Roles: []inventory.Role{inventory.RoleNUTServer}}

	// Without PurgeNUT it's a no-op that reports what the flag would remove —
	// and never touches the (nil) connection.
	rem, err := nutServer{}.Uninstall(context.TODO(), nil, h, UninstallParams{PurgeNUT: false}, io.Discard)
	if err != nil {
		t.Fatalf("non-purge uninstall should not error, got: %v", err)
	}
	if len(rem.Removed) != 0 {
		t.Errorf("non-purge should remove nothing, got: %v", rem.Removed)
	}
	if len(rem.Skipped) == 0 || !strings.Contains(rem.Skipped[0], "--purge-nut") {
		t.Errorf("non-purge should hint at --purge-nut, got: %v", rem.Skipped)
	}

	// With PurgeNUT and a nil conn, it tries to run and fails on the conn.
	_, purgeErr := (nutServer{}).Uninstall(context.TODO(), nil, h, UninstallParams{PurgeNUT: true}, io.Discard)
	if purgeErr == nil {
		t.Error("purge uninstall with nil conn should error")
	}
}

func TestNutClientUninstall_PurgeGate(t *testing.T) {
	h := &inventory.Host{Name: "ws", Roles: []inventory.Role{inventory.RoleNUTClient}}
	rem, err := nutClient{}.Uninstall(context.TODO(), nil, h, UninstallParams{PurgeNUT: false}, io.Discard)
	if err != nil {
		t.Fatalf("non-purge uninstall should not error, got: %v", err)
	}
	if len(rem.Removed) != 0 || len(rem.Skipped) == 0 {
		t.Errorf("non-purge nut-client should skip the package, got removed=%v skipped=%v", rem.Removed, rem.Skipped)
	}
}

func TestShutdownTargetUninstall_ScriptVsInline(t *testing.T) {
	// The Uninstall file list keys off resolvedMode: script mode removes the
	// deployed ~/shutdown.sh plus the sudoers rule; inline mode only the rule.
	scriptHost := &inventory.Host{
		Name: "nas", User: "admin", Roles: []inventory.Role{inventory.RoleShutdownTarget},
		Shutdown: &inventory.Shutdown{Command: "~/shutdown.sh"},
	}
	inlineHost := &inventory.Host{
		Name: "udm", User: "root", Roles: []inventory.Role{inventory.RoleShutdownTarget},
		Shutdown: &inventory.Shutdown{Command: "poweroff"},
	}
	r := shutdownTarget{}
	if got := r.resolvedMode(scriptHost); got != "script" {
		t.Errorf("script host mode = %q, want script", got)
	}
	if got := r.resolvedMode(inlineHost); got != "inline" {
		t.Errorf("inline host mode = %q, want inline", got)
	}
}
