package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestSchemaParse(t *testing.T) {
	doc, err := ParseSchema("")
	if err != nil {
		t.Fatalf("ParseSchema(default): %v", err)
	}
	if doc.SpecdSchemaVersion != SchemaVersionID {
		t.Errorf("specdSchemaVersion = %q, want %q", doc.SpecdSchemaVersion, SchemaVersionID)
	}
	if doc.StateSchemaVersion != SchemaVersion {
		t.Errorf("stateSchemaVersion = %d, want %d (state.go SchemaVersion)", doc.StateSchemaVersion, SchemaVersion)
	}
	if doc.ID == "" {
		t.Error("schema missing $id")
	}
	for _, name := range []string{"State", "TaskState", "VerificationRecord", "CriterionRecord", "Blocker"} {
		if _, ok := doc.Defs[name]; !ok {
			t.Errorf("schema $defs missing %q", name)
		}
	}
	// "v1" alias resolves to the same document; unknown versions fail closed.
	if _, err := ParseSchema("v1"); err != nil {
		t.Errorf("ParseSchema(v1): %v", err)
	}
	if _, err := Schema("9"); err == nil {
		t.Error("Schema(unknown) must error, got nil")
	}
}

// jsonFields returns the JSON field name and whether it is omitempty for every
// serialized field of a struct type.
func jsonFields(rt reflect.Type) map[string]bool /* name -> omitempty */ {
	out := map[string]bool{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "" {
			continue
		}
		omitempty := false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				omitempty = true
			}
		}
		out[name] = omitempty
	}
	return out
}

// TestSchemaConformance is the drift trip-wire: it reflects the canonical Go
// types and asserts the embedded schema mirrors them exactly. Adding a struct
// field without updating schema/v1.json (or vice versa) fails this test, so the
// schema cannot silently fall out of sync with the source of truth.
func TestSchemaConformance(t *testing.T) {
	doc, err := ParseSchema("")
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}

	types := map[string]reflect.Type{
		"State":              reflect.TypeOf(State{}),
		"TaskState":          reflect.TypeOf(TaskState{}),
		"VerificationRecord": reflect.TypeOf(VerificationRecord{}),
		"CriterionRecord":    reflect.TypeOf(CriterionRecord{}),
		"Blocker":            reflect.TypeOf(Blocker{}),
	}

	for name, rt := range types {
		def, ok := doc.Defs[name]
		if !ok {
			t.Errorf("%s: no $defs entry", name)
			continue
		}
		fields := jsonFields(rt)

		// Every Go field is a schema property.
		for fname := range fields {
			if _, ok := def.Properties[fname]; !ok {
				t.Errorf("%s.%s: Go field missing from schema properties (struct changed without schema update)", name, fname)
			}
		}
		// Every schema property is a Go field.
		for pname := range def.Properties {
			if _, ok := fields[pname]; !ok {
				t.Errorf("%s.%s: schema property has no matching Go field (schema changed without struct update)", name, pname)
			}
		}
		// required == the set of non-omitempty fields.
		gotRequired := map[string]bool{}
		for fname, omit := range fields {
			if !omit {
				gotRequired[fname] = true
			}
		}
		schemaRequired := map[string]bool{}
		for _, r := range def.Required {
			schemaRequired[r] = true
		}
		for r := range gotRequired {
			if !schemaRequired[r] {
				t.Errorf("%s.%s: non-omitempty Go field not in schema 'required'", name, r)
			}
		}
		for r := range schemaRequired {
			if !gotRequired[r] {
				t.Errorf("%s.%s: schema 'required' field is omitempty (or absent) in Go", name, r)
			}
		}
		// Canonical objects are closed.
		if def.AdditionalProperties == nil || *def.AdditionalProperties {
			t.Errorf("%s: schema should set additionalProperties:false", name)
		}
	}
}
