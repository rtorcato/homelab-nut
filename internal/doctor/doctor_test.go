package doctor

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func host(roles ...inventory.Role) *inventory.Host {
	return &inventory.Host{Name: "h", Roles: roles}
}

func failed(cs []Check) map[string]string {
	out := map[string]string{}
	for _, c := range cs {
		if !c.OK {
			out[c.Name] = c.Detail
		}
	}
	return out
}

const healthyClient = `MODE|netclient
UPSMON|active
LIST|myups@10.0.0.2|0
UPS|myups@10.0.0.2|myups
DAEMON|noconf
`

func TestHealthyClient(t *testing.T) {
	cs := evaluate(host(inventory.RoleNUTClient), parseFacts(healthyClient))
	if f := failed(cs); len(f) != 0 || len(cs) != 3 {
		t.Fatalf("want 3 passing checks, got %+v", cs)
	}
}

// The incident behind the command: every service "active", server gone.
func TestDecommissionedServer(t *testing.T) {
	out := strings.Replace(healthyClient, "LIST|myups@10.0.0.2|0\nUPS|myups@10.0.0.2|myups\n", "LIST|myups@10.0.0.2|1\n", 1)
	f := failed(evaluate(host(inventory.RoleNUTClient), parseFacts(out)))
	if _, ok := f["monitor myups@10.0.0.2"]; !ok || len(f) != 1 {
		t.Fatalf("want only the monitor check to fail, got %v", f)
	}
}

func TestUPSNameMissingOnServer(t *testing.T) {
	out := strings.Replace(healthyClient, "|myups\n", "|other\n", 1)
	f := failed(evaluate(host(inventory.RoleNUTClient), parseFacts(out)))
	if d := f["monitor myups@10.0.0.2"]; !strings.Contains(d, "other") {
		t.Fatalf("want a name mismatch listing the server's UPSes, got %v", f)
	}
}

func TestModeMustMatchRoles(t *testing.T) {
	for _, tc := range []struct {
		roles []inventory.Role
		mode  string
		fail  bool
	}{
		{[]inventory.Role{inventory.RoleNUTServer}, "netclient", true},
		{[]inventory.Role{inventory.RoleNUTServer}, "netserver", false},
		{[]inventory.Role{inventory.RoleNUTServer, inventory.RoleNUTClient}, "standalone", false},
		{[]inventory.Role{inventory.RoleNUTClient}, "netserver", true},
		{[]inventory.Role{inventory.RoleNUTClient}, "none", true},
	} {
		f := parseFacts("MODE|" + tc.mode + "\nUPSMON|active\nLIST|u@s|0\nUPS|u@s|u\n")
		if _, bad := failed(evaluate(host(tc.roles...), f))["nut.conf mode"]; bad != tc.fail {
			t.Errorf("roles %v mode %q: failed=%v, want %v", tc.roles, tc.mode, bad, tc.fail)
		}
	}
}

func TestNoMonitorLines(t *testing.T) {
	for _, out := range []string{"MODE|netclient\nUPSMON|active\n", "MODE|netclient\nUPSMON|active\nUPSMONCONF|unreadable\n"} {
		if _, ok := failed(evaluate(host(inventory.RoleNUTClient), parseFacts(out)))["monitor targets"]; !ok {
			t.Errorf("want monitor targets to fail for %q", out)
		}
	}
}

func TestShutdownDaemon(t *testing.T) {
	h := host(inventory.RoleShutdownDaemon)
	ok := evaluate(h, parseFacts("DAEMON|active\nDAEMONREAD|myups@localhost|0|OL\n"))
	if len(failed(ok)) != 0 || len(ok) != 2 {
		t.Fatalf("healthy daemon: %+v", ok)
	}
	// Service active, UPS name no longer resolves: the 860-failures case.
	f := failed(evaluate(h, parseFacts("DAEMON|active\nDAEMONREAD|gone@localhost|1|\n")))
	if _, bad := f["daemon UPS read"]; !bad || len(f) != 1 {
		t.Fatalf("want only the read to fail, got %v", f)
	}
	if _, bad := failed(evaluate(h, parseFacts("DAEMON|noconf\n")))["shutdown daemon"]; !bad {
		t.Fatal("missing conf should fail")
	}
}

func TestHealthy(t *testing.T) {
	if (Result{Error: "ssh: refused"}).Healthy() {
		t.Error("probe error is not healthy")
	}
	if !(Result{Checks: []Check{{OK: true}}}).Healthy() {
		t.Error("all-ok should be healthy")
	}
	if (Result{Checks: []Check{{OK: true}, {OK: false}}}).Healthy() {
		t.Error("one failure is not healthy")
	}
}

func TestProbeSyntax(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	if out, err := exec.Command(sh, "-n", "-c", probeCmd).CombinedOutput(); err != nil {
		t.Fatalf("probe is not valid sh: %v\n%s", err, out)
	}
}
