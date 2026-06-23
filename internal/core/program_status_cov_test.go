package core

import "testing"

// program_status_cov_test.go covers the pure decision/report builders in
// program_status.go via constructed sessions and snapshots (no filesystem).

func TestProgramStatusDecision(t *testing.T) {
	snap := ProgramSnapshot{}
	cases := []struct {
		status OrchestrationSessionStatus
		want   ProgramDecisionAction
	}{
		{OrchestrationSessionPaused, ProgramDecisionWait},
		{OrchestrationSessionCancelling, ProgramDecisionWait},
		{OrchestrationSessionComplete, ProgramDecisionComplete},
		{OrchestrationSessionFailed, ProgramDecisionEscalate},
	}
	for _, c := range cases {
		got, err := programStatusDecision(ProgramSession{Status: c.status}, snap)
		if err != nil {
			t.Fatalf("status %s: %v", c.status, err)
		}
		if got.Action != c.want {
			t.Errorf("status %s → %s, want %s", c.status, got.Action, c.want)
		}
	}

	// Running (default) delegates to DecideProgram, which requires a versioned
	// snapshot. An empty (but versioned) snapshot has no runnable children.
	got, err := programStatusDecision(ProgramSession{Status: OrchestrationSessionRunning}, ProgramSnapshot{Version: OrchestrationModelVersion})
	if err != nil {
		t.Fatal(err)
	}
	if got.Action == "" {
		t.Fatal("running status should yield a concrete DecideProgram action")
	}
}

func TestBuildProgramStatusReport(t *testing.T) {
	snap := ProgramSnapshot{
		ActiveCount: 1,
		Children: []ProgramChildSnapshot{
			{Slug: "a", Wave: 1, Complete: true},
			{Slug: "b", Wave: 1, Active: true},
			{Slug: "c", Wave: 2, Blocked: true},
			{Slug: "d", Wave: 2, Escalated: true},
		},
	}
	session := ProgramSession{ParentSessionID: "p1", Status: OrchestrationSessionRunning}
	decision := programControlDecision(ProgramDecisionWait, "test")

	r := buildProgramStatusReport(session, snap, decision)
	if r.Counts.Total != 4 || r.Counts.Complete != 1 || r.Counts.Blocked != 1 || r.Counts.Escalated != 1 {
		t.Fatalf("counts wrong: %+v", r.Counts)
	}
	if r.Counts.Active != 1 {
		t.Fatalf("active count want 1, got %d", r.Counts.Active)
	}
	if len(r.Waves) != 2 {
		t.Fatalf("want 2 waves, got %d", len(r.Waves))
	}
	// Wave 1: one complete, one active.
	if r.Waves[0].Wave != 1 || r.Waves[0].Complete != 1 || r.Waves[0].Active != 1 {
		t.Fatalf("wave 1 wrong: %+v", r.Waves[0])
	}
	// Blocked and escalated children both surface in the escalation list.
	if len(r.Escalation) != 2 {
		t.Fatalf("escalation want 2 (blocked+escalated), got %v", r.Escalation)
	}
	if r.Frontier == nil {
		t.Fatal("frontier should be non-nil (empty slice, never nil)")
	}
	if r.Session.ParentSessionID != "p1" {
		t.Errorf("session not threaded through: %+v", r.Session)
	}
}
