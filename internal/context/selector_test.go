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

func TestSelectorNamesMissingRequiredSource(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".specd/specs/demo"), 0o755)
	os.MkdirAll(filepath.Join(root, ".specd/roles"), 0o755)
	os.WriteFile(filepath.Join(root, ".specd/specs/demo/requirements.md"), []byte("r"), 0o644)
	os.WriteFile(filepath.Join(root, ".specd/roles/craftsman.md"), []byte("r"), 0o644)
	_, err := SelectRequiredLanes(root, "demo", core.TaskRow{ID: "T1", Role: "craftsman"})
	if err == nil || !strings.Contains(err.Error(), ".specd/specs/demo/design.md") || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
}
