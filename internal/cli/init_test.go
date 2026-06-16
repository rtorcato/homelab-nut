package cli

import (
	"strings"
	"testing"
)

func TestRequireNonEmpty(t *testing.T) {
	v := requireNonEmpty("name")
	for _, in := range []string{"", " ", "\t\n"} {
		if err := v(in); err == nil {
			t.Errorf("requireNonEmpty(%q) = nil, want error", in)
		}
	}
	if err := v("hello"); err != nil {
		t.Errorf("requireNonEmpty(\"hello\") = %v, want nil", err)
	}
}

func TestIntInRange(t *testing.T) {
	v := intInRange("threshold", 1, 99)
	cases := []struct {
		in      string
		wantErr string // substring; empty means expect nil
	}{
		{"50", ""},
		{"1", ""},
		{"99", ""},
		{"0", "between 1 and 99"},
		{"100", "between 1 and 99"},
		{"-3", "between 1 and 99"},
		{"abc", "must be a number"},
		{"", "must be a number"},
	}
	for _, tc := range cases {
		err := v(tc.in)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("intInRange(%q) = %v, want nil", tc.in, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("intInRange(%q) = nil, want error containing %q", tc.in, tc.wantErr)
		case tc.wantErr != "" && err != nil && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("intInRange(%q) error = %q, want substring %q", tc.in, err, tc.wantErr)
		}
	}
}

func TestIntMin(t *testing.T) {
	v := intMin("poll_interval", 1)
	if err := v("0"); err == nil {
		t.Error("intMin allowed 0")
	}
	if err := v("30"); err != nil {
		t.Errorf("intMin(30) = %v", err)
	}
}
