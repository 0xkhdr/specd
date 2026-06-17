package core

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
)

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
