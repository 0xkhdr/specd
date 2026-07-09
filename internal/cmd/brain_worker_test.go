package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

const pinnedHead = "0123456789abcdef0123456789abcdef01234567"

func TestReportRequiresVerify(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	records := map[string]core.EvidenceRecord{
		"T1": {TaskID: "T1", Command: "go test ./...", ExitCode: 0, GitHead: pinnedHead},
		"T2": {TaskID: "T2", Command: "go test ./...", ExitCode: 1, GitHead: pinnedHead},
	}
	report := workerReport{
		TaskID:   "T1",
		WorkerID: "worker-1",
		GitHead:  pinnedHead,
		Lease:    orchestration.Lease{TaskID: "T1", WorkerID: "worker-1", ExpiresAt: now.Add(time.Minute)},
		Now:      now,
	}
	if err := acceptWorkerReport(records, report); err != nil {
		t.Fatalf("passing report rejected: %v", err)
	}
	if err := acceptWorkerReport(records, workerReport{TaskID: "T2", WorkerID: "worker-1", GitHead: pinnedHead, Lease: orchestration.Lease{TaskID: "T2", WorkerID: "worker-1", ExpiresAt: now.Add(time.Minute)}, Now: now}); err == nil {
		t.Fatal("failing verify accepted")
	}
	if err := acceptWorkerReport(records, workerReport{TaskID: "T3", WorkerID: "worker-1", GitHead: pinnedHead, Lease: orchestration.Lease{TaskID: "T3", WorkerID: "worker-1", ExpiresAt: now.Add(time.Minute)}, Now: now}); err == nil {
		t.Fatal("missing report accepted")
	}
}

func TestReportRequiresLeaseAndPinnedHead(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	records := map[string]core.EvidenceRecord{
		"T1": {TaskID: "T1", Command: "go test ./...", ExitCode: 0, GitHead: pinnedHead},
	}
	valid := workerReport{
		TaskID:   "T1",
		WorkerID: "worker-1",
		GitHead:  pinnedHead,
		Lease:    orchestration.Lease{TaskID: "T1", WorkerID: "worker-1", ExpiresAt: now.Add(time.Minute)},
		Now:      now,
	}
	cases := []struct {
		name string
		edit func(workerReport) workerReport
		want string
	}{
		{name: "missing task", edit: func(r workerReport) workerReport { r.TaskID = ""; return r }, want: "missing task"},
		{name: "missing lease", edit: func(r workerReport) workerReport { r.Lease = orchestration.Lease{}; return r }, want: "missing lease"},
		{name: "task mismatch", edit: func(r workerReport) workerReport { r.Lease.TaskID = "T2"; return r }, want: "does not match lease task"},
		{name: "worker mismatch", edit: func(r workerReport) workerReport { r.Lease.WorkerID = "worker-2"; return r }, want: "does not match lease worker"},
		{name: "expired lease", edit: func(r workerReport) workerReport { r.Lease.ExpiresAt = now; return r }, want: "expired lease"},
		{name: "missing report head", edit: func(r workerReport) workerReport { r.GitHead = ""; return r }, want: "missing git HEAD"},
		{name: "fake report head", edit: func(r workerReport) workerReport { r.GitHead = "unknown"; return r }, want: "report HEAD is not pinned"},
		{name: "head mismatch", edit: func(r workerReport) workerReport { r.GitHead = "1111111111111111111111111111111111111111"; return r }, want: "does not match evidence HEAD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := acceptWorkerReport(records, tc.edit(valid))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestReportRejectsUnpinnedEvidenceHead(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	records := map[string]core.EvidenceRecord{
		"T1": {TaskID: "T1", Command: "go test ./...", ExitCode: 0, GitHead: "unknown"},
	}
	report := workerReport{
		TaskID:   "T1",
		WorkerID: "worker-1",
		GitHead:  pinnedHead,
		Lease:    orchestration.Lease{TaskID: "T1", WorkerID: "worker-1", ExpiresAt: now.Add(time.Minute)},
		Now:      now,
	}
	err := acceptWorkerReport(records, report)
	if err == nil || !strings.Contains(err.Error(), "evidence is not pinned") {
		t.Fatalf("err=%v, want unpinned evidence rejection", err)
	}
}
