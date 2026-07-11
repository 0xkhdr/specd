package core

import "testing"

func TestRecordFreshRevisionAndDigest(t *testing.T) {
	e := EvidenceEnvelopeV1{SubjectRevision: "abc", OutputDigest: "out1"}
	if !RecordFresh(e, FreshnessSubject{Revision: "abc", OutputDigest: "out1"}) {
		t.Fatal("matching subject should be fresh")
	}
	if RecordFresh(e, FreshnessSubject{Revision: "def"}) {
		t.Fatal("wrong revision should be stale")
	}
	if RecordFresh(e, FreshnessSubject{Revision: "abc", OutputDigest: "out2"}) {
		t.Fatal("wrong output digest should be stale")
	}
	if RecordFresh(EvidenceEnvelopeV1{SubjectRevision: "abc"}, FreshnessSubject{Revision: "abc", OutputDigest: "out1"}) {
		t.Fatal("missing configured output digest should be stale")
	}
	// digest the subject does not configure is not checked
	if !RecordFresh(e, FreshnessSubject{Revision: "abc"}) {
		t.Fatal("unconfigured digest must not invent a mismatch")
	}
}

func TestEvaluateQualityMissingStalePass(t *testing.T) {
	c := QualityContract{TaskID: "T1", Required: []EvidenceRequirement{
		{EvidenceClass: EvidenceTest, CheckID: "unit"},
		{EvidenceClass: EvidenceOutputEval, CheckID: "rubric"},
		{EvidenceClass: EvidenceReview, CheckID: "human"},
	}}
	subject := FreshnessSubject{Revision: "abc"}
	records := []EvidenceEnvelopeV1{
		{EvidenceClass: EvidenceTest, TaskID: "T1", CheckID: "unit", Verdict: EvalPass, SubjectRevision: "abc"},         // fresh pass
		{EvidenceClass: EvidenceOutputEval, TaskID: "T1", CheckID: "rubric", Verdict: EvalPass, SubjectRevision: "old"}, // stale
		{EvidenceClass: EvidenceReview, TaskID: "T1", CheckID: "human", Verdict: EvalFail, SubjectRevision: "abc"},      // failing -> missing
		{EvidenceClass: EvidenceReview, TaskID: "T2", CheckID: "human", Verdict: EvalPass, SubjectRevision: "abc"},      // wrong task -> missing
	}
	st := EvaluateQuality(c, records, subject)
	if len(st.Missing) != 1 || st.Missing[0].CheckID != "human" {
		t.Fatalf("missing = %+v", st.Missing)
	}
	if len(st.Stale) != 1 || st.Stale[0].CheckID != "rubric" {
		t.Fatalf("stale = %+v", st.Stale)
	}
	if st.OK() {
		t.Fatal("status should not be OK")
	}
}
