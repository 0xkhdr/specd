package orchestration

import (
	"testing"
	"time"
)

func TestConflictOverlappingWriteScopes(t *testing.T) {
	now := time.Now()
	candidate := MissionV1{MissionID: "m2", TaskID: "T2", DeclaredFiles: []string{"internal/a.go"}}
	active := []MissionV1{{MissionID: "m1", TaskID: "T1", DeclaredFiles: []string{"./internal/a.go"}}}
	leases := []Lease{{MissionID: "m1", TaskID: "T1", State: LeaseActive, ExpiresAt: now.Add(time.Minute)}}
	if err := CheckParallelConflict(candidate, active, leases, CoordinationRule{}, now); err == nil {
		t.Fatal("expected overlapping write scope refusal")
	}
	rule := CoordinationRule{Digest: "sha256:approved", OrderedTasks: []string{"T1", "T2"}}
	if err := CheckParallelConflict(candidate, active, leases, rule, now); err != nil {
		t.Fatalf("approved ordering should permit claim: %v", err)
	}
	candidate.DeclaredFiles = []string{"internal/b.go"}
	if err := CheckParallelConflict(candidate, active, leases, CoordinationRule{}, now); err != nil {
		t.Fatalf("disjoint scopes should permit claim: %v", err)
	}
}
