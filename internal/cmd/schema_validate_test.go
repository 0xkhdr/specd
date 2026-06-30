package cmd_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestSchemaCmd confirms the survivor `specd check --schema` emits the embedded
// schema, that an unknown version fails closed, and that no .specd/ root is
// required.
func TestSchemaCmd(t *testing.T) {
	h := th.New(t)

	res := h.RunExpect(core.ExitOK, "check", "--schema")
	if !strings.Contains(res.Stdout, "$defs") || !strings.Contains(res.Stdout, "specdSchemaVersion") {
		t.Errorf("schema output missing expected keys:\n%s", res.Stdout)
	}
	// Output is valid JSON.
	var doc map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &doc); err != nil {
		t.Errorf("schema output is not valid JSON: %v", err)
	}

	h.RunExpect(core.ExitOK, "check", "--schema", "--version", "1")
	h.RunExpect(core.ExitGate, "check", "--schema", "--version", "99")
}

// TestValidateSchema confirms a freshly-created spec conforms via the survivor
// `specd check --schema-only`, and that an injected unknown property is reported
// as a conformance violation.
func TestValidateSchema(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitOK, "new", "widget")

	// A clean spec conforms.
	res := h.RunExpect(core.ExitOK, "check", "widget", "--schema-only")
	if !strings.Contains(res.Out(), "conforms") {
		t.Errorf("expected conformance message, got:\n%s", res.Out())
	}

	// Inject an unknown top-level property and confirm it is flagged.
	statePath := h.SpecPath("widget", "state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	obj["bogusField"] = json.RawMessage(`"x"`)
	mutated, _ := json.Marshal(obj)
	if err := os.WriteFile(statePath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}

	bad := h.RunExpect(core.ExitGate, "check", "widget", "--schema-only")
	if !strings.Contains(bad.Out(), "bogusField") {
		t.Errorf("expected violation naming bogusField, got:\n%s", bad.Out())
	}
}
