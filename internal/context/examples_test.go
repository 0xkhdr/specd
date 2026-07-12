package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExamplesSelection(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "examples")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("verify.md", "<!-- specd-example\nid: verify-failure\nversion: 1\nphases: execute\nroles: craftsman\ntags: go\nnegative: true\npriority: 70\n-->\n# Bad\nDo not ignore failure.\n")
	write("design.md", "<!-- specd-example\nid: design\nversion: 1\nphases: design\n-->\n# Design\n")

	items, omissions, err := SelectExamples(root, SelectionContext{Phase: "execute", Role: "craftsman", Tags: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Selector != "example:verify-failure@1:negative" || items[0].Trust != "example" {
		t.Fatalf("selected = %+v", items)
	}
	if len(omissions) != 1 || omissions[0].Reason != "not applicable" {
		t.Fatalf("omissions = %+v", omissions)
	}
}

func TestExamplesRejectUnknownVersion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "examples")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("<!-- specd-example\nid: bad\nversion: 99\nphases: execute\n-->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SelectExamples(root, SelectionContext{Phase: "execute"}); err == nil {
		t.Fatal("unknown version must fail")
	}
}
