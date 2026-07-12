package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScaffoldCarriesFormatGuidance pins spec 01 R3.1/R2.1 authoring migration
// guidance: a scaffolded project's structure steering documents the design
// decision contract and the optional task trace/risk columns, and states that
// legacy tables stay backward compatible.
func TestScaffoldCarriesFormatGuidance(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	assets, err := ManagedAssets()
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for _, asset := range assets {
		if strings.HasSuffix(asset.RelPath, "structure.md") {
			raw, err := os.ReadFile(filepath.Join(root, asset.RelPath))
			if err != nil {
				t.Fatalf("read %s: %v", asset.RelPath, err)
			}
			body = string(raw)
		}
	}
	if body == "" {
		t.Fatal("structure steering not scaffolded")
	}
	for _, want := range []string{"references:", "boundaries:", "risk", "refs", "backward compatible"} {
		if !strings.Contains(body, want) {
			t.Fatalf("scaffold authoring guidance missing %q:\n%s", want, body)
		}
	}
}

func TestScaffoldCommandsUseExplicitPlaceholders(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".specd/steering/memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "specd memory <slug> add") {
		t.Fatalf("memory guidance lacks required spec placeholder:\n%s", raw)
	}
}

func TestScaffoldCreatesPortableSkillsRoot(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".specd", "skills", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SKILL.md", "specd-skill", "provenance", "capabilities", "advisory"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("skills scaffold missing %q:\n%s", want, raw)
		}
	}
}

func TestScaffoldManagedGuidanceDigestPreservesUserRegion(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	digest, err := GuidanceDigest(root)
	if err != nil || digest == "" {
		t.Fatalf("guidance digest: %q, %v", digest, err)
	}
	path := filepath.Join(root, "AGENTS.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte("user-owned\n"), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := GuidanceDigest(root)
	if err != nil || changed != digest {
		t.Fatalf("user-owned guidance changed digest: %q != %q", changed, digest)
	}
}
