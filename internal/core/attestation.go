package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

const AttestationEnvelopeV1 = "attestation/v1"

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
