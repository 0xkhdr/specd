package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestPortfolioExport(t *testing.T) {
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"zeta", "alpha"} {
		dir := filepath.Join(core.SpecdDir(root), "specs", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := core.SaveState(filepath.Join(dir, "state.json"), core.InitialState(slug)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "requirements.md"), []byte("PRIVATE-SOURCE-CONTENT\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	first, err := captureStdout(t, func() error { return Run(root, "report", nil, map[string]string{"portfolio": "true"}) })
	if err != nil {
		t.Fatal(err)
	}
	second, err := captureStdout(t, func() error { return Run(root, "report", nil, map[string]string{"portfolio": "true"}) })
	if err != nil {
		t.Fatal(err)
	}
	if first != second || strings.Index(first, "alpha") > strings.Index(first, "zeta") {
		t.Fatalf("unstable export: %s", first)
	}
	if strings.Contains(first, "PRIVATE-SOURCE-CONTENT") {
		t.Fatalf("source leaked: %s", first)
	}
}

func TestOutcomeReviewUnknown(t *testing.T) {
	out := renderOutcomeReview([]outcomeInput{{SpecID: "alpha"}})
	if !strings.Contains(out, `"outcome":"unknown"`) || strings.Contains(out, `"outcome":"success"`) {
		t.Fatalf("outcome=%s", out)
	}
	root := t.TempDir()
	dir := filepath.Join(core.SpecdDir(root), "specs", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveState(filepath.Join(dir, "state.json"), core.InitialState("alpha")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("# Tasks\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"alpha"}, map[string]string{"outcome-review": "true"})
	})
	if err != nil || !strings.Contains(cli, `"outcome":"unknown"`) {
		t.Fatalf("cli outcome=%s err=%v", cli, err)
	}
}
