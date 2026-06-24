package inventory

import (
	"errors"
	"strings"
	"testing"
)

// pickFirstIssue returns the first issue whose path matches needle, or
// fails the test if none was found. Helps tests stay readable.
func pickFirstIssue(t *testing.T, err error, needle string) FieldError {
	t.Helper()
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	for _, iss := range vErr.Issues {
		if strings.Contains(iss.Path, needle) {
			return iss
		}
	}
	t.Fatalf("no issue with path containing %q; issues=%v", needle, vErr.Issues)
	return FieldError{}
}

func TestValidate_NoHosts(t *testing.T) {
	inv := &Inventory{}
	err := inv.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	_ = pickFirstIssue(t, err, "hosts")
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	inv := &Inventory{Hosts: []Host{{Roles: []Role{RoleNUTClient}}}}
	err := inv.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"hosts[0].name", "hosts[0].address", "hosts[0].user"} {
		_ = pickFirstIssue(t, err, want)
	}
}

func TestValidate_UnknownRole(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "a", Address: "192.0.2.1", User: "u",
			Roles: []Role{"scheduler"},
		}},
	}
	err := inv.Validate()
	iss := pickFirstIssue(t, err, "roles[0]")
	if !strings.Contains(iss.Message, "unknown role") {
		t.Errorf("message = %q, want substring 'unknown role'", iss.Message)
	}
}

func TestValidate_DuplicateHostName(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{
			{Name: "dupe", Address: "192.0.2.1", User: "u", Roles: []Role{RoleNUTClient}},
			{Name: "dupe", Address: "192.0.2.2", User: "u", Roles: []Role{RoleNUTClient}},
		},
	}
	err := inv.Validate()
	iss := pickFirstIssue(t, err, "hosts[1].name")
	if !strings.Contains(iss.Message, "duplicate") {
		t.Errorf("message = %q, want substring 'duplicate'", iss.Message)
	}
}

func TestValidate_NUTServerNeedsUPS(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "pi", Address: "192.0.2.1", User: "pi",
			Roles: []Role{RoleNUTServer},
		}},
	}
	err := inv.Validate()
	_ = pickFirstIssue(t, err, "hosts[0].ups")
}

func TestValidate_ShutdownDaemonRange(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "a", Address: "192.0.2.1", User: "u",
			Roles: []Role{RoleShutdownDaemon},
		}},
		ShutdownDaemon: &ShutdownDaemon{Threshold: 150, PollInterval: -1},
	}
	err := inv.Validate()
	_ = pickFirstIssue(t, err, "shutdown_daemon.threshold")
	_ = pickFirstIssue(t, err, "shutdown_daemon.poll_interval")
}

func TestValidate_OrphanShutdownDaemon(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "a", Address: "192.0.2.1", User: "u",
			Roles: []Role{RoleNUTClient},
		}},
		ShutdownDaemon: &ShutdownDaemon{Threshold: 50, PollInterval: 30},
	}
	err := inv.Validate()
	iss := pickFirstIssue(t, err, "shutdown_daemon")
	if !strings.Contains(iss.Message, "no host has role 'shutdown-daemon'") {
		t.Errorf("message = %q", iss.Message)
	}
}

func TestValidate_NegativeShutdownDelay(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "ws", Address: "192.0.2.1", User: "u",
			Roles:    []Role{RoleShutdownTarget},
			Shutdown: &Shutdown{Command: "poweroff", Delay: -5},
		}},
	}
	err := inv.Validate()
	iss := pickFirstIssue(t, err, "hosts[0].shutdown.delay")
	if !strings.Contains(iss.Message, "zero or positive") {
		t.Errorf("message = %q", iss.Message)
	}
}

func TestValidate_AcceptsShutdownDelay(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "udm", Address: "10.0.10.1", User: "root",
			Roles:    []Role{RoleShutdownTarget},
			Shutdown: &Shutdown{Command: "poweroff", Delay: 60},
		}},
	}
	if err := inv.Validate(); err != nil {
		t.Fatalf("a non-negative shutdown delay should validate, got: %v", err)
	}
}

