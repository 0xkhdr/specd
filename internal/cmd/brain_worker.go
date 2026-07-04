package cmd

import "github.com/0xkhdr/specd/internal/core"

type workerReport struct {
	TaskID string
}

func acceptWorkerReport(records map[string]core.EvidenceRecord, report workerReport) error {
	return requirePassingVerify(records, report.TaskID)
}
