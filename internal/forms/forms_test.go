package forms

import (
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestRoleOptionsLabelsAndValues(t *testing.T) {
	opts := roleOptions()
	if len(opts) != len(inventory.AllRoles) {
		t.Fatalf("roleOptions() = %d options, want %d", len(opts), len(inventory.AllRoles))
	}
	for i, r := range inventory.AllRoles {
		o := opts[i]
		// Value must stay the bare role string so the inventory schema is
		// unchanged.
		if o.Value != string(r) {
			t.Errorf("option %d value = %q, want %q", i, o.Value, string(r))
		}
		// Label must carry both the role name and its description.
		if !strings.Contains(o.Key, r.String()) {
			t.Errorf("option %d label %q missing role name %q", i, o.Key, r.String())
		}
		if desc := roleDescriptions[r]; desc != "" && !strings.Contains(o.Key, desc) {
			t.Errorf("option %d label %q missing description %q", i, o.Key, desc)
		}
	}
}

func TestRoleDescriptionsCoverAllRoles(t *testing.T) {
	for _, r := range inventory.AllRoles {
		if strings.TrimSpace(roleDescriptions[r]) == "" {
			t.Errorf("role %q has no description", r)
		}
	}
}

func TestRequireNonEmpty(t *testing.T) {
	v := RequireNonEmpty("name")
	for _, in := range []string{"", " ", "\t\n"} {
		if err := v(in); err == nil {
			t.Errorf("RequireNonEmpty(%q) = nil, want error", in)
		}
	}
	if err := v("hello"); err != nil {
		t.Errorf("RequireNonEmpty(\"hello\") = %v, want nil", err)
	}
}

func TestIntInRange(t *testing.T) {
	v := IntInRange("threshold", 1, 99)
	cases := []struct {
		in      string
		wantErr string
	}{
		{"50", ""},
		{"1", ""},
		{"99", ""},
		{"0", "between 1 and 99"},
		{"100", "between 1 and 99"},
		{"abc", "must be a number"},
		{"", "must be a number"},
	}
	for _, tc := range cases {
		err := v(tc.in)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("IntInRange(%q) = %v, want nil", tc.in, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("IntInRange(%q) = nil, want error containing %q", tc.in, tc.wantErr)
		case tc.wantErr != "" && err != nil && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("IntInRange(%q) error = %q, want substring %q", tc.in, err, tc.wantErr)
		}
	}
}

func TestIntMin(t *testing.T) {
	v := IntMin("poll_interval", 1)
	if err := v("0"); err == nil {
		t.Error("IntMin allowed 0")
	}
	if err := v("30"); err != nil {
		t.Errorf("IntMin(30) = %v", err)
	}
}

// Note: the AskHost / AskShutdownDaemon / ConfirmXxx functions invoke
// huh.Form.Run() which requires an interactive terminal. They're not
// unit-tested here — coverage comes from end-to-end smoke tests of
// `homelab-nut init` and the TUI init flow.