func TestValidate_PerTargetThresholdRange(t *testing.T) {
	for _, bad := range []int{-1, 100, 150} {
		inv := &Inventory{
			Hosts: []Host{{
				Name: "nas", Address: "10.0.10.125", User: "root",
				Roles:    []Role{RoleShutdownTarget},
				Shutdown: &Shutdown{Command: "poweroff", Threshold: bad},
			}},
		}
		err := inv.Validate()
		iss := pickFirstIssue(t, err, "hosts[0].shutdown.threshold")
		if !strings.Contains(iss.Message, "out of range") {
			t.Errorf("threshold %d: message = %q, want 'out of range'", bad, iss.Message)
		}
	}
}

func TestValidate_AcceptsPerTargetThreshold(t *testing.T) {
	// A set, in-range threshold validates; an omitted (0) threshold inherits
	// the daemon default and must not trip the range check.
	for _, ok := range []int{0, 1, 60, 99} {
		inv := &Inventory{
			Hosts: []Host{{
				Name: "nas", Address: "10.0.10.125", User: "root",
				Roles:    []Role{RoleShutdownTarget},
				Shutdown: &Shutdown{Command: "poweroff", Threshold: ok},
			}},
		}
		if err := inv.Validate(); err != nil {
			t.Fatalf("threshold %d should validate, got: %v", ok, err)
		}
	}
}

func TestValidate_PerHostShutdownDaemonRange(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "a", Address: "192.0.2.1", User: "u",
			Roles:          []Role{RoleShutdownDaemon},
			ShutdownDaemon: &ShutdownDaemon{Threshold: 150, PollInterval: -1},
		}},
	}
	err := inv.Validate()
	_ = pickFirstIssue(t, err, "hosts[0].shutdown_daemon.threshold")
	_ = pickFirstIssue(t, err, "hosts[0].shutdown_daemon.poll_interval")
}

func TestValidate_PerHostShutdownDaemonWithoutRole(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{{
			Name: "a", Address: "192.0.2.1", User: "u",
			Roles:          []Role{RoleNUTClient},
			ShutdownDaemon: &ShutdownDaemon{Threshold: 40, PollInterval: 20},
		}},
	}
	err := inv.Validate()
	iss := pickFirstIssue(t, err, "hosts[0].shutdown_daemon")
	if !strings.Contains(iss.Message, "lacks role 'shutdown-daemon'") {
		t.Errorf("message = %q", iss.Message)
	}
}

func TestValidate_AcceptsPerHostShutdownDaemon(t *testing.T) {
	inv := &Inventory{
		Hosts: []Host{
			{
				Name: "pi", Address: "192.0.2.1", User: "pi",
				Roles:          []Role{RoleShutdownDaemon},
				ShutdownDaemon: &ShutdownDaemon{Threshold: 40, PollInterval: 20},
			},
			{
				Name: "ws", Address: "192.0.2.2", User: "u",
				Roles:    []Role{RoleShutdownTarget},
				Shutdown: &Shutdown{Command: "poweroff"},
			},
		},
	}
	if err := inv.Validate(); err != nil {
		t.Fatalf("per-host daemon override should validate, got: %v", err)
	}
}

func TestValidate_AcceptsExampleInventory(t *testing.T) {
	// The same example.yaml we ship in examples/ should validate cleanly.
	yml := `
hosts:
  - name: pi-rack
    address: 192.0.2.10
    user: pi
    roles: [nut-server, exporter, shutdown-daemon]
    ups: { name: myups, driver: usbhid-ups }
    shutdown_daemon: { threshold: 50, poll_interval: 30, slack_webhook_env: SLACK_WEBHOOK }
  - name: workstation
    address: 192.0.2.20
    user: admin
    roles: [nut-client, shutdown-target]
    shutdown: { command: ~/shutdown.sh }
  - name: dream-machine
    address: 192.0.2.1
    user: admin
    roles: [shutdown-target]
    shutdown: { command: poweroff }
shutdown_daemon:
  threshold: 50
  poll_interval: 30
  slack_webhook_env: SLACK_WEBHOOK
`
	_, err := LoadReader(strings.NewReader(yml))
	if err != nil {
		t.Fatalf("example inventory should validate, got: %v", err)
	}
}
