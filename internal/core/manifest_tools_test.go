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

func TestManifestToolContractsUseOperationEffects(t *testing.T) {
	contracts := ManifestToolContracts()
	byName := make(map[string]ToolContract, len(contracts))
	for _, contract := range contracts {
		byName[contract.OperationID] = contract
	}

	for _, name := range []string{"archive", "eval.import", "link", "new", "recurring.record", "spike", "unlink"} {
		contract, ok := byName[name]
		if !ok {
			t.Fatalf("%q missing from manifest contracts", name)
		}
		if !contract.Mutable || contract.Capability != "write" {
			t.Fatalf("%q = mutable %t, capability %q", name, contract.Mutable, contract.Capability)
		}
	}
	if got := byName["eval.status"]; got.Mutable || got.Capability != "read" {
		t.Fatalf("eval.status = mutable %t, capability %q", got.Mutable, got.Capability)
	}
}
