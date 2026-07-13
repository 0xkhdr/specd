package core

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAttestationAcceptsAllowlistedSignedEnvelopeAndRejectsTampering(t *testing.T) {
	ann := Annotations{EnvelopeVersion: TelemetryEnvelopeV1, Source: TelemetrySourceAdapter, InputTokens: 10, OutputTokens: 2, Cost: "0.125", Currency: "USD", PricingRef: "price:v1", Provider: "example", Model: "small"}
	env, err := SignAttestation("billing-key", []byte("offline-test-key"), "usage:run-1:attempt-1", ann)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyAttestation(env, map[string][]byte{"billing-key": []byte("offline-test-key")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != TelemetrySourceAdapter || got.AttestationRef != "usage:run-1:attempt-1" {
		t.Fatalf("accepted telemetry = %#v", got)
	}

	tampered := env
	if err := json.Unmarshal(env.Payload, &ann); err != nil {
		t.Fatal(err)
	}
	ann.InputTokens++
	tampered.Payload, _ = json.Marshal(ann)
	if _, err := VerifyAttestation(tampered, map[string][]byte{"billing-key": []byte("offline-test-key")}); err == nil {
		t.Fatal("tampered payload accepted")
	}
	if _, err := VerifyAttestation(env, map[string][]byte{"other": []byte("offline-test-key")}); err == nil {
		t.Fatal("non-allowlisted key accepted")
	}
}

func TestAttestationOffline(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	claims := CIIdentityV1{
		Repository: "0xkhdr/specd", Environment: "production", Audience: "specd-delivery",
		Subject: "repo:0xkhdr/specd:environment:production", IssuedAt: now.Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt: now.Add(time.Minute).Format(time.RFC3339),
	}
	env, err := SignCIIdentity("release-key", priv, claims)
	if err != nil {
		t.Fatal(err)
	}
	want := CIIdentityExpectation{Repository: claims.Repository, Environment: EnvironmentProduction, Audience: claims.Audience, Now: now}
	if _, err := VerifyCIIdentity(env, map[string]ed25519.PublicKey{"release-key": pub}, want); err != nil {
		t.Fatal(err)
	}

	cases := map[string]func(*CIIdentityEnvelopeV1, *CIIdentityExpectation){
		"tampered":          func(e *CIIdentityEnvelopeV1, _ *CIIdentityExpectation) { e.Claims.Repository = "attacker/fork" },
		"wrong_repository":  func(_ *CIIdentityEnvelopeV1, w *CIIdentityExpectation) { w.Repository = "other/repo" },
		"wrong_environment": func(_ *CIIdentityEnvelopeV1, w *CIIdentityExpectation) { w.Environment = EnvironmentStaging },
		"wrong_audience":    func(_ *CIIdentityEnvelopeV1, w *CIIdentityExpectation) { w.Audience = "other" },
		"expired":           func(_ *CIIdentityEnvelopeV1, w *CIIdentityExpectation) { w.Now = now.Add(2 * time.Minute) },
		"untrusted_key":     func(e *CIIdentityEnvelopeV1, _ *CIIdentityExpectation) { e.KeyID = "unknown" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			bad, expectation := env, want
			mutate(&bad, &expectation)
			if _, err := VerifyCIIdentity(bad, map[string]ed25519.PublicKey{"release-key": pub}, expectation); err == nil {
				t.Fatal("invalid CI identity accepted")
			}
		})
	}
}

func TestCIDeliveryBinding(t *testing.T) {
	candidate := ReleaseCandidateV1{Schema: ReleaseCandidateSchemaV1, ReleaseID: "rel-1", SpecID: "spec", SpecRevision: 1, GitHead: "abc", TaskEvidenceSetDigest: "sha256:evidence", ArtifactDigest: "sha256:artifact", SBOMRef: "sha256:sbom", ProvenanceRef: "sha256:provenance", BootstrapDigest: "sha256:bootstrap", StateSchema: "2", CreatedAt: "2026-07-13T11:00:00Z"}
	binding := CIDeliveryBindingV1{SourceEvidenceDigest: "sha256:evidence", GitHead: "abc", ArtifactDigest: "sha256:artifact", SBOMRef: "sha256:sbom", ProvenanceRef: "sha256:provenance", Environment: EnvironmentProduction, DeploymentID: "dep-1", Attempt: 1}
	if err := ValidateCIDeliveryBinding(candidate, binding); err != nil {
		t.Fatal(err)
	}

	swapped := binding
	swapped.ArtifactDigest = "sha256:swapped"
	if err := ValidateCIDeliveryBinding(candidate, swapped); err == nil || !strings.Contains(err.Error(), "artifact digest mismatch") {
		t.Fatalf("artifact swap error = %v", err)
	}
	if err := ValidateCIDeliveryEvent(CIDeliveryEvent{EventName: "pull_request", Fork: false, Environment: EnvironmentProduction, HasProductionCredentials: true}); err == nil {
		t.Fatal("PR impersonated production delivery")
	}
	if err := ValidateCIDeliveryEvent(CIDeliveryEvent{EventName: "pull_request", Fork: true, Environment: EnvironmentStaging, HasProductionCredentials: true}); err == nil {
		t.Fatal("fork PR received production credentials")
	}
}
