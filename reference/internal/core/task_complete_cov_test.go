package core

import (
	"strings"
	"testing"
)

// task_complete_cov_test.go covers ValidateTaskCompletion's gate branches and
// DeriveSpecStatus's lifecycle transitions directly.

func TestValidateTaskCompletionBranches(t *testing.T) {
	doc := &ParsedTask{Meta: map[string]string{"verify": "go test ./..."}}

	// Dependency not complete → gate error.
	st := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Status: TaskPending},
		"T2": {ID: "T2", Status: TaskPending, Depends: []string{"T1"}},
	}}
	if _, e := ValidateTaskCompletion(st, st.Tasks["T2"], doc, "demo", "T2", "", false); e == nil {
		t.Error("incomplete dependency should gate")
	}

	// --unverified without evidence → gate.
	t1 := TaskState{ID: "T1", Status: TaskRunning}
	st = &State{Tasks: map[string]TaskState{"T1": t1}}
	if _, e := ValidateTaskCompletion(st, t1, doc, "demo", "T1", "", true); e == nil {
		t.Error("unverified without evidence should gate")
	}
	// --unverified with evidence → ok, evidence preserved.
	if ev, e := ValidateTaskCompletion(st, t1, doc, "demo", "T1", "manual proof", true); e != nil || ev != "manual proof" {
		t.Errorf("unverified with evidence: ev=%q e=%v", ev, e)
	}

	// Verified path: missing verification record → gate.
	if _, e := ValidateTaskCompletion(st, t1, doc, "demo", "T1", "", false); e == nil {
		t.Error("missing verification should gate")
	}

	// Stale verification command → gate.
	stale := t1
	stale.Verification = &VerificationRecord{Verified: true, Command: "old cmd"}
	st = &State{Tasks: map[string]TaskState{"T1": stale}}
	if _, e := ValidateTaskCompletion(st, stale, doc, "demo", "T1", "", false); e == nil {
		t.Error("stale verify command should gate")
	}

	// Fresh verification → derived evidence string is synthesized.
	head := "abc123"
	fresh := t1
	fresh.Verification = &VerificationRecord{Verified: true, Command: "go test ./...", GitHead: &head, RanAt: "2026-01-01T00:00:00Z"}
	st = &State{Tasks: map[string]TaskState{"T1": fresh}}
	ev, e := ValidateTaskCompletion(st, fresh, doc, "demo", "T1", "", false)
	if e != nil || !strings.Contains(ev, "verified:") || !strings.Contains(ev, head) {
		t.Errorf("derived evidence: ev=%q e=%v", ev, e)
	}
}

func TestDeriveSpecStatusTransitions(t *testing.T) {
	// Empty tasks → no change.
	s := &State{Status: StatusExecuting, Tasks: map[string]TaskState{}}
	DeriveSpecStatus(s)
	if s.Status != StatusExecuting {
		t.Errorf("empty → %s", s.Status)
	}

	// Not started (all pending) → no change.
	s = &State{Status: StatusTasks, Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskPending}}}
	DeriveSpecStatus(s)
	if s.Status != StatusTasks {
		t.Errorf("not started → %s, want unchanged", s.Status)
	}

	// All complete → verifying.
	s = &State{Status: StatusExecuting, Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskComplete}}}
	DeriveSpecStatus(s)
	if s.Status != StatusVerifying {
		t.Errorf("all complete → %s, want verifying", s.Status)
	}

	// All complete but already complete → stays complete.
	s = &State{Status: StatusComplete, Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskComplete}}}
	DeriveSpecStatus(s)
	if s.Status != StatusComplete {
		t.Errorf("already complete → %s", s.Status)
	}

	// All-blocked frontier → blocked.
	s = &State{Status: StatusExecuting, Tasks: map[string]TaskState{
		"T1": {ID: "T1", Wave: 1, Status: TaskBlocked},
	}}
	DeriveSpecStatus(s)
	if s.Status != StatusBlocked {
		t.Errorf("all blocked → %s, want blocked", s.Status)
	}

	// Mixed in-progress → executing.
	s = &State{Status: StatusVerifying, Tasks: map[string]TaskState{
		"T1": {ID: "T1", Wave: 1, Status: TaskComplete},
		"T2": {ID: "T2", Wave: 2, Status: TaskPending},
	}}
	DeriveSpecStatus(s)
	if s.Status != StatusExecuting {
		t.Errorf("mixed → %s, want executing", s.Status)
	}
}
