package core

import (
	"strings"
	"testing"
)

// TestRejectUnknownHead asserts evidence that cannot be pinned to a commit
// (empty or "unknown" git_head) is refused as completion evidence, and the
// error names the re-verify remedy (R3.2, R3.3).
func TestRejectUnknownHead(t *testing.T) {
	raw := []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | scout | a.go | - | true | ok |\n")
	for _, head := range []string{"", "unknown"} {
		_, err := CompleteTask(raw, "T1", map[string]EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: head}})
		if err == nil {
			t.Fatalf("git_head %q accepted as completion evidence", head)
		}
		if !strings.Contains(err.Error(), "specd verify") {
			t.Fatalf("error does not name remedy: %v", err)
		}
	}
	if _, err := CompleteTask(raw, "T1", map[string]EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc123"}}); err != nil {
		t.Fatalf("pinned evidence rejected: %v", err)
	}
}

func TestCompleteRequiresEvidence(t *testing.T) {
	raw := []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | go test ./... | ok |\n")
	if _, err := CompleteTask(raw, "T1", nil); err == nil {
		t.Fatalf("CompleteTask without evidence succeeded")
	}
	got, err := CompleteTask(raw, "T1", map[string]EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc"}})
	if err != nil {
		t.Fatalf("CompleteTask with evidence: %v", err)
	}
	if string(got) == string(raw) {
		t.Fatalf("task row was not updated")
	}
}

func TestCompleteTaskWithQualityMissingStaleAndTestNoBypass(t *testing.T) {
	raw := []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | go test ./... | ok |\n")
	pass := map[string]EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc"}}
	contract := QualityContract{TaskID: "T1", Required: []EvidenceRequirement{{EvidenceClass: EvidenceOutputEval, CheckID: "rubric"}}}
	subject := FreshnessSubject{Revision: "abc"}

	// missing required eval -> refused even though verify passed
	if _, err := CompleteTaskWithQuality(raw, "T1", pass, contract, nil, subject); err == nil || !strings.Contains(err.Error(), "EVIDENCE_MISSING") {
		t.Fatalf("missing eval accepted: %v", err)
	}
	// present but stale -> EVIDENCE_STALE
	stale := []EvidenceEnvelopeV1{{EvidenceClass: EvidenceOutputEval, TaskID: "T1", CheckID: "rubric", Verdict: EvalPass, SubjectRevision: "old"}}
	if _, err := CompleteTaskWithQuality(raw, "T1", pass, contract, stale, subject); err == nil || !strings.Contains(err.Error(), "EVIDENCE_STALE") {
		t.Fatalf("stale eval accepted: %v", err)
	}
	// fresh pass -> completes
	fresh := []EvidenceEnvelopeV1{{EvidenceClass: EvidenceOutputEval, TaskID: "T1", CheckID: "rubric", Verdict: EvalPass, SubjectRevision: "abc"}}
	if _, err := CompleteTaskWithQuality(raw, "T1", pass, contract, fresh, subject); err != nil {
		t.Fatalf("fresh eval refused: %v", err)
	}
	// failing deterministic test blocks regardless of a passing eval (R3.4)
	failVerify := map[string]EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 1, GitHead: "abc"}}
	if _, err := CompleteTaskWithQuality(raw, "T1", failVerify, contract, fresh, subject); err == nil {
		t.Fatalf("failing verify bypassed by passing eval")
	}
	wrongContract := contract
	wrongContract.TaskID = "T2"
	if _, err := CompleteTaskWithQuality(raw, "T1", pass, wrongContract, fresh, subject); err == nil || !strings.Contains(err.Error(), "QUALITY_TASK_MISMATCH") {
		t.Fatalf("wrong-task quality contract accepted: %v", err)
	}
}

func TestCompleteTaskLegacySubjectFreshnessBaseline(t *testing.T) {
	raw := []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | go test ./... | ok |\n")
	// Current API has no expected subject input, so any pinned historical head
	// passes. W2 replaces this baseline with explicit freshness refusal.
	if _, err := CompleteTask(raw, "T1", map[string]EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "old-head"}}); err != nil {
		t.Fatalf("baseline changed early: %v", err)
	}
}
