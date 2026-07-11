package core

import (
	"encoding/json"
	"testing"
)

func validOutputEnvelope(id, check string) EvidenceEnvelopeV1 {
	return EvidenceEnvelopeV1{
		SchemaVersion: EvalSchemaVersion, EvidenceID: id, EvidenceClass: EvidenceOutputEval,
		SpecSlug: "demo", TaskID: "T1", RunID: "r1", Attempt: 1, SubjectRevision: "abc",
		Producer: "ci", ProducerVersion: "1", ConfigDigest: "cfg", CheckID: check,
		Verdict: EvalPass, CreatedAt: "2026-01-01T00:00:00Z", Actor: "ci",
		ArtifactRef: "evals/" + id + ".json", ArtifactDigest: "sha",
		DatasetDigest: "ds", RubricDigest: "rb", OutputDigest: "out",
	}
}

func marshalJSONL(t *testing.T, envs ...EvidenceEnvelopeV1) []byte {
	t.Helper()
	var out []byte
	for _, e := range envs {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

func TestEvalImportAcceptsAndPins(t *testing.T) {
	raw := marshalJSONL(t, validOutputEnvelope("e1", "rubric-v1"))
	records, findings := ImportEvals(raw, ImportExpect{SpecSlug: "demo", TaskID: "T1", CheckIDs: []string{"rubric-v1"}})
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
	if len(records) != 1 || records[0].EvidenceID != "e1" {
		t.Fatalf("records = %+v", records)
	}
}

func TestEvalImportRejectsStableOrdered(t *testing.T) {
	good := validOutputEnvelope("e1", "rubric-v1")
	wrongTask := validOutputEnvelope("e2", "rubric-v1")
	wrongTask.TaskID = "T9"
	dup := validOutputEnvelope("e1", "rubric-v1") // duplicate id of good
	raw := marshalJSONL(t, good, wrongTask, dup)
	records, findings := ImportEvals(raw, ImportExpect{SpecSlug: "demo", TaskID: "T1"})
	if records != nil {
		t.Fatal("records leaked despite findings")
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %+v", findings)
	}
	if findings[0].Index != 1 || findings[0].Code != "EVAL_IMPORT_TASK_MISMATCH" {
		t.Fatalf("finding[0] = %+v", findings[0])
	}
	if findings[1].Index != 2 || findings[1].Code != "EVAL_IMPORT_DUPLICATE" {
		t.Fatalf("finding[1] = %+v", findings[1])
	}
}

func TestEvalImportMalformedAndDigest(t *testing.T) {
	if _, f := ImportEvals([]byte("{not json}\n"), ImportExpect{}); len(f) != 1 || f[0].Code != "EVAL_IMPORT_MALFORMED" {
		t.Fatalf("malformed = %+v", f)
	}
	unknown := marshalJSONL(t, validOutputEnvelope("unknown", "c1"))
	unknown = append(unknown[:len(unknown)-2], []byte(`,"raw_result":"secret"}`+"\n")...)
	if _, f := ImportEvals(unknown, ImportExpect{}); len(f) != 1 || f[0].Code != "EVAL_IMPORT_MALFORMED" {
		t.Fatalf("unknown field = %+v", f)
	}
	env := validOutputEnvelope("e1", "c1")
	env.ArtifactDigest = Digest([]byte("real"))
	raw := marshalJSONL(t, env)
	// wrong artifact bytes -> digest mismatch
	_, f := ImportEvals(raw, ImportExpect{Artifacts: map[string][]byte{env.ArtifactRef: []byte("tampered")}})
	if len(f) != 1 || f[0].Code != "EVAL_IMPORT_DIGEST_MISMATCH" {
		t.Fatalf("digest = %+v", f)
	}
	// matching artifact bytes -> accepted
	if r, f := ImportEvals(raw, ImportExpect{Artifacts: map[string][]byte{env.ArtifactRef: []byte("real")}}); len(f) != 0 || len(r) != 1 {
		t.Fatalf("match r=%+v f=%+v", r, f)
	}
}

func TestEvalImportTraceDigest(t *testing.T) {
	env := validOutputEnvelope("e1", "c1")
	env.EvidenceClass = EvidenceTrajectoryEval
	env.DatasetDigest, env.RubricDigest, env.OutputDigest = "", "", ""
	trace := []byte(`{"run_id":"r1","event_id":"a","seq":1,"tool":"read"}`)
	env.TraceDigest = Digest(trace)
	raw := marshalJSONL(t, env)
	if _, f := ImportEvals(raw, ImportExpect{Traces: map[string][]byte{env.ArtifactRef: []byte("other")}}); len(f) != 1 || f[0].Code != "EVAL_IMPORT_TRACE_MISMATCH" {
		t.Fatalf("trace mismatch = %+v", f)
	}
	if r, f := ImportEvals(raw, ImportExpect{Traces: map[string][]byte{env.ArtifactRef: trace}}); len(f) != 0 || len(r) != 1 {
		t.Fatalf("trace match r=%+v f=%+v", r, f)
	}
}
