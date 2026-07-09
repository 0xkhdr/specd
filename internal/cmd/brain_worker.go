package cmd

import (
	"fmt"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

type workerReport struct {
	TaskID   string
	WorkerID string
	GitHead  string
	Lease    orchestration.Lease
	Now      time.Time
}

func acceptWorkerReport(records map[string]core.EvidenceRecord, report workerReport) error {
	if report.TaskID == "" {
		return fmt.Errorf("worker report missing task")
	}
	if err := requireLiveLease(report); err != nil {
		return err
	}
	return requirePassingVerify(records, report)
}

func requireLiveLease(report workerReport) error {
	if report.Lease.TaskID == "" {
		return fmt.Errorf("worker report for %s missing lease", report.TaskID)
	}
	if report.Lease.TaskID != report.TaskID {
		return fmt.Errorf("worker report task %s does not match lease task %s", report.TaskID, report.Lease.TaskID)
	}
	if report.WorkerID == "" {
		return fmt.Errorf("worker report for %s missing worker", report.TaskID)
	}
	if report.Lease.WorkerID != report.WorkerID {
		return fmt.Errorf("worker report worker %s does not match lease worker %s", report.WorkerID, report.Lease.WorkerID)
	}
	now := report.Now
	if now.IsZero() {
		now = time.Now()
	}
	if !now.Before(report.Lease.ExpiresAt) {
		return fmt.Errorf("worker report for %s has expired lease", report.TaskID)
	}
	return nil
}

func requirePassingVerify(records map[string]core.EvidenceRecord, report workerReport) error {
	record, ok := records[report.TaskID]
	if !ok {
		return fmt.Errorf("worker report for %s rejected: missing passing verify evidence", report.TaskID)
	}
	if record.ExitCode != 0 {
		return fmt.Errorf("worker report for %s rejected: verify exit code %d", report.TaskID, record.ExitCode)
	}
	if !core.HeadPinned(record.GitHead) {
		return fmt.Errorf("worker report for %s rejected: verify evidence is not pinned to a commit", report.TaskID)
	}
	if report.GitHead == "" {
		return fmt.Errorf("worker report for %s missing git HEAD", report.TaskID)
	}
	if !core.HeadPinned(report.GitHead) {
		return fmt.Errorf("worker report for %s rejected: report HEAD is not pinned to a commit", report.TaskID)
	}
	if report.GitHead != record.GitHead {
		return fmt.Errorf("worker report for %s rejected: report HEAD %s does not match evidence HEAD %s", report.TaskID, report.GitHead, record.GitHead)
	}
	return nil
}
