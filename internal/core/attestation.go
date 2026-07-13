package core

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const AttestationEnvelopeV1 = "attestation/v1"

const CIIdentitySchemaV1 = "ci-identity/v1"

// CIIdentityV1 contains only bounded identity claims needed to authorize a
// delivery. It deliberately carries no credential or provider token.
type CIIdentityV1 struct {
	Repository  string `json:"repository"`
	Environment string `json:"environment"`
	Audience    string `json:"audience"`
	Subject     string `json:"subject"`
	IssuedAt    string `json:"issued_at"`
	ExpiresAt   string `json:"expires_at"`
}

// CIIdentityEnvelopeV1 is transport-neutral. Key lookup is an explicit local
// allowlist, keeping verification offline and deterministic.
type CIIdentityEnvelopeV1 struct {
	SchemaVersion string       `json:"schema_version"`
	KeyID         string       `json:"key_id"`
	Claims        CIIdentityV1 `json:"claims"`
	Signature     string       `json:"signature"`
}

type CIIdentityExpectation struct {
	Repository  string
	Environment EnvironmentName
	Audience    string
	Now         time.Time
}

func SignCIIdentity(keyID string, key ed25519.PrivateKey, claims CIIdentityV1) (CIIdentityEnvelopeV1, error) {
	if strings.TrimSpace(keyID) == "" || len(key) != ed25519.PrivateKeySize {
		return CIIdentityEnvelopeV1{}, errors.New("CI identity requires key_id and Ed25519 private key")
	}
	if err := validateCIIdentityClaims(claims); err != nil {
		return CIIdentityEnvelopeV1{}, err
	}
	env := CIIdentityEnvelopeV1{SchemaVersion: CIIdentitySchemaV1, KeyID: keyID, Claims: claims}
	env.Signature = hex.EncodeToString(ed25519.Sign(key, canonicalCIIdentity(env)))
	return env, nil
}

func VerifyCIIdentity(env CIIdentityEnvelopeV1, keys map[string]ed25519.PublicKey, want CIIdentityExpectation) (CIIdentityV1, error) {
	if env.SchemaVersion != CIIdentitySchemaV1 {
		return CIIdentityV1{}, fmt.Errorf("unknown CI identity schema version %q", env.SchemaVersion)
	}
	key, ok := keys[env.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return CIIdentityV1{}, fmt.Errorf("CI identity key %q is not allowlisted", env.KeyID)
	}
	sig, err := hex.DecodeString(env.Signature)
	if err != nil || !ed25519.Verify(key, canonicalCIIdentity(env), sig) {
		return CIIdentityV1{}, errors.New("CI identity signature mismatch")
	}
	if err := validateCIIdentityClaims(env.Claims); err != nil {
		return CIIdentityV1{}, err
	}
	if env.Claims.Repository != want.Repository {
		return CIIdentityV1{}, errors.New("CI identity repository mismatch")
	}
	if env.Claims.Environment != string(want.Environment) {
		return CIIdentityV1{}, errors.New("CI identity environment mismatch")
	}
	if env.Claims.Audience != want.Audience {
		return CIIdentityV1{}, errors.New("CI identity audience mismatch")
	}
	issued, _ := time.Parse(time.RFC3339, env.Claims.IssuedAt)
	expires, _ := time.Parse(time.RFC3339, env.Claims.ExpiresAt)
	if want.Now.IsZero() || want.Now.Before(issued) || !want.Now.Before(expires) {
		return CIIdentityV1{}, errors.New("CI identity assertion expired or not yet valid")
	}
	return env.Claims, nil
}

func validateCIIdentityClaims(claims CIIdentityV1) error {
	if missing := firstEmpty("repository", claims.Repository, "environment", claims.Environment, "audience", claims.Audience, "subject", claims.Subject, "issued_at", claims.IssuedAt, "expires_at", claims.ExpiresAt); missing != "" {
		return fmt.Errorf("CI identity missing %s", missing)
	}
	if !EnvironmentName(claims.Environment).valid() {
		return fmt.Errorf("unknown environment %q", claims.Environment)
	}
	issued, err := time.Parse(time.RFC3339, claims.IssuedAt)
	if err != nil {
		return fmt.Errorf("invalid issued_at: %w", err)
	}
	expires, err := time.Parse(time.RFC3339, claims.ExpiresAt)
	if err != nil {
		return fmt.Errorf("invalid expires_at: %w", err)
	}
	if !expires.After(issued) {
		return errors.New("CI identity expires_at must follow issued_at")
	}
	return nil
}

func canonicalCIIdentity(env CIIdentityEnvelopeV1) []byte {
	b, _ := json.Marshal(struct {
		SchemaVersion string       `json:"schema_version"`
		KeyID         string       `json:"key_id"`
		Claims        CIIdentityV1 `json:"claims"`
	}{env.SchemaVersion, env.KeyID, env.Claims})
	return b
}

// CIDeliveryBindingV1 binds immutable source proof to one deployment attempt.
type CIDeliveryBindingV1 struct {
	SourceEvidenceDigest string          `json:"source_evidence_digest"`
	GitHead              string          `json:"git_head"`
	ArtifactDigest       string          `json:"artifact_digest"`
	SBOMRef              string          `json:"sbom_ref"`
	ProvenanceRef        string          `json:"provenance_ref"`
	Environment          EnvironmentName `json:"environment"`
	DeploymentID         string          `json:"deployment_id"`
	Attempt              int             `json:"attempt"`
}

