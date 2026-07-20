package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestSelectorRequiredLanes(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		".specd/specs/demo/requirements.md": "# Requirements\n",
		".specd/specs/demo/design.md":       "# Design\n",
		".specd/roles/craftsman.md":         "# Role\n",
		"internal/a.go":                     "package internal\n",
		"internal/a_test.go":                "package internal\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	task := core.TaskRow{ID: "T7", Role: "craftsman", DeclaredFiles: []string{"internal/a.go", "internal/a_test.go"}, Verify: "go test ./...", Acceptance: "R2.1"}
	items, err := SelectRequiredLanes(root, "demo", task)
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []string{"design", "requirements", "role", "source", "task", "test"}
	if len(items) != len(wantKinds) {
		t.Fatalf("items = %+v", items)
	}
	for i, want := range wantKinds {
		if items[i].Kind != want || !items[i].Required || items[i].SourceDigest == "" {
			t.Fatalf("item %d = %+v, want required %s with digest", i, items[i], want)
		}
		wantTrust := ContentTrustUntrustedData
		if items[i].ContentTrust != wantTrust {
			t.Errorf("item %s content trust = %q, want %q", want, items[i].ContentTrust, wantTrust)
		}
	}
}

func TestSelectorGreenfieldDeclaredFile(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		".specd/specs/demo/requirements.md": "# Requirements\n",
		".specd/specs/demo/design.md":       "# Design\n",
		".specd/roles/craftsman.md":         "# Role\n",
		"internal/existing.go":              "package internal\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// One existing output, one that does not exist yet (the task will create it).
	task := core.TaskRow{ID: "T3", Role: "craftsman", DeclaredFiles: []string{"internal/existing.go", "internal/new/module.go"}, Verify: "go test ./...", Acceptance: "R1"}
	items, err := SelectRequiredLanes(root, "demo", task)
	if err != nil {
		t.Fatalf("greenfield declared file must not fail context: %v", err)
	}
	for _, item := range items {
		if item.Source == "internal/new/module.go" {
			t.Fatalf("missing output must not appear as a required source lane: %+v", item)
		}
	}
	var loadedExisting bool
	for _, item := range items {
		if item.Source == "internal/existing.go" {
			loadedExisting = true
		}
	}
	if !loadedExisting {
		t.Fatalf("existing declared file must load as a source lane: %+v", items)
	}
}

func TestSelectorDeclaredFileTraversalFails(t *testing.T) {
	root := t.TempDir()
	for name := range map[string]string{
		".specd/specs/demo/requirements.md": "r",
		".specd/specs/demo/design.md":       "d",
		".specd/roles/craftsman.md":         "role",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	task := core.TaskRow{ID: "T9", Role: "craftsman", DeclaredFiles: []string{"../outside.go"}, Verify: "go test ./...", Acceptance: "R1"}
	if _, err := SelectRequiredLanes(root, "demo", task); err == nil {
		t.Fatal("declared file escaping repository base must fail closed")
	}
}

func TestSelectorNamesMissingRequiredSource(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".specd/specs/demo", ".specd/roles"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{".specd/specs/demo/requirements.md", ".specd/roles/craftsman.md"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("r"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := SelectRequiredLanes(root, "demo", core.TaskRow{ID: "T1", Role: "craftsman"})
	if err == nil || !strings.Contains(err.Error(), ".specd/specs/demo/design.md") || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
}
