package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestReportRequiresVerify(t *testing.T) {
	records := map[string]core.EvidenceRecord{
		"T1": {TaskID: "T1", Command: "go test ./...", ExitCode: 0, GitHead: "abc"},
		"T2": {TaskID: "T2", Command: "go test ./...", ExitCode: 1, GitHead: "abc"},
	}
	if err := acceptWorkerReport(records, workerReport{TaskID: "T1"}); err != nil {
		t.Fatalf("passing report rejected: %v", err)
	}
	if err := acceptWorkerReport(records, workerReport{TaskID: "T2"}); err == nil {
		t.Fatal("failing report accepted")
	}
	if err := acceptWorkerReport(records, workerReport{TaskID: "T3"}); err == nil {
		t.Fatal("missing report accepted")
	}
}
