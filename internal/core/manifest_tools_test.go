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

func TestManifestToolContractMutationGapsBaseline(t *testing.T) {
	contracts := ManifestToolContracts()
	byName := make(map[string]ToolContract, len(contracts))
	for _, contract := range contracts {
		byName[contract.Name] = contract
	}

	// W0 characterization: each command has a mutating operation, but current
	// command-level inference projects the whole command as read-only. W2
	// replaces this assertion with per-operation effect truth.
	for _, name := range []string{"archive", "eval", "link", "new", "recurring", "spike", "unlink"} {
		contract, ok := byName[name]
		if !ok {
			t.Fatalf("%q missing from manifest contracts", name)
		}
		if contract.Mutable || contract.Capability != "read" {
			t.Fatalf("%q baseline = mutable %t, capability %q; update W0 characterization when W2 closes gap", name, contract.Mutable, contract.Capability)
		}
	}
}
