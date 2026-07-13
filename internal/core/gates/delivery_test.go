package gates

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// deliveryFixture builds a fully valid production delivery: policy, release
// candidate, deployment, and one fresh matching health observation. Tests mutate
// a copy to exercise each fail-closed rule.
func deliveryFixture() DeliveryInput {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	digest := "sha256:aaaa"
	return DeliveryInput{
		Policy: core.EnvironmentV1{
			Schema: core.EnvironmentSchemaV1, Name: core.EnvironmentProduction, Strategy: "canary",
			RequiredApprover: "release-manager", RequiredAuthority: "oncall", HealthCriteria: []string{"health"},
			ObservationWindow: "10m", Freshness: "5m", RollbackTarget: "previous",
		},
		Release: core.ReleaseCandidateV1{
			Schema: core.ReleaseCandidateSchemaV1, ReleaseID: "rel-1", ArtifactDigest: digest, GitHead: "head1",
		},
		Deployment: core.DeploymentV1{
			Schema: core.DeploymentSchemaV1, ReleaseID: "rel-1", Environment: core.EnvironmentProduction,
			ArtifactDigest: digest, GitHead: "head1", Adapter: "reference", Authority: "oncall", Status: core.StatusObserving,
		},
		Observations: []core.HealthObservationV1{{
			ReleaseIdentity: core.DeliveryIdentity{ReleaseID: "rel-1", ArtifactDigest: digest, Environment: core.EnvironmentProduction},
			Freshness:       core.ObservationFreshness{ObservedAt: now.Add(-1 * time.Minute).Format(time.RFC3339), MaxAge: "5m"},
		}},
		Now: now,
	}
}

// TestDeliveryGate pins R7.1: the delivery gate is a pure function of on-disk
// policy + evidence. The same input always yields the same verdict, and a fully
// valid production delivery passes.
func TestDeliveryGate(t *testing.T) {
	in := deliveryFixture()
	first := delivery(in)
	second := delivery(in)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic verdict: %#v vs %#v", first, second)
	}
	if HasErrors(first) {
		t.Fatalf("valid production delivery failed: %#v", first)
	}
}

// TestProductionRequires pins R7.2: production requires an explicit
// adapter/authority, artifact identity, fresh observation, and a rollback
// target; lower environments may opt out without those controls.
func TestProductionRequires(t *testing.T) {
	cases := map[string]func(*DeliveryInput){
		"no-adapter":   func(in *DeliveryInput) { in.Deployment.Adapter = "" },
		"no-authority": func(in *DeliveryInput) { in.Deployment.Authority = "" },
		"no-artifact":  func(in *DeliveryInput) { in.Deployment.ArtifactDigest = ""; in.Release.ArtifactDigest = "" },
		"no-rollback":  func(in *DeliveryInput) { in.Policy.RollbackTarget = "" },
		"stale-obs":    func(in *DeliveryInput) { in.Observations = nil },
	}
	for name, mut := range cases {
		in := deliveryFixture()
		mut(&in)
		if !HasErrors(delivery(in)) {
			t.Fatalf("%s: production delivery accepted, want fail-closed", name)
		}
	}

	// A lower environment opts out: staging without adapter/authority/observation
	// is not required to carry production controls.
	in := deliveryFixture()
	in.Deployment.Environment = core.EnvironmentStaging
	in.Policy.Name = core.EnvironmentStaging
	in.Deployment.Adapter = ""
	in.Deployment.Authority = ""
	in.Observations = nil
	if got := delivery(in); HasErrors(got) {
		t.Fatalf("staging delivery must not require production controls: %#v", got)
	}
}

// TestArtifactSubstitution pins R7.3: an artifact swapped after candidate
// creation fails the digest check.
func TestArtifactSubstitution(t *testing.T) {
	in := deliveryFixture()
	in.Deployment.ArtifactDigest = "sha256:evil"
	findings := delivery(in)
	if !HasErrors(findings) {
		t.Fatal("swapped artifact accepted, want digest failure")
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "artifact digest") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no artifact digest finding: %#v", findings)
	}
}

func TestObservationStale(t *testing.T) {
	in := deliveryFixture()
	in.Policy.HealthCriteria = []string{"health", "latency"}
	in.Deployment.StartedAt = in.Now.Add(-10 * time.Minute).Format(time.RFC3339)
	in.Deployment.Window = "10m"
	in.Deployment.DeploymentID = "dep-1"
	in.Observations[0].Schema = core.HealthObservationSchemaV1
	in.Observations[0].DeploymentID = in.Deployment.DeploymentID
	in.Observations[0].CriterionID = "health"
	in.Observations[0].HealthCheck = "http"
	in.Observations[0].Threshold = "pass"
	in.Observations[0].Observation = "pass"
	in.Observations[0].Source = "reference"
	in.Observations[0].Freshness.WindowStartedAt = in.Now.Add(-10 * time.Minute).Format(time.RFC3339)
	verdict := CanaryVerdict(in, []string{"reference"})
	if verdict.Healthy || len(verdict.Findings) == 0 {
		t.Fatal("missing criterion promoted")
	}
	in.Observations = append(in.Observations, in.Observations[0])
	in.Observations[1].CriterionID = "latency"
	if got := CanaryVerdict(in, []string{"reference"}); !got.Healthy {
		t.Fatalf("complete observations failed: %#v", got)
	}
	in.Observations[1].Freshness.ObservedAt = in.Now.Add(-6 * time.Minute).Format(time.RFC3339)
	if CanaryVerdict(in, []string{"reference"}).Healthy {
		t.Fatal("stale observation promoted")
	}
	in.Observations[1].Freshness.ObservedAt = in.Now.Add(-1 * time.Minute).Format(time.RFC3339)
	in.Observations[1].Source = "prompt"
	if CanaryVerdict(in, []string{"reference"}).Healthy {
		t.Fatal("unallowlisted source promoted")
	}
}
