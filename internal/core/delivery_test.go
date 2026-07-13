package core

import (
	"strings"
	"testing"
	"time"
)

func TestDeliveryEnvelope(t *testing.T) {
	release := validReleaseCandidate()
	if err := ValidateReleaseCandidate(release); err != nil {
		t.Fatalf("release: %v", err)
	}
	environment := EnvironmentV1{Schema: EnvironmentSchemaV1, Name: EnvironmentStaging, Strategy: "canary", RequiredAuthority: "ci", HealthCriteria: []string{"http_5xx"}, ObservationWindow: "30m", Freshness: "5m", RollbackTarget: "release-old"}
	if err := ValidateEnvironment(environment); err != nil {
		t.Fatalf("environment: %v", err)
	}
	deployment := validDeployment()
	if err := ValidateDeployment(deployment); err != nil {
		t.Fatalf("deployment: %v", err)
	}
	health := validHealthObservation()
	if err := ValidateHealthObservation(health); err != nil {
		t.Fatalf("health: %v", err)
	}
	rollback := RollbackV1{Schema: RollbackSchemaV1, DeploymentID: deployment.DeploymentID, RollbackTarget: "release-old", Reason: "failed criterion", Adapter: "ci", ActionResult: "issued", CapabilityClass: "reversible"}
	if err := ValidateRollback(rollback); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	release.Schema = "ReleaseCandidateV2"
	if err := ValidateReleaseCandidate(release); err == nil || !strings.Contains(err.Error(), "unknown release schema") {
		t.Fatalf("unknown schema accepted: %v", err)
	}
	deployment.Environment = EnvironmentName("preview")
	if err := ValidateDeployment(deployment); err == nil || !strings.Contains(err.Error(), "unknown environment") {
		t.Fatalf("unknown environment accepted: %v", err)
	}
}

func TestDeliveryTransition(t *testing.T) {
	now := time.Date(2026, 7, 11, 9, 22, 0, 0, time.UTC)
	base := DeliveryTransition{Release: validReleaseCandidate(), Deployment: validDeployment(), Now: now}

	valid := []struct{ from, to DeploymentStatus }{
		{StatusRequested, StatusStarted},
		{StatusStarted, StatusObserving},
		{StatusObserving, StatusHealthy},
		{StatusObserving, StatusFailed},
		{StatusFailed, StatusRollingBack},
		{StatusRollingBack, StatusRolledBack},
	}
	for _, tc := range valid {
		tr := base
		tr.From, tr.To = tc.from, tc.to
		if tc.to == StatusHealthy {
			h := validHealthObservation()
			tr.Health = &h
		}
		if tc.to == StatusFailed || tc.from == StatusFailed || tc.from == StatusRollingBack {
			tr.RollbackTarget = "release-old"
		}
		if err := ValidateDeliveryTransition(tr); err != nil {
			t.Errorf("%s -> %s: %v", tc.from, tc.to, err)
		}
	}

	bad := base
	bad.From, bad.To = StatusRequested, StatusHealthy
	if err := ValidateDeliveryTransition(bad); err == nil {
		t.Fatal("state jump accepted")
	}
	bad.From, bad.To = DeploymentStatus("mystery"), StatusStarted
	if err := ValidateDeliveryTransition(bad); err == nil {
		t.Fatal("unknown status accepted")
	}
	bad = base
	bad.From, bad.To = StatusObserving, StatusHealthy
	if err := ValidateDeliveryTransition(bad); err == nil || !strings.Contains(err.Error(), "health observation required") {
		t.Fatalf("missing health accepted: %v", err)
	}
	health := validHealthObservation()
	health.Freshness.ObservedAt = "2026-07-11T09:00:00Z"
	bad.Health = &health
	if err := ValidateDeliveryTransition(bad); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale health accepted: %v", err)
	}
	bad = base
	bad.From, bad.To = StatusObserving, StatusFailed
	if err := ValidateDeliveryTransition(bad); err == nil || !strings.Contains(err.Error(), "rollback target required") {
		t.Fatalf("missing rollback target accepted: %v", err)
	}
	bad = base
	bad.From, bad.To = StatusStarted, StatusObserving
	bad.Deployment.ReleaseID = "wrong"
	if err := ValidateDeliveryTransition(bad); err == nil || !strings.Contains(err.Error(), "release identity mismatch") {
		t.Fatalf("release mismatch accepted: %v", err)
	}
	bad = base
	bad.From, bad.To = StatusStarted, StatusObserving
	bad.Deployment.GitHead = strings.Repeat("b", 40)
	if err := ValidateDeliveryTransition(bad); err == nil || !strings.Contains(err.Error(), "git HEAD mismatch") {
		t.Fatalf("HEAD mismatch accepted: %v", err)
	}
	bad = base
	bad.From, bad.To = StatusStarted, StatusObserving
	bad.Deployment.ArtifactDigest = "sha256:" + strings.Repeat("b", 64)
	if err := ValidateDeliveryTransition(bad); err == nil || !strings.Contains(err.Error(), "artifact digest mismatch") {
		t.Fatalf("artifact mismatch accepted: %v", err)
	}
}

