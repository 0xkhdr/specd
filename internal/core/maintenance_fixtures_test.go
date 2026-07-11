package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestMaintenanceFixture pins the operating-model contract (docs/operating-model-contract.md)
// against the canonical offline fixtures under testdata/maintenance. 09a validates structure
// only: every fixture decodes and carries the required, non-empty fields for its schema, and each
// closed-set enum value (link kind, provenance source_type, decision status) is a member of its
// set. The behavioral rules — typed-link decode (09b), intake readiness (09c), decision/exception
// lifecycle (09d), memory aging (09e) — land in later waves that extend this test rather than
// redefining the fields.
func TestMaintenanceFixture(t *testing.T) {
	// required[schema] = contract fields that must be present and non-empty.
	required := map[string][]string{
		"ProgramLinkV2": {"from", "to", "kind", "reason", "created_at"},
		"ProvenanceV1":  {"schema_version", "source_type", "source_ref", "systems", "severity", "owner"},
		"DecisionV1":    {"id", "status", "owner", "created_at", "review_at", "expires_at"},
		"MemoryEntryV1": {"key", "owner", "last_validated_at", "provenance", "confidence", "expires_at"},
	}
	linkKinds := map[string]bool{"follows": true, "regresses": true, "maintains": true, "supersedes": true}
	sourceTypes := map[string]bool{
		"feature": true, "incident": true, "vulnerability": true, "drift": true,
		"dependency": true, "migration": true, "deprecation": true, "policy": true,
	}
	decisionStatuses := map[string]bool{
		"proposed": true, "accepted": true, "superseded": true, "expired": true, "revoked": true,
	}

	dir := filepath.Join("..", "..", "testdata", "maintenance")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("%s: read: %v", e.Name(), err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Errorf("%s: not valid JSON: %v", e.Name(), err)
			continue
		}
		schema, _ := fields["schema"].(string)
		want, ok := required[schema]
		if !ok {
			t.Errorf("%s: unknown or missing schema %q", e.Name(), schema)
			continue
		}
		seen[schema] = true
		for _, f := range want {
			v, present := fields[f]
			if !present || v == nil || v == "" {
				t.Errorf("%s (%s): missing or empty required field %q", e.Name(), schema, f)
			}
		}
		switch schema {
		case "ProgramLinkV2":
			if k, _ := fields["kind"].(string); !linkKinds[k] {
				t.Errorf("%s: link kind %q not in closed set", e.Name(), k)
			}
		case "ProvenanceV1":
			if st, _ := fields["source_type"].(string); !sourceTypes[st] {
				t.Errorf("%s: source_type %q not in closed set", e.Name(), st)
			}
		case "DecisionV1":
			if st, _ := fields["status"].(string); !decisionStatuses[st] {
				t.Errorf("%s: decision status %q not in closed set", e.Name(), st)
			}
		}
	}

	// Every canonical record must have a landed fixture — an absent kind is RED,
	// not a vacuous pass (this is the P0-gap coverage the wave promises).
	for schema := range required {
		if !seen[schema] {
			t.Errorf("no fixture for canonical schema %q", schema)
		}
	}
}
