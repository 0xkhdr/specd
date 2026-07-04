package core

import (
	"strings"
	"testing"
)

// TestRejectUnknownHead asserts evidence that cannot be pinned to a commit
// (empty or "unknown" git_head) is refused as completion evidence, and the
// error names the re-verify remedy (R3.2, R3.3).
func TestRejectUnknownHead(t *testing.T) {
	raw := []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | builder | a.go | - | true | ok |\n")
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
	raw := []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | builder | a.go | - | go test ./... | ok |\n")
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
