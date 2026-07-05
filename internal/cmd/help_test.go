package cmd

import (
	"encoding/json"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestHelpJSON pins spec 03 R4: `help --json` emits the full palette as stable,
// machine-readable JSON that unmarshals into the versioned core.HelpPayload
// struct, carries the payload schema version, and lists every command in help
// order.
func TestHelpJSON(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return runHelp("", nil, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatalf("runHelp --json: %v", err)
	}

	var payload core.HelpPayload
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("help --json is not valid HelpPayload: %v\n%s", err, out)
	}
	if payload.SchemaVersion != core.HelpSchemaVersion {
		t.Errorf("schema version = %d, want %d", payload.SchemaVersion, core.HelpSchemaVersion)
	}
	if len(payload.Commands) != len(core.Commands) {
		t.Fatalf("payload has %d commands, want %d", len(payload.Commands), len(core.Commands))
	}
	for i, want := range core.CommandNames() {
		if payload.Commands[i].Name != want {
			t.Errorf("command %d = %q, want %q (order must be stable)", i, payload.Commands[i].Name, want)
		}
	}
	// Every command carries its enforcement contract.
	for _, cmd := range payload.Commands {
		if len(cmd.AllowedPhases) == 0 {
			t.Errorf("%s: help payload dropped AllowedPhases", cmd.Name)
		}
		if len(cmd.ExitCodes) == 0 {
			t.Errorf("%s: help payload dropped ExitCodes", cmd.Name)
		}
	}
}