func validReleaseCandidate() ReleaseCandidateV1 {
	return ReleaseCandidateV1{Schema: ReleaseCandidateSchemaV1, ReleaseID: "release-1", SpecID: "demo", SpecRevision: 1, GitHead: strings.Repeat("a", 40), TaskEvidenceSetDigest: "sha256:" + strings.Repeat("1", 64), ArtifactDigest: "sha256:" + strings.Repeat("2", 64), SBOMRef: "sbom.json", ProvenanceRef: "provenance.json", BootstrapDigest: "sha256:" + strings.Repeat("3", 64), StateSchema: "1", CreatedAt: "2026-07-11T09:00:00Z"}
}

func validDeployment() DeploymentV1 {
	r := validReleaseCandidate()
	return DeploymentV1{Schema: DeploymentSchemaV1, DeploymentID: "dep-1", Attempt: 1, ReleaseID: r.ReleaseID, GitHead: r.GitHead, ArtifactDigest: r.ArtifactDigest, Environment: EnvironmentStaging, Status: StatusObserving, Strategy: "canary", Population: "5%", Window: "30m", Adapter: "ci", Authority: "ci", Actor: "bot", IdempotencyKey: "key-1", StartedAt: "2026-07-11T09:05:00Z", TelemetrySource: "metrics", EvidenceRef: "evidence.json", AttestationRef: "attestation.json"}
}

func validHealthObservation() HealthObservationV1 {
	r := validReleaseCandidate()
	return HealthObservationV1{Schema: HealthObservationSchemaV1, DeploymentID: "dep-1", CriterionID: "http_5xx", HealthCheck: "rate", Threshold: "<0.01", Observation: "0.002", Freshness: ObservationFreshness{ObservedAt: "2026-07-11T09:20:00Z", MaxAge: "5m"}, ReleaseIdentity: DeliveryIdentity{ReleaseID: r.ReleaseID, ArtifactDigest: r.ArtifactDigest, Environment: EnvironmentStaging}, Source: "metrics"}
}

func TestCanaryWindow(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	deployment := validDeployment()
	deployment.Status = StatusObserving
	deployment.StartedAt = now.Add(-10 * time.Minute).Format(time.RFC3339)
	deployment.Window = "10m"
	observations := []HealthObservationV1{validHealthObservation()}
	observations[0].Freshness.ObservedAt = now.Add(-9 * time.Minute).Format(time.RFC3339)
	observations[0].Freshness.MaxAge = "15m"
	observations[0].Freshness.WindowStartedAt = now.Add(-10 * time.Minute).Format(time.RFC3339)
	if err := ValidateCanaryWindow(deployment, observations, now); err != nil {
		t.Fatalf("full window rejected: %v", err)
	}
	deployment.StartedAt = now.Add(-9 * time.Minute).Format(time.RFC3339)
	if err := ValidateCanaryWindow(deployment, observations, now); err == nil {
		t.Fatal("partial window accepted")
	}
	deployment.StartedAt = now.Add(-10 * time.Minute).Format(time.RFC3339)
	observations[0].ReleaseIdentity.ArtifactDigest = "sha256:wrong"
	if err := ValidateCanaryWindow(deployment, observations, now); err == nil {
		t.Fatal("wrong-artifact observation accepted")
	}
}

func TestRollbackComplete(t *testing.T) {
	r := RollbackV1{Schema: RollbackSchemaV2, DeploymentID: "dep-1", FailedReleaseID: "rel-bad",
		RollbackTarget: "rel-good", Reason: "error budget", Adapter: "deploy/v1", AdapterIdentity: "attested:ci",
		ActionResult: "succeeded", CapabilityClass: RollbackCapabilityAutomatic,
		PostRollbackHealth: RollbackHealth{CriterionID: "health", Observation: "pass", ObservedAt: "2026-07-13T12:00:00Z"}}
	if err := ValidateRollback(r); err != nil {
		t.Fatalf("complete rollback rejected: %v", err)
	}
	r.PostRollbackHealth = RollbackHealth{}
	if err := ValidateRollback(r); err == nil {
		t.Fatal("rollback completed without target health")
	}
	r.CapabilityClass = RollbackCapabilityHumanRequired
	r.HumanRequired = false
	if err := ValidateRollback(r); err == nil {
		t.Fatal("human-required strategy accepted without human requirement")
	}
}
