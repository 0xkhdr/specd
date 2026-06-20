package core

import (
	"encoding/json"
	"io/fs"
	"os"
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
	for _, name := range []string{
		"State", "TaskState", "VerificationRecord", "CriterionRecord", "Blocker",
		"ACPEnvelope", "ACPAuthority", "MissionContextManifest", "MissionContextItem",
		"ACPMissionPayload", "ACPAcceptedPayload", "ACPHeartbeatPayload", "ACPProgressPayload", "ACPEvidencePayload",
		"ACPBlockerPayload", "ACPQueryPayload", "ACPDirectivePayload", "ACPCancelledPayload",
	} {
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
		"State":                  reflect.TypeOf(State{}),
		"TaskState":              reflect.TypeOf(TaskState{}),
		"VerificationRecord":     reflect.TypeOf(VerificationRecord{}),
		"CriterionRecord":        reflect.TypeOf(CriterionRecord{}),
		"Blocker":                reflect.TypeOf(Blocker{}),
		"ACPEnvelope":            reflect.TypeOf(ACPEnvelope{}),
		"ACPAuthority":           reflect.TypeOf(ACPAuthority{}),
		"MissionContextManifest": reflect.TypeOf(MissionContextManifest{}),
		"MissionContextItem":     reflect.TypeOf(MissionContextItem{}),
		"ACPMissionPayload":      reflect.TypeOf(ACPMissionPayload{}),
		"ACPAcceptedPayload":     reflect.TypeOf(ACPAcceptedPayload{}),
		"ACPHeartbeatPayload":    reflect.TypeOf(ACPHeartbeatPayload{}),
		"ACPProgressPayload":     reflect.TypeOf(ACPProgressPayload{}),
		"ACPEvidencePayload":     reflect.TypeOf(ACPEvidencePayload{}),
		"ACPBlockerPayload":      reflect.TypeOf(ACPBlockerPayload{}),
		"ACPQueryPayload":        reflect.TypeOf(ACPQueryPayload{}),
		"ACPDirectivePayload":    reflect.TypeOf(ACPDirectivePayload{}),
		"ACPCancelledPayload":    reflect.TypeOf(ACPCancelledPayload{}),
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

// genState writes a fresh state.json the way `specd new` does (InitialState +
// CAS save) and returns the on-disk bytes — the real generated artifact.
func genState(t *testing.T, slug string) []byte {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, "Generated "+slug)
	if _, err := WithSpecLock[int](root, slug, func() (int, error) {
		return 0, SaveState(root, slug, &st)
	}); err != nil {
		t.Fatalf("save generated state: %v", err)
	}
	return readStateRaw(t, root, slug)
}

// R1.1: a state.json that `specd new` generates validates against the embedded
// schema — downstream tooling never chokes on a freshly scaffolded spec.
func TestGeneratedStateValidatesAgainstSchema(t *testing.T) {
	raw := genState(t, "fresh")
	viols, err := ValidateState(raw, SchemaVersionID)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	if len(viols) != 0 {
		t.Errorf("generated state.json is not schema-valid: %v", viols)
	}
}

// R1.2: an artifact that violates the schema (a required property removed) is
// rejected — `specd check`'s validator surfaces it rather than passing it on.
func TestSchemaRejectsInvalidArtifact(t *testing.T) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(genState(t, "broken"), &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, "status") // status is a required State property
	raw, _ := json.Marshal(doc)

	viols, err := ValidateState(raw, SchemaVersionID)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	if len(viols) == 0 {
		t.Error("R1.2: a state.json missing a required property was accepted")
	}
}

// R1.3: the schema specd serves declares the same versions the state files it
// validates declare, so `specd schema` and on-disk state can never disagree.
func TestServedSchemaVersionMatchesState(t *testing.T) {
	doc, err := ParseSchema("")
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if doc.StateSchemaVersion != SchemaVersion {
		t.Errorf("schema stateSchemaVersion = %d, want %d (state.go SchemaVersion)", doc.StateSchemaVersion, SchemaVersion)
	}
	if doc.SpecdSchemaVersion != SchemaVersionID {
		t.Errorf("schema specdSchemaVersion = %q, want %q", doc.SpecdSchemaVersion, SchemaVersionID)
	}
}

// R4.1: every embedded template artifact is well-formed — a JSON template parses
// as JSON and a markdown template is non-empty — so a corrupt embed can never
// ship as a usable scaffold.
func TestEmbeddedTemplatesAreWellFormed(t *testing.T) {
	count := 0
	err := fs.WalkDir(TemplatesFS, "embed_templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		b, rerr := TemplatesFS.ReadFile(path)
		if rerr != nil {
			t.Errorf("read template %s: %v", path, rerr)
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			var v interface{}
			if jerr := json.Unmarshal(b, &v); jerr != nil {
				t.Errorf("template %s is not valid JSON: %v", path, jerr)
			}
			return nil
		}
		if len(strings.TrimSpace(string(b))) == 0 {
			t.Errorf("template %s is empty", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
	if count == 0 {
		t.Fatal("no embedded templates found — embed broken")
	}
}

// R4.2: a state document carrying a property absent from the schema fails
// validation. This is the lockstep trip-wire — a template (or struct) that
// references a field the schema does not define cannot pass silently.
func TestTemplateSchemaLockstepRejectsUnknownField(t *testing.T) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(genState(t, "extra"), &doc); err != nil {
		t.Fatal(err)
	}
	doc["bogusFieldNotInSchema"] = json.RawMessage(`"x"`)
	raw, _ := json.Marshal(doc)

	viols, err := ValidateState(raw, SchemaVersionID)
	if err != nil {
		t.Fatalf("ValidateState: %v", err)
	}
	found := false
	for _, v := range viols {
		if strings.Contains(v, "bogusFieldNotInSchema") || strings.Contains(v, "additionalProperties") {
			found = true
		}
	}
	if !found {
		t.Errorf("R4.2: unknown property was not flagged; violations=%v", viols)
	}
}
