package roles

import (
	"context"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	"github.com/rtorcato/homelab-nut/internal/ssh"
)

func TestResolveMonitorPassword_EnvWins(t *testing.T) {
	// When the env var is set it's used verbatim — no SSH, server ignored.
	t.Setenv(nutMonitorPasswordEnv, "from-env")
	got, err := resolveMonitorPassword(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-env" {
		t.Errorf("password = %q, want from-env", got)
	}
}

func TestResolveMonitorPassword_NoEnvNoServer(t *testing.T) {
	t.Setenv(nutMonitorPasswordEnv, "")
	_, err := resolveMonitorPassword(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), nutMonitorPasswordEnv) {
		t.Fatalf("want error mentioning %s, got %v", nutMonitorPasswordEnv, err)
	}
}

func TestResolveMonitorPassword_NoEnvNoSSHConfig(t *testing.T) {
	// A server is known but the context carries no SSH config (an ad-hoc
	// call outside the orchestrator) — fail fast without dialing.
	t.Setenv(nutMonitorPasswordEnv, "")
	server := &inventory.Host{Name: "pi", Address: "192.0.2.10", User: "pi"}
	_, err := resolveMonitorPassword(context.Background(), server)
	if err == nil || !strings.Contains(err.Error(), "no SSH config") {
		t.Fatalf("want 'no SSH config' error, got %v", err)
	}
}

func TestSSHConfigRoundTripsOnContext(t *testing.T) {
	cfg := ssh.NewConfig()
	cfg.ConnectTimeout = 7 // sentinel
	ctx := WithSSHConfig(context.Background(), cfg)
	got, ok := sshConfigFrom(ctx)
	if !ok {
		t.Fatal("sshConfigFrom: expected config present")
	}
	if got.ConnectTimeout != 7 {
		t.Errorf("ConnectTimeout = %v, want 7", got.ConnectTimeout)
	}
	if _, ok := sshConfigFrom(context.Background()); ok {
		t.Error("sshConfigFrom on bare context should report absent")
	}
}
