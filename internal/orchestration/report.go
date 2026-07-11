package orchestration

import (
	"fmt"
	"time"
)

type WorkerReportV1 struct {
	MissionID   string `json:"mission_id"`
	LeaseID     string `json:"lease_id"`
	WorkerID    string `json:"worker_id"`
	TaskID      string `json:"task_id"`
	Attempt     int    `json:"attempt"`
	Role        string `json:"role"`
	SubjectHead string `json:"subject_head"`
	VerifyRef   string `json:"verify_ref"`
	Status      string `json:"status"`
	Blocker     string `json:"blocker,omitempty"`
	NextAction  string `json:"next_action,omitempty"`
}

func ValidateWorkerReport(r WorkerReportV1, m MissionV1, l Lease, now time.Time) error {
	if r.MissionID != m.MissionID || r.MissionID != l.MissionID || r.LeaseID != l.LeaseID || r.WorkerID != l.WorkerID || r.TaskID != m.TaskID || r.TaskID != l.TaskID || r.Attempt != m.Attempt || r.Attempt != l.Attempt || r.Role != m.Role || r.SubjectHead == "" || r.VerifyRef == "" {
		return fmt.Errorf("REPORT_IDENTITY_OR_PIN_MISMATCH")
	}
	if l.State != LeaseActive || !now.Before(l.ExpiresAt) {
		return fmt.Errorf("REPORT_LEASE_NOT_LIVE")
	}
	if r.Status != "complete" && r.Status != "blocked" && r.Status != "failed" {
		return fmt.Errorf("REPORT_STATUS_INVALID")
	}
	return nil
}
