package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpecResolverPrecedence(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(root, ".specd/specs", slug), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ResolveSpec(root, "b", "a")
	if err != nil || got.Slug != "b" || got.Source != "explicit" {
		t.Fatalf("explicit = %+v, %v", got, err)
	}
	got, err = ResolveSpec(root, "", "a")
	if err != nil || got.Slug != "a" || got.Source != "pinned" {
		t.Fatalf("pinned = %+v, %v", got, err)
	}
}

func TestSpecResolverSingleAndAmbiguous(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".specd/specs/only"), 0o755)
	got, err := ResolveSpec(root, "", "")
	if err != nil || got.Source != "single" || got.Slug != "only" {
		t.Fatalf("single = %+v, %v", got, err)
	}
	os.MkdirAll(filepath.Join(root, ".specd/specs/other"), 0o755)
	if _, err := ResolveSpec(root, "", ""); err == nil || FindingCode(err) != "SPEC_AMBIGUOUS" {
		t.Fatalf("ambiguity error = %v", err)
	}
}

func TestSpecResolverInvalidPinFailsClosed(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".specd/specs/only"), 0o755)
	if _, err := ResolveSpec(root, "", "missing"); err == nil || FindingCode(err) != "SPEC_PIN_INVALID" {
		t.Fatalf("invalid pin = %v", err)
	}
}
