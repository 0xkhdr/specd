package orchestration

import "testing"

func TestWorkerReportMatchesLease(t *testing.T) {
	m := validMission()
	l := Lease{LeaseID: "l1", MissionID: m.MissionID, TaskID: m.TaskID, Attempt: m.Attempt, WorkerID: "w1", IssuedAt: m.IssuedAt, ExpiresAt: m.ExpiresAt, PolicyDigest: m.PolicyDigest, State: LeaseActive}
	r := WorkerReportV1{MissionID: m.MissionID, LeaseID: l.LeaseID, WorkerID: l.WorkerID, TaskID: m.TaskID, Attempt: m.Attempt, Role: m.Role, SubjectHead: m.SubjectHead, VerifyRef: "evidence.jsonl#T1", Status: "complete"}
	if err := ValidateWorkerReport(r, m, l, m.IssuedAt); err != nil {
		t.Fatal(err)
	}
	r.MissionID = "wrong"
	if err := ValidateWorkerReport(r, m, l, m.IssuedAt); err == nil {
		t.Fatal("wrong mission accepted")
	}
}
