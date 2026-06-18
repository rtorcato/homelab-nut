package ups

import (
	"errors"
	"testing"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"bare tokens", `UPS myups Office`, []string{"UPS", "myups", "Office"}},
		{"quoted string", `UPS myups "Office UPS"`, []string{"UPS", "myups", "Office UPS"}},
		{"quoted with embedded quote", `UPS myups "Rack \"big\" UPS"`, []string{"UPS", "myups", `Rack "big" UPS`}},
		{"quoted with backslash", `VAR myups path "a\\b"`, []string{"VAR", "myups", "path", `a\b`}},
		{"empty quoted string", `VAR myups note ""`, []string{"VAR", "myups", "note", ""}},
		{"multi-space separator", `UPS    myups    Office`, []string{"UPS", "myups", "Office"}},
		{"empty input", ``, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLine(tc.in)
			if err != nil {
				t.Fatalf("parseLine(%q): %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseLine(%q) = %v (len %d), want %v (len %d)", tc.in, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseLine(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseLineUnterminatedQuote(t *testing.T) {
	_, err := parseLine(`UPS myups "unterminated`)
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("got %v, want errors.Is(ErrProtocol)", err)
	}
}

func TestMapErrKnownSentinels(t *testing.T) {
	cases := map[string]error{
		"ACCESS-DENIED":     ErrAccessDenied,
		"UNKNOWN-UPS":       ErrUnknownUPS,
		"VAR-NOT-SUPPORTED": ErrVarNotSupported,
	}
	for reason, want := range cases {
		got := mapErr(reason)
		if !errors.Is(got, want) {
			t.Errorf("mapErr(%q) = %v, want errors.Is %v", reason, got, want)
		}
	}
}

func TestMapErrUnknownWrapsReason(t *testing.T) {
	err := mapErr("DRIVER-NOT-CONNECTED")
	var ue *Error
	if !errors.As(err, &ue) {
		t.Fatalf("mapErr unknown returned %v, want *Error", err)
	}
	if ue.Reason != "DRIVER-NOT-CONNECTED" {
		t.Errorf("Reason = %q, want DRIVER-NOT-CONNECTED", ue.Reason)
	}
}
