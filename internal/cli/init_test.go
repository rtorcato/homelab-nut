package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultInventoryPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	t.Run("no project-local file falls back to home", func(t *testing.T) {
		t.Chdir(t.TempDir()) // empty dir, no homelab-nut.yaml
		want := filepath.Join(home, inventoryFileName)
		if got := defaultInventoryPath(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("project-local file is preferred", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := os.WriteFile(inventoryFileName, []byte("hosts: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := defaultInventoryPath(); got != inventoryFileName {
			t.Errorf("got %q, want %q", got, inventoryFileName)
		}
	})
}
