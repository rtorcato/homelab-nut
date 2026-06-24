package inventory

import (
	"path/filepath"
	"testing"
)

// TestSampleInventoriesValidate loads every shipped sample inventory through
// Load (which parses + Validate()s) so the commented examples under
// examples/inventories/ can never drift out of sync with the schema. Tests run
// from the package directory, so the relative path resolves to the repo root.
func TestSampleInventoriesValidate(t *testing.T) {
	paths, err := filepath.Glob("../../examples/inventories/*.yaml")
	if err != nil {
		t.Fatalf("glob sample inventories: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no sample inventories found under examples/inventories/ — did the folder move?")
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			if _, err := Load(p); err != nil {
				t.Errorf("%s should parse and validate cleanly: %v", p, err)
			}
		})
	}
}
