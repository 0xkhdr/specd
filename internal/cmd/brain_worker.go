package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

type workerReport struct {
	TaskID string
}

func acceptWorkerReport(records map[string]core.EvidenceRecord, report workerReport) error {
	return requirePassingVerify(records, report.TaskID)
}

func requirePassingVerify(records map[string]core.EvidenceRecord, taskID string) error {
	if !core.HasPassingEvidence(records, taskID) {
		return fmt.Errorf("task %s has no passing verify evidence", taskID)
	}
	return nil
}
