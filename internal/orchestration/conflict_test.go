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
	// Disjoint scopes are only safe once the host proves isolation. In a shared
	// worktree the second mission's diff would carry the first one's edits, so
	// R4.1 serializes it regardless of declared files.
	candidate.DeclaredFiles = []string{"internal/b.go"}
	if err := CheckParallelConflict(candidate, active, leases, CoordinationRule{}, now); err == nil {
		t.Fatal("disjoint scopes still share the worktree; expected serialization")
	}
	if err := CheckParallelConflict(candidate, active, leases, CoordinationRule{IsolationID: "wt-1"}, now); err != nil {
		t.Fatalf("disjoint scopes under proven isolation should permit claim: %v", err)
	}
}
