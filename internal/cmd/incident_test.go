package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestIncidentSeed(t *testing.T) {
	root := t.TempDir()
	if err := runIncident(root, []string{"seed", "checkout-recovery"}, map[string]string{
		"source-spec": "checkout", "release": "rel-7", "deployment": "dep-4",
		"criterion": "availability", "evidence-ref": "obs://health/42,runbook://rollback/7",
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", "checkout-recovery")
	requirements, err := os.ReadFile(filepath.Join(dir, "requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(requirements)
	for _, want := range []string{"rel-7", "dep-4", "availability", "obs://health/42", "runbook://rollback/7"} {
		if !strings.Contains(text, want) {
			t.Fatalf("requirements missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "raw_payload") {
		t.Fatal("raw payload entered seeded context")
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	if err := runIncident(root, []string{"seed", "bad"}, map[string]string{
		"source-spec": "checkout", "release": "rel-7", "deployment": "dep-4",
		"criterion": "availability", "evidence-ref": "https://example.invalid/raw?token=secret",
	}); err == nil {
		t.Fatal("unsafe evidence reference accepted")
	}
}
