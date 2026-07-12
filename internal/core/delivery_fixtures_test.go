package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDeliveryFixture pins the delivery contract (docs/delivery-contract.md) against
// the canonical offline fixtures under testdata/delivery. 08a validates structure only:
// every fixture decodes and carries the required, non-empty fields for its schema, and a
// DeploymentV1 status is a member of the closed set. The full state-machine validation
// (illegal jumps, digest/HEAD mismatch, stale observation) lands with internal/core/delivery.go
// in 08d/08g (T18), which extends this test rather than redefining the fields.
func TestDeliveryFixture(t *testing.T) {
	// required[schema] = contract fields that must be present and non-empty.
	required := map[string][]string{
		"ReleaseCandidateV1":  {"release_id", "spec_id", "git_head", "artifact_digest", "bootstrap_digest", "state_schema"},
		"DeploymentV1":        {"deployment_id", "release_id", "environment", "status", "idempotency_key"},
		"HealthObservationV1": {"deployment_id", "criterion_id", "freshness", "source"},
		"RollbackV1":          {"deployment_id", "rollback_target", "capability_class"},
	}
	deploymentStatuses := map[string]bool{
		"requested": true, "started": true, "observing": true, "healthy": true,
		"failed": true, "rolling_back": true, "rolled_back": true,
	}

	dir := filepath.Join("..", "..", "testdata", "delivery")
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
		if schema == "DeploymentV1" {
			if st, _ := fields["status"].(string); !deploymentStatuses[st] {
				t.Errorf("%s: status %q not in closed set", e.Name(), st)
			}
		}
		switch schema {
		case ReleaseCandidateSchemaV1:
			var envelope ReleaseCandidateV1
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Errorf("%s: decode typed release: %v", e.Name(), err)
			} else if err := ValidateReleaseCandidate(envelope); err != nil {
				t.Errorf("%s: validate typed release: %v", e.Name(), err)
			}
		case DeploymentSchemaV1:
			var envelope DeploymentV1
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Errorf("%s: decode typed deployment: %v", e.Name(), err)
			} else if err := ValidateDeployment(envelope); err != nil {
				t.Errorf("%s: validate typed deployment: %v", e.Name(), err)
			}
		case HealthObservationSchemaV1:
			var envelope HealthObservationV1
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Errorf("%s: decode typed health observation: %v", e.Name(), err)
			} else if err := ValidateHealthObservation(envelope); err != nil {
				t.Errorf("%s: validate typed health observation: %v", e.Name(), err)
			}
		case RollbackSchemaV1:
			var envelope RollbackV1
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Errorf("%s: decode typed rollback: %v", e.Name(), err)
			} else if err := ValidateRollback(envelope); err != nil {
				t.Errorf("%s: validate typed rollback: %v", e.Name(), err)
			}
		}
	}

	// Every canonical envelope must have a landed fixture — an absent kind is RED,
	// not a vacuous pass (this is the P0-gap coverage the wave promises).
	for schema := range required {
		if !seen[schema] {
			t.Errorf("no fixture for canonical schema %q", schema)
		}
	}
}
