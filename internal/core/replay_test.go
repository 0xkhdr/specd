package core

import (
	"testing"
)

func strptr(s string) *string { return &s }

func TestReplayTimeline(t *testing.T) {
	state := &State{
		Spec: "demo",
		Tasks: map[string]TaskState{
			"T1": {
				ID: "T1", Title: "build", StartedAt: strptr("2026-01-01T00:00:00Z"),
				FinishedAt:   strptr("2026-01-01T00:05:00Z"),
				Verification: &VerificationRecord{Command: "go test", Verified: true, RanAt: "2026-01-01T00:04:00Z"},
			},
			"T2": {
				ID: "T2", Title: "review", StartedAt: strptr("2026-01-01T00:02:00Z"),
				Blocker: strptr("waiting on T1"),
			},
		},
		Acceptance: map[string]CriterionRecord{
			"1.1": {Requirement: 1, Criterion: 1, Status: "pass", Evidence: "proof", RanAt: "2026-01-01T00:06:00Z"},
		},
	}

	ev := ReplayTimeline(state)

	// Chronological order by timestamp (blocked has no timestamp → sorts first).
	wantOrder := []struct{ kind, task string }{
		{"blocked", "T2"},      // At=""
		{"started", "T1"},      // 00:00
		{"started", "T2"},      // 00:02
		{"verified", "T1"},     // 00:04
		{"finished", "T1"},     // 00:05
		{"criterion-pass", ""}, // 00:06
	}
	if len(ev) != len(wantOrder) {
		t.Fatalf("got %d events, want %d: %+v", len(ev), len(wantOrder), ev)
	}
	for i, w := range wantOrder {
		if ev[i].Kind != w.kind || ev[i].Task != w.task {
			t.Errorf("event %d = {%s %s}, want {%s %s}", i, ev[i].Kind, ev[i].Task, w.kind, w.task)
		}
	}

	// Determinism: identical input yields identical order.
	again := ReplayTimeline(state)
	for i := range ev {
		if ev[i] != again[i] {
			t.Errorf("non-deterministic at %d: %+v vs %+v", i, ev[i], again[i])
		}
	}

	// Tolerates nil / empty state without panicking.
	if ReplayTimeline(nil) != nil {
		t.Error("nil state should yield nil timeline")
	}
	if got := ReplayTimeline(&State{Tasks: map[string]TaskState{}}); len(got) != 0 {
		t.Errorf("empty state should yield no events, got %+v", got)
	}

	// A failed verify is distinguished.
	failState := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Verification: &VerificationRecord{Command: "x", Verified: false, RanAt: "2026-01-01T00:00:00Z"}},
	}}
	fe := ReplayTimeline(failState)
	if len(fe) != 1 || fe[0].Kind != "verify-failed" {
		t.Errorf("failed verify = %+v, want verify-failed", fe)
	}
}

// TestReplayDeterministic asserts the timeline is byte-stable across repeated
// calls even when several events share a timestamp — ties break on a fixed key
// (task ordinal, kind), never on Go map iteration order.
func TestReplayDeterministic(t *testing.T) {
	state := &State{
		Spec: "demo",
		Tasks: map[string]TaskState{
			"T3": {ID: "T3", StartedAt: strptr("2026-01-01T00:00:00Z")},
			"T1": {ID: "T1", StartedAt: strptr("2026-01-01T00:00:00Z")},
			"T2": {ID: "T2", StartedAt: strptr("2026-01-01T00:00:00Z")},
		},
	}
	first := ReplayTimeline(state)
	for i := 0; i < 20; i++ {
		again := ReplayTimeline(state)
		if len(again) != len(first) {
			t.Fatalf("iteration %d: length drift %d != %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("iteration %d event %d: %+v != %+v", i, j, again[j], first[j])
			}
		}
	}
}

// TestReplayMissing confirms the collector tolerates gaps — nil state, no tasks,
// and records with missing timestamps — without panicking or dropping into an
// undefined order.
func TestReplayMissing(t *testing.T) {
	if got := ReplayTimeline(nil); got != nil {
		t.Errorf("nil state should yield nil, got %+v", got)
	}
	if got := ReplayTimeline(&State{}); len(got) != 0 {
		t.Errorf("zero-value state should yield no events, got %+v", got)
	}
	// A task with no timestamps anywhere must not panic and must still surface
	// whatever events it can (here: none crash, started omitted when At empty).
	gapState := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Verification: &VerificationRecord{Command: "x", Verified: true}},
	}}
	_ = ReplayTimeline(gapState) // must not panic
}
