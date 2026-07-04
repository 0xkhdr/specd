package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadToolPolicyMalformedManifestDeniesOptionalTools(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"optional":`), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := LoadToolPolicy(path)
	if len(policy.Optional) != 0 {
		t.Fatalf("expected empty policy, got %#v", policy.Optional)
	}
}

func TestLoadToolPolicyNeverAllowsForbiddenTools(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"optional":["approve","context"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := LoadToolPolicy(path)
	if policy.Optional["approve"] {
		t.Fatal("forbidden tool allowed")
	}
	if !policy.Optional["context"] {
		t.Fatal("allowed optional tool missing")
	}
}
