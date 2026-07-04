package core

import "testing"

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
