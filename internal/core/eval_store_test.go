package core

import (
	"path/filepath"
	"testing"
)

func TestEvalStoreAppendAndDuplicate(t *testing.T) {
	root := t.TempDir()
	path := EvalStorePath(root, "demo")
	e := EvidenceEnvelopeV1{SchemaVersion: EvalSchemaVersion, EvidenceID: "e1", EvidenceClass: EvidenceTest, SpecSlug: "demo", TaskID: "T1", RunID: "r1", Attempt: 1, SubjectRevision: "abc", Producer: "specd", ProducerVersion: "1", ConfigDigest: "cfg", CheckID: "unit", Verdict: EvalPass, CreatedAt: "2026-01-01T00:00:00Z", Actor: "ci", ArtifactRef: "verify", ArtifactDigest: "sha"}
	if err := AppendEval(path, e); err != nil {
		t.Fatal(err)
	}
	if err := AppendEval(path, e); err == nil {
		t.Fatal("duplicate evidence identity accepted")
	}
	got, err := LoadEvals(path)
	if err != nil || len(got) != 1 || got[0].EvidenceID != "e1" {
		t.Fatalf("load = %+v, %v", got, err)
	}
}

func TestEvalStorePaths(t *testing.T) {
	root := "/repo"
	if got := EvalStorePath(root, "demo"); got != filepath.Join(root, ".specd", "specs", "demo", "evals", "records.jsonl") {
		t.Fatalf("store path = %s", got)
	}
	if got := EvalTracePath(root, "demo", "r1"); got != filepath.Join(root, ".specd", "specs", "demo", "evals", "traces", "r1.jsonl") {
		t.Fatalf("trace path = %s", got)
	}
}