func ValidateCIDeliveryBinding(candidate ReleaseCandidateV1, binding CIDeliveryBindingV1) error {
	if err := ValidateReleaseCandidate(candidate); err != nil {
		return err
	}
	if !binding.Environment.valid() {
		return fmt.Errorf("unknown environment %q", binding.Environment)
	}
	if missing := firstEmpty("source_evidence_digest", binding.SourceEvidenceDigest, "git_head", binding.GitHead, "artifact_digest", binding.ArtifactDigest, "sbom_ref", binding.SBOMRef, "provenance_ref", binding.ProvenanceRef, "deployment_id", binding.DeploymentID); missing != "" {
		return fmt.Errorf("CI delivery binding missing %s", missing)
	}
	if binding.Attempt < 1 {
		return errors.New("CI delivery binding attempt must be positive")
	}
	checks := []struct{ name, got, want string }{
		{"source evidence digest", binding.SourceEvidenceDigest, candidate.TaskEvidenceSetDigest},
		{"git HEAD", binding.GitHead, candidate.GitHead},
		{"artifact digest", binding.ArtifactDigest, candidate.ArtifactDigest},
		{"SBOM reference", binding.SBOMRef, candidate.SBOMRef},
		{"provenance reference", binding.ProvenanceRef, candidate.ProvenanceRef},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf("CI delivery %s mismatch", check.name)
		}
	}
	return nil
}

type CIDeliveryEvent struct {
	EventName                string
	Fork                     bool
	Environment              EnvironmentName
	HasProductionCredentials bool
}

func ValidateCIDeliveryEvent(event CIDeliveryEvent) error {
	if event.EventName == "pull_request" && event.Environment == EnvironmentProduction {
		return errors.New("pull_request cannot authorize production delivery")
	}
	if event.Fork && event.HasProductionCredentials {
		return errors.New("fork pull_request cannot receive production credentials")
	}
	return nil
}

// AttestedEnvelope is adapter-produced, transport-neutral billing evidence.
// Key material stays in operator config; core only validates local bytes.
type AttestedEnvelope struct {
	SchemaVersion  string          `json:"schema_version"`
	KeyID          string          `json:"key_id"`
	AttestationRef string          `json:"attestation_ref"`
	PayloadDigest  string          `json:"payload_digest"`
	Payload        json.RawMessage `json:"payload"`
	Signature      string          `json:"signature"`
}

func SignAttestation(keyID string, key []byte, ref string, telemetry Annotations) (AttestedEnvelope, error) {
	telemetry.EnvelopeVersion = TelemetryEnvelopeV1
	telemetry.Source = TelemetrySourceAdapter
	telemetry.AttestationRef = ref
	if keyID == "" || len(key) == 0 || ref == "" {
		return AttestedEnvelope{}, errors.New("attestation requires key_id, key, and attestation_ref")
	}
	if err := ValidateAnnotations(&telemetry); err != nil {
		return AttestedEnvelope{}, err
	}
	payload, err := json.Marshal(telemetry)
	if err != nil {
		return AttestedEnvelope{}, err
	}
	digest := sha256.Sum256(payload)
	env := AttestedEnvelope{SchemaVersion: AttestationEnvelopeV1, KeyID: keyID, AttestationRef: ref, PayloadDigest: "sha256:" + hex.EncodeToString(digest[:]), Payload: payload}
	env.Signature = signAttestation(env, key)
	return env, nil
}

func VerifyAttestation(env AttestedEnvelope, allowlistedKeys map[string][]byte) (Annotations, error) {
	if env.SchemaVersion != AttestationEnvelopeV1 {
		return Annotations{}, fmt.Errorf("unknown attestation schema version %q", env.SchemaVersion)
	}
	key, ok := allowlistedKeys[env.KeyID]
	if !ok || len(key) == 0 {
		return Annotations{}, fmt.Errorf("attestation key %q is not allowlisted", env.KeyID)
	}
	digest := sha256.Sum256(env.Payload)
	wantDigest := "sha256:" + hex.EncodeToString(digest[:])
	if !hmac.Equal([]byte(env.PayloadDigest), []byte(wantDigest)) {
		return Annotations{}, errors.New("attestation payload digest mismatch")
	}
	wantSig := signAttestation(env, key)
	if !hmac.Equal([]byte(env.Signature), []byte(wantSig)) {
		return Annotations{}, errors.New("attestation signature mismatch")
	}
	var telemetry Annotations
	if err := json.Unmarshal(env.Payload, &telemetry); err != nil {
		return Annotations{}, fmt.Errorf("decode attested telemetry: %w", err)
	}
	if telemetry.Source != TelemetrySourceAdapter || telemetry.AttestationRef == "" || telemetry.AttestationRef != env.AttestationRef {
		return Annotations{}, errors.New("attested telemetry provenance mismatch")
	}
	if err := ValidateAnnotations(&telemetry); err != nil {
		return Annotations{}, err
	}
	return telemetry, nil
}

func signAttestation(env AttestedEnvelope, key []byte) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s", env.SchemaVersion, env.KeyID, env.AttestationRef, env.PayloadDigest)
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil))
}
