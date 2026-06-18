package ssh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewConfigDefaults(t *testing.T) {
	c := NewConfig()
	if !c.StrictHostKey {
		t.Error("NewConfig().StrictHostKey = false, want true")
	}
	if !c.UseAgent {
		t.Error("NewConfig().UseAgent = false, want true")
	}
	if c.ConnectTimeout != 15*time.Second {
		t.Errorf("NewConfig().ConnectTimeout = %v, want 15s", c.ConnectTimeout)
	}
}

func TestApplyDefaultsFillsConnectTimeout(t *testing.T) {
	c := Config{}.applyDefaults()
	if c.ConnectTimeout == 0 {
		t.Error("applyDefaults left ConnectTimeout zero")
	}
}

func TestApplyDefaultsLeavesExplicitValuesAlone(t *testing.T) {
	c := Config{ConnectTimeout: 3 * time.Second}.applyDefaults()
	if c.ConnectTimeout != 3*time.Second {
		t.Errorf("applyDefaults overrode explicit ConnectTimeout: got %v", c.ConnectTimeout)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"/etc/ssh", "/etc/ssh", false},
		{"~", home, false},
		{"~/", home, false},
		{"~/.ssh/id_ed25519", filepath.Join(home, ".ssh/id_ed25519"), false},
		{"~bob/.ssh/id_rsa", "", true}, // ~user not supported
	}
	for _, tc := range cases {
		got, err := expandHome(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("expandHome(%q) = %q nil, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("expandHome(%q) returned err: %v", tc.in, err)
			continue
		}
		// "~/" expands to home (filepath.Join("", home) drops trailing slash).
		if tc.in == "~/" {
			if got != home {
				t.Errorf("expandHome(~/) = %q, want %q", got, home)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("expandHome(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveKeyPath_ExplicitWins(t *testing.T) {
	tmp := t.TempDir()
	keyFile := filepath.Join(tmp, "id_custom")
	if err := os.WriteFile(keyFile, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := Config{KeyPath: keyFile}
	got, err := c.resolveKeyPath()
	if err != nil {
		t.Fatalf("resolveKeyPath: %v", err)
	}
	if got != keyFile {
		t.Errorf("resolveKeyPath = %q, want %q", got, keyFile)
	}
}

func TestResolveKeyPath_ErrNoKeyWhenNothingExists(t *testing.T) {
	// Point the explicit path at a definitely-missing file so we get a
	// deterministic miss without touching the user's real ~/.ssh.
	c := Config{KeyPath: filepath.Join(t.TempDir(), "no-such-key")}
	_, err := c.resolveKeyPath()
	if !errors.Is(err, errNoKey) {
		t.Errorf("resolveKeyPath err = %v, want errNoKey", err)
	}
}

func TestResolveKnownHosts_DefaultsToTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	c := Config{}
	got, err := c.resolveKnownHosts()
	if err != nil {
		t.Fatalf("resolveKnownHosts: %v", err)
	}
	want := filepath.Join(home, ".ssh/known_hosts")
	if got != want {
		t.Errorf("resolveKnownHosts = %q, want %q", got, want)
	}
}

func TestResolveKnownHosts_Override(t *testing.T) {
	c := Config{KnownHostsPath: "/etc/ssh/ssh_known_hosts"}
	got, err := c.resolveKnownHosts()
	if err != nil {
		t.Fatalf("resolveKnownHosts: %v", err)
	}
	if got != "/etc/ssh/ssh_known_hosts" {
		t.Errorf("resolveKnownHosts = %q, want explicit override", got)
	}
}

func TestJoinHostPort(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"pi.local", "pi.local:22"},
		{"192.0.2.10", "192.0.2.10:22"},
		{"pi.local:2222", "pi.local:2222"},        // already has port → leave
		{"192.0.2.10:2222", "192.0.2.10:2222"},
	}
	for _, tc := range cases {
		got := joinHostPort(tc.in, 22)
		if got != tc.want {
			t.Errorf("joinHostPort(%q, 22) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestErrNoKeyMessage(t *testing.T) {
	// Light check that the error message points at the conventional fix.
	msg := errNoKey.Error()
	if !strings.Contains(msg, "agent") || !strings.Contains(msg, "key") {
		t.Errorf("errNoKey message %q should mention both agent and key", msg)
	}
}
