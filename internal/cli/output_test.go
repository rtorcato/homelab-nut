package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/inventory"
)

func TestEmitJSON_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := emitJSON(&buf, map[string]int{"a": 1, "b": 2}); err != nil {
		t.Fatalf("emitJSON: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
	}
	if got["a"] != 1 || got["b"] != 2 {
		t.Errorf("round-trip mismatch: %v", got)
	}
}

func TestEmitValidateJSON_Success(t *testing.T) {
	var buf bytes.Buffer
	err := emitValidateJSON(&buf, "test.yaml", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var res struct {
		Valid bool   `json:"valid"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if !res.Valid {
		t.Errorf("Valid should be true")
	}
	if res.Path != "test.yaml" {
		t.Errorf("Path = %q", res.Path)
	}
}

func TestEmitValidateJSON_FailureSurfacesErrors(t *testing.T) {
	vErr := &inventory.ValidationError{
		Issues: []inventory.FieldError{
			{Path: "hosts[0].name", Message: "required"},
			{Path: "hosts[0].roles[0]", Message: "unknown role"},
		},
	}
	var buf bytes.Buffer
	err := emitValidateJSON(&buf, "bad.yaml", vErr)
	if !errors.Is(err, errSilent) {
		t.Errorf("expected errSilent for failure path, got %v", err)
	}
	var res struct {
		Valid  bool                  `json:"valid"`
		Errors []map[string]string   `json:"errors"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if res.Valid {
		t.Error("Valid should be false")
	}
	if len(res.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(res.Errors))
	}
	if !strings.Contains(buf.String(), "hosts[0].name") {
		t.Errorf("missing field path in output:\n%s", buf.String())
	}
}

func TestOutputFormat_DefaultIsText(t *testing.T) {
	// Without addOutputFlag the GetString lookup returns "" — that
	// should still resolve to text mode.
	cmd := newVersionCmd(BuildInfo{Version: "v0", Commit: "x", Date: "now"})
	if got := getOutputFormat(cmd); got != outputText {
		t.Errorf("default format = %q, want text", got)
	}
}

func TestExitCodesAreStable(t *testing.T) {
	// AGENTS.md documents these — guard against accidental renumbering.
	if ExitOK != 0 {
		t.Errorf("ExitOK = %d, want 0", ExitOK)
	}
	if ExitValidation != 1 {
		t.Errorf("ExitValidation = %d, want 1", ExitValidation)
	}
	if ExitNetwork != 2 {
		t.Errorf("ExitNetwork = %d, want 2", ExitNetwork)
	}
	if ExitApplyPartial != 3 {
		t.Errorf("ExitApplyPartial = %d, want 3", ExitApplyPartial)
	}
}
