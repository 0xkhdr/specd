package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func validReport(t *testing.T) []byte {
	t.Helper()
	record := core.EvidenceEnvelopeV1{
		SchemaVersion:   core.EvalSchemaVersion,
		EvidenceID:      "evidence-1",
		EvidenceClass:   core.EvidenceTest,
		SpecSlug:        "demo",
		TaskID:          "T1",
		RunID:           "run-1",
		Attempt:         1,
		SubjectRevision: strings.Repeat("a", 40),
		Producer:        core.VerifyProducer,
		ProducerVersion: "test",
		ConfigDigest:    "config",
		CheckID:         "unit",
		Verdict:         core.EvalPass,
		CreatedAt:       time.Unix(0, 0).UTC().Format(time.RFC3339),
		Actor:           core.VerifyProducer,
		ArtifactRef:     "evidence.jsonl#T1",
		ArtifactDigest:  "artifact",
	}
	raw, err := json.MarshalIndent(statusReport{
		SchemaVersion: core.EvalSchemaVersion,
		Count:         1,
		Records:       []core.EvidenceEnvelopeV1{record},
	}, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestValidateAcceptsStructuralJSON(t *testing.T) {
	if err := validate(bytes.NewReader(validReport(t)), "demo", "T1", "unit"); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateRejectsWrongProducer(t *testing.T) {
	raw := bytes.Replace(validReport(t), []byte(core.VerifyProducer), []byte("external-eval"), 1)
	err := validate(bytes.NewReader(raw), "demo", "T1", "unit")
	if err == nil || !strings.Contains(err.Error(), "producer") {
		t.Fatalf("error = %v, want producer mismatch", err)
	}
}
