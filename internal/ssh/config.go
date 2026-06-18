// Package ssh provides a small SSH executor used by homelab-nut roles
// to run commands on inventory hosts.
//
// The package is intentionally narrow: connection caching per host,
// key-or-agent auth (no passwords), strict known_hosts by default,
// and Run/Stream methods for one-shot vs. streamed command execution.
// Concurrency control sits above this layer (in the role runner).
package ssh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config controls how the Executor opens connections.
//
// All fields have sensible zero-value defaults — passing a zero Config
// to NewExecutor uses the ssh-agent if available, then falls back to
// ~/.ssh/id_ed25519, then ~/.ssh/id_rsa, with strict known_hosts and
// a 15-second connect timeout.
type Config struct {
	// KeyPath is an explicit private key path. Empty means try the default
	// key locations (~/.ssh/id_ed25519, ~/.ssh/id_rsa).
	KeyPath string

	// KnownHostsPath is the OpenSSH known_hosts file. Empty means
	// ~/.ssh/known_hosts.
	KnownHostsPath string

	// StrictHostKey rejects unknown host keys. Defaults to true — set to
	// false for `homelab-nut` flows that explicitly accept fingerprints.
	StrictHostKey bool

	// UseAgent tries ssh-agent first when SSH_AUTH_SOCK is set.
	// Defaults to true.
	UseAgent bool

	// ConnectTimeout caps the SSH handshake. Defaults to 15 seconds.
	ConnectTimeout time.Duration

	// CommandTimeout caps a single command. Zero means no per-command
	// limit (the caller's context still applies).
	CommandTimeout time.Duration
}

// defaultKeyCandidates is the order the Executor checks when KeyPath
// is unset. Edit here, not at every call site.
var defaultKeyCandidates = []string{
	"~/.ssh/id_ed25519",
	"~/.ssh/id_rsa",
}

// applyDefaults returns a Config with zero fields populated. The
// returned copy never mutates the caller's value.
func (c Config) applyDefaults() Config {
	out := c

	if out.ConnectTimeout == 0 {
		out.ConnectTimeout = 15 * time.Second
	}

	// Bool defaults: StrictHostKey and UseAgent should default to true,
	// but we can't distinguish "unset" from "false" on bool. Use a
	// constructor (NewConfig) for explicit defaults — see below.
	return out
}

// NewConfig returns a Config with the recommended defaults
// (StrictHostKey=true, UseAgent=true, 15s connect, no command timeout).
// Prefer this over the zero value when you want defaults.
func NewConfig() Config {
	return Config{
		StrictHostKey:  true,
		UseAgent:       true,
		ConnectTimeout: 15 * time.Second,
	}
}

// expandHome expands a leading "~/" to $HOME. Used to keep key paths
// readable in config files.
func expandHome(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if p[0] != '~' {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand ~: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	if len(p) > 1 && (p[1] == '/' || p[1] == os.PathSeparator) {
		return filepath.Join(home, p[2:]), nil
	}
	// "~user" form — not supported; surface clearly.
	return "", fmt.Errorf("~user paths not supported: %q", p)
}

// resolveKeyPath returns the first usable key path: explicit KeyPath
// first, then the candidate list. Returns errNoKey if nothing is
// readable.
func (c Config) resolveKeyPath() (string, error) {
	candidates := defaultKeyCandidates
	if c.KeyPath != "" {
		candidates = []string{c.KeyPath}
	}
	for _, raw := range candidates {
		p, err := expandHome(raw)
		if err != nil {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errNoKey
}

// resolveKnownHosts returns the known_hosts path, expanding ~/ if
// present. Empty input falls back to ~/.ssh/known_hosts.
func (c Config) resolveKnownHosts() (string, error) {
	p := c.KnownHostsPath
	if p == "" {
		p = "~/.ssh/known_hosts"
	}
	return expandHome(p)
}

// errNoKey signals that no usable private key was found in any of the
// configured paths. Callers should fall back to the agent (if enabled)
// or surface a "no auth available" error.
var errNoKey = errors.New("no SSH private key found (tried agent + default key paths)")
