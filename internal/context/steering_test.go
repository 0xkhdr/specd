package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSteeringSelection(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "steering")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.md", "<!-- specd-context\ntags: go,test\nphases: execute\nroles: craftsman\nfiles: **/*.go\npriority: 20\n-->\n# Go\nUse table tests.\n")
	write("docs.md", "<!-- specd-context\ntags: docs\nphases: design\n-->\n# Docs\nWrite prose.\n")

	items, omissions, err := SelectSteering(root, SelectionContext{Phase: "execute", Role: "craftsman", Tags: []string{"go"}, Files: []string{"internal/x.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Source != ".specd/steering/go.md" || items[0].Applicability == "" {
		t.Fatalf("selected = %+v", items)
	}
	if len(omissions) != 1 || omissions[0].Source != ".specd/steering/docs.md" || omissions[0].Reason != "not applicable" {
		t.Fatalf("omissions = %+v", omissions)
	}
	items2, omissions2, err := SelectSteering(root, SelectionContext{Phase: "execute", Role: "craftsman", Tags: []string{"go"}, Files: []string{"internal/x.go"}})
	if err != nil || MachineManifestDigest(MachineManifest{Items: items}) != MachineManifestDigest(MachineManifest{Items: items2}) || len(omissions2) != 1 {
		t.Fatal("selection must be deterministic")
	}
}

func TestSteeringMetadataInvalid(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "steering")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("<!-- specd-context\npriority: nope\n-->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SelectSteering(root, SelectionContext{}); err == nil {
		t.Fatal("invalid declared metadata must fail")
	}
}
