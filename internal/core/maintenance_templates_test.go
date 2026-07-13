package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaintenanceTemplates(t *testing.T) {
	templates, err := MaintenanceTemplates()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dependency", "incident", "migration", "recurring"}
	if len(templates) != len(want) {
		t.Fatalf("got %d templates, want %d", len(templates), len(want))
	}
	for i, template := range templates {
		if template.Name != want[i] || template.Schema != "specd-maintenance" || template.Version != 1 {
			t.Fatalf("template %d metadata = %+v", i, template)
		}
		for _, section := range []string{"Source", "Requirements", "Tasks", "Evidence", "Learning"} {
			if !strings.Contains(template.Body, "## "+section) {
				t.Errorf("%s missing %s section", template.Name, section)
			}
		}
	}
}

func TestTemplateRefreshPreserves(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".specd", "templates", "maintenance", "incident.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	custom := "\n## Project notes\nkeep this exactly\n"
	if err := os.WriteFile(path, append(raw, custom...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyManagedRepair(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(after), custom) {
		t.Fatalf("refresh clobbered user content:\n%s", after)
	}
}

func TestTemplateReadinessPassable(t *testing.T) {
	templates, err := MaintenanceTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, template := range templates {
		start := strings.Index(template.Body, "```json\n")
		if start < 0 {
			t.Fatalf("%s missing provenance JSON", template.Name)
		}
		end := strings.Index(template.Body[start+8:], "\n```")
		if end < 0 {
			t.Fatalf("%s has unterminated provenance JSON", template.Name)
		}
		provenance, err := DecodeProvenance([]byte(template.Body[start+8 : start+8+end]))
		if err != nil {
			t.Fatalf("%s provenance: %v", template.Name, err)
		}
		if provenance.SchemaVersion != ProvenanceSchemaV1 || provenance.SourceType == "" || provenance.SourceRef == "" || len(provenance.Systems) == 0 || len(provenance.AffectedSpecs) == 0 || provenance.Severity == "" || provenance.Risk == "" || provenance.Owner == "" || len(provenance.PriorLinks) == 0 {
			t.Errorf("%s intake is incomplete: %+v", template.Name, provenance)
		}
		if link := provenance.PriorLinks[0]; !link.Kind.Valid() || link.Reason == "" {
			t.Errorf("%s successor link is incomplete: %+v", template.Name, link)
		}
		if len(provenance.RequiredFields) == 0 {
			t.Errorf("%s has no configured readiness policy", template.Name)
		}
	}
}
