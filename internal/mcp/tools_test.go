package mcp

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestToolsList asserts the generated tool list mirrors the command schema:
// one tool per non-meta command, correct namespacing, and read-only annotations
// matching the spec's R4 classification.
func TestToolsList(t *testing.T) {
	tools := buildTools()

	// Parity: exactly one tool per non-meta core.Commands entry (R2). Adding a
	// command without it surfacing as a tool fails here.
	wantCount := 0
	for _, c := range core.Commands {
		if !metaCommands[c.Command] {
			wantCount++
		}
	}
	if len(tools) != wantCount {
		t.Fatalf("tool count = %d, want %d (one per non-meta command)", len(tools), wantCount)
	}

	byName := map[string]toolDef{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}

	// Meta commands are never exposed.
	for _, name := range []string{"specd_help", "specd_version", "specd_mcp"} {
		if _, ok := byName[name]; ok {
			t.Errorf("meta command exposed as tool: %s", name)
		}
	}

	// Read-only annotation (R4).
	if tl := byName["specd_status"]; !tl.Annotations.ReadOnlyHint {
		t.Error("specd_status should be readOnlyHint:true")
	}
	if tl := byName["specd_verify"]; tl.Annotations.ReadOnlyHint {
		t.Error("specd_verify should be readOnlyHint:false")
	}

	// Each tool carries an object input schema with an ordered args array.
	for _, tl := range tools {
		if tl.InputSchema.Type != "object" {
			t.Errorf("%s inputSchema.type = %q, want object", tl.Name, tl.InputSchema.Type)
		}
		if p, ok := tl.InputSchema.Properties["args"]; !ok || p.Type != "array" {
			t.Errorf("%s missing args array property", tl.Name)
		}
	}

	// Verify's --status flag is surfaced as a typed property.
	if tl, ok := byName["specd_verify"]; ok {
		if p, ok := tl.InputSchema.Properties["status"]; !ok || p.Type != "string" {
			t.Errorf("specd_verify missing string 'status' flag prop")
		}
	}
}
