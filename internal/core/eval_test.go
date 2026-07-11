package core

import "testing"

func TestEvalEnvelopeValidateAndDigest(t *testing.T) {
	e := EvidenceEnvelopeV1{SchemaVersion: EvalSchemaVersion, EvidenceID: "e1", EvidenceClass: EvidenceOutputEval, SpecSlug: "demo", TaskID: "T1", RunID: "r1", Attempt: 1, SubjectRevision: "abc", Producer: "ci", ProducerVersion: "1", ConfigDigest: "cfg", CheckID: "rubric-v1", Verdict: EvalPass, CreatedAt: "2026-01-01T00:00:00Z", Actor: "ci", ArtifactRef: "evals/e1.json", ArtifactDigest: "sha", DatasetDigest: "ds", RubricDigest: "rb", OutputDigest: "out"}
	if err := ValidateEvidenceEnvelope(e); err != nil {
		t.Fatal(err)
	}
	d := EvidenceEnvelopeDigest(e)
	e.ArtifactDigest = "other"
	if EvidenceEnvelopeDigest(e) == d {
		t.Fatal("digest ignored semantic field")
	}
}

func TestEvalEnvelopeFailsUnknownAndMissing(t *testing.T) {
	e := EvidenceEnvelopeV1{SchemaVersion: EvalSchemaVersion, EvidenceClass: EvidenceClass("vibes")}
	if err := ValidateEvidenceEnvelope(e); err == nil {
		t.Fatal("unknown class accepted")
	}
	e.EvidenceClass = EvidenceTest
	if err := ValidateEvidenceEnvelope(e); err == nil {
		t.Fatal("missing identity accepted")
	}
}

func TestEvidenceLegacyAdapterIsTestOnly(t *testing.T) {
	e := AdaptLegacyVerify("demo", EvidenceRecord{TaskID: "T1", Command: "go test ./...", ExitCode: 0, GitHead: "abc"})
	if e.EvidenceClass != EvidenceTest || e.Verdict != EvalPass {
		t.Fatalf("legacy adapter = %+v", e)
	}
}
