package pack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestMigrationPacksApply proves the V10/P5.4 migration packs resolve, apply to
// a clean project, and ship a valid V5 eval rubric plus a task-DAG template —
// the "gate-passing scaffolds for all three packs" acceptance.
func TestMigrationPacksApply(t *testing.T) {
	for _, name := range []string{"migrate-deps", "modernize-tests", "upgrade-go"} {
		t.Run(name, func(t *testing.T) {
			p, err := ResolvePack(name, "")
			if err != nil {
				t.Fatalf("ResolvePack(%s): %v", name, err)
			}
			root := t.TempDir()
			if _, err := ApplyPack(root, p, false); err != nil {
				t.Fatalf("apply: %v", err)
			}

			// Steering + task template + rubric all landed.
			steering := filepath.Join(root, ".specd", "steering", name+".md")
			if _, err := os.Stat(steering); err != nil {
				t.Errorf("steering not written: %v", err)
			}
			tasks := filepath.Join(root, ".specd", "packs", name, "tasks-template.md")
			if b, err := os.ReadFile(tasks); err != nil || !strings.Contains(string(b), "Task DAG") {
				t.Errorf("task template missing/invalid: %v", err)
			}

			// The embedded eval rubric is a valid V5 rubric.
			rubricPath := filepath.Join(root, ".specd", "packs", name, "eval-rubric.json")
			rubric, _, err := core.LoadEvalRubric(rubricPath)
			if err != nil {
				t.Fatalf("rubric invalid: %v", err)
			}
			if rubric.Suite != name || len(rubric.Checks) == 0 {
				t.Errorf("rubric suite=%q checks=%d", rubric.Suite, len(rubric.Checks))
			}
		})
	}
}

// TestMigrationPacksListed confirms the three migration packs surface in
// --list-packs (BuiltinPacks) so operators can discover them.
func TestMigrationPacksListed(t *testing.T) {
	packs, err := BuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range packs {
		have[p.Name] = true
	}
	for _, want := range []string{"migrate-deps", "modernize-tests", "upgrade-go"} {
		if !have[want] {
			t.Errorf("built-in packs missing %q", want)
		}
	}
}

// TestMigrationPackRubricJSONShape is a guard that each embedded rubric decodes
// with the expected top-level keys (no drift from the V5 schema).
func TestMigrationPackRubricJSONShape(t *testing.T) {
	for _, name := range []string{"migrate-deps", "modernize-tests", "upgrade-go"} {
		p, err := ResolvePack(name, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range p.Files {
			if !strings.HasSuffix(f.Path, "eval-rubric.json") {
				continue
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(f.Content), &doc); err != nil {
				t.Errorf("%s rubric not JSON: %v", name, err)
			}
			if _, ok := doc["checks"]; !ok {
				t.Errorf("%s rubric missing checks key", name)
			}
		}
	}
}
