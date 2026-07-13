package core

import (
	"encoding/json"
	"testing"
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
