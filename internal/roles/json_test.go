package roles

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStateMarshalJSON(t *testing.T) {
	cases := map[State]string{
		StateUnknown: `"unknown"`,
		StateMissing: `"missing"`,
		StatePartial: `"partial"`,
		StateOK:      `"ok"`,
	}
	for s, want := range cases {
		got, err := s.MarshalJSON()
		if err != nil {
			t.Errorf("MarshalJSON(%v): %v", s, err)
			continue
		}
		if string(got) != want {
			t.Errorf("MarshalJSON(%v) = %s, want %s", s, got, want)
		}
	}
}

func TestStateMarshalJSON_InStruct(t *testing.T) {
	// Verify States embedded in a struct serialise correctly — catches
	// any future regression where the custom marshal is bypassed.
	type wrap struct {
		Current State `json:"current"`
		Target  State `json:"target"`
	}
	w := wrap{Current: StateMissing, Target: StateOK}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"current":"missing"`) || !strings.Contains(got, `"target":"ok"`) {
		t.Errorf("nested marshal wrong: %s", got)
	}
}
