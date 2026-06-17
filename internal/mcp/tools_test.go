package mcp

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// updateGolden regenerates the schema golden instead of asserting against it:
//
//	go test ./internal/mcp/ -run Schema -update
var updateGolden = flag.Bool("update", false, "rewrite golden schema snapshot")

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

// TestToolSchemaGolden snapshots every tool's name → input-schema mapping to a
// golden file (R2.3). Any change to a tool's input schema — a renamed flag, a
// changed type, an added/removed tool — diffs against the golden and fails,
// forcing the change to be deliberate. Regenerate with `-update` after an
// intentional schema change. The snapshot is keyed by tool name (stable, not
// help-display order) so reordering commands is not a spurious failure.
func TestToolSchemaGolden(t *testing.T) {
	schemas := map[string]jsonSchema{}
	for _, tl := range buildTools() {
		schemas[tl.Name] = tl.InputSchema
	}
	got, err := json.MarshalIndent(schemas, "", "  ")
	if err != nil {
		t.Fatalf("marshal schemas: %v", err)
	}
	got = append(got, '\n')

	golden := filepath.Join("testdata", "tool_schemas.golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s (%d tools)", golden, len(schemas))
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("tool input schemas drifted from golden.\n"+
			"If intentional, regenerate: go test ./internal/mcp/ -run Schema -update\n"+
			"--- got ---\n%s", got)
	}
}

// TestToolSchemaValidity asserts each generated tool input schema is a
// structurally valid JSON-Schema object (R2.1): an object type, a non-nil
// properties map, an ordered string-array `args`, and a concrete type on every
// flag property. A host must be able to consume every advertised schema.
func TestToolSchemaValidity(t *testing.T) {
	for _, tl := range buildTools() {
		if tl.Name == "" {
			t.Error("tool with empty name")
		}
		s := tl.InputSchema
		if s.Type != "object" {
			t.Errorf("%s: inputSchema.type = %q, want object", tl.Name, s.Type)
		}
		if s.Properties == nil {
			t.Errorf("%s: nil properties", tl.Name)
			continue
		}
		args, ok := s.Properties["args"]
		if !ok || args.Type != "array" || args.Items == nil || args.Items.Type != "string" {
			t.Errorf("%s: args must be an array of strings, got %+v", tl.Name, args)
		}
		for name, p := range s.Properties {
			if p.Type != "string" && p.Type != "boolean" && p.Type != "array" {
				t.Errorf("%s: property %q has unsupported type %q", tl.Name, name, p.Type)
			}
		}
	}
}
