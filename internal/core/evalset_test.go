package core

import "testing"

func TestEvalManifestDigestAndImmutability(t *testing.T) {
	m := EvalManifestV1{Kind: ManifestDataset, ID: "ds", Owner: "qa", Version: "1", Cases: []EvalCase{{ID: "critical", Labels: []string{"must-pass"}, Ref: "cases/1", Digest: "case-digest"}}, CriticalCases: []string{"critical"}, Redaction: "refs-only", Source: "git:abc", ReviewState: ReviewApproved, Repetitions: 2, Aggregation: AggregationMean, Threshold: .8}
	digest, err := EvalManifestDigest(m)
	if err != nil || digest == "" {
		t.Fatalf("digest=%q err=%v", digest, err)
	}
	m.Digest = digest
	if err := ValidateEvalManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Cases[0].Digest = "changed"
	if err := ValidateEvalManifest(m); err == nil || err.Error() != "EVAL_MANIFEST_DIGEST_MISMATCH" {
		t.Fatalf("edited manifest accepted: %v", err)
	}
}

func TestEvalManifestRejectsUnsafeOrIncompleteGovernance(t *testing.T) {
	m := EvalManifestV1{Kind: ManifestRubric, ID: "rb", Owner: "qa", Version: "1", Cases: []EvalCase{{ID: "c", Ref: "raw-body", Digest: "d"}}, Redaction: "inline", ReviewState: ReviewDraft, Repetitions: 0, Aggregation: AggregationMean}
	if err := ValidateEvalManifest(m); err == nil {
		t.Fatal("incomplete manifest accepted")
	}
}

func TestEvalEvidenceRejectsChangedManifest(t *testing.T) {
	dataset := EvalManifestV1{Kind: ManifestDataset, ID: "ds", Owner: "qa", Version: "1", Cases: []EvalCase{{ID: "c", Ref: "r", Digest: "d"}}, Redaction: "refs-only", Source: "git:abc", ReviewState: ReviewApproved, Repetitions: 1, Aggregation: AggregationMean, Threshold: .8}
	rubric := dataset
	rubric.Kind, rubric.ID = ManifestRubric, "rb"
	digest, _ := EvalManifestDigest(dataset)
	dataset.Digest = digest
	digest, _ = EvalManifestDigest(rubric)
	rubric.Digest = digest
	e := EvidenceEnvelopeV1{EvidenceClass: EvidenceOutputEval, DatasetDigest: dataset.Digest, RubricDigest: "old"}
	if err := ValidateEvalEvidenceManifests(e, dataset, rubric); err == nil || err.Error() != "EVAL_EVIDENCE_MANIFEST_STALE" {
		t.Fatalf("stale evidence accepted: %v", err)
	}
}
