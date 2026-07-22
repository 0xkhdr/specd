package core

import (
	"encoding/json"
	"testing"
)

// TestClarificationLifecycle pins the spec 03 R4 contract: a versioned
// clarification blocks only the entity it names, resolutions are appended rather
// than edited, and an answer overtaken by an artifact revision stays as history
// while naming the review it now needs.
func TestClarificationLifecycle(t *testing.T) {
	tasks := []TaskRow{{ID: "T1", Verify: "true", Acceptance: "ok"}, {ID: "T2", Verify: "true", Acceptance: "ok"}}
	versions := TaskEntityVersions(tasks)
	open := func(id, task string, blocking bool) ClarificationRecord {
		return ClarificationRecord{
			ID: id, Transition: ClarificationOpen, EntityKind: ClarificationEntityTask,
			EntityID: task, EntityVersion: versions[task], Blocking: blocking, Question: "which rounding?",
		}
	}
	waits := func(t *testing.T, records []ClarificationRecord, task string) []WaitReason {
		t.Helper()
		states, err := ProjectTaskStates(tasks, nil, ClarificationTaskFacts(records, versions))
		if err != nil {
			t.Fatalf("ProjectTaskStates: %v", err)
		}
		for _, state := range states {
			if state.ID == task {
				return state.Waits
			}
		}
		t.Fatalf("task %s missing from projection", task)
		return nil
	}

	t.Run("MultipleOpen", func(t *testing.T) {
		records := []ClarificationRecord{open("C1", "T1", true), open("C2", "T1", true), open("C3", "T2", true)}
		got := waits(t, records, "T1")
		if len(got) != 2 || got[0].Refs[0] != "C1" || got[1].Refs[0] != "C2" {
			t.Fatalf("T1 waits = %#v, want both open questions", got)
		}
		for _, wait := range got {
			if wait.Code != WaitClarificationOpen || wait.Readiness != ReadinessWaitingClarification || wait.Recovery == "" {
				t.Fatalf("wait = %#v, want an actionable open-clarification wait", wait)
			}
		}
		if len(waits(t, records, "T2")) != 1 {
			t.Fatalf("T2 waits = %#v, want only its own question", waits(t, records, "T2"))
		}
	})

	t.Run("Nonblocking", func(t *testing.T) {
		if got := waits(t, []ClarificationRecord{open("C1", "T1", false)}, "T1"); len(got) != 0 {
			t.Fatalf("non-blocking clarification produced waits %#v", got)
		}
	})

	t.Run("Withdrawn", func(t *testing.T) {
		records := []ClarificationRecord{open("C1", "T1", true)}
		key, resolved, err := PlanClarification(records, ClarificationRecord{ID: "C1", Transition: ClarificationWithdrawn})
		if err != nil {
			t.Fatalf("withdraw: %v", err)
		}
		if key != "clarification:C1:1" || resolved.EntityID != "T1" || !resolved.Blocking {
			t.Fatalf("withdrawal = %s %#v, want a new key inheriting the question's identity", key, resolved)
		}
		records = append(records, resolved)
		if got := waits(t, records, "T1"); len(got) != 0 {
			t.Fatalf("withdrawn clarification still blocks: %#v", got)
		}
		if _, _, err := PlanClarification(records, ClarificationRecord{ID: "C1", Transition: ClarificationAnswered, Answer: "x"}); err == nil {
			t.Fatal("answering a withdrawn clarification was accepted; records must stay immutable")
		}
	})

	t.Run("Expired", func(t *testing.T) {
		records := []ClarificationRecord{open("C1", "T1", true)}
		_, resolved, err := PlanClarification(records, ClarificationRecord{ID: "C1", Transition: ClarificationExpired, Reason: "no answer"})
		if err != nil {
			t.Fatalf("expire: %v", err)
		}
		if got := waits(t, append(records, resolved), "T1"); len(got) != 0 {
			t.Fatalf("expired clarification still blocks: %#v", got)
		}
	})

	t.Run("StaleAnswer", func(t *testing.T) {
		records := []ClarificationRecord{open("C1", "T1", true)}
		_, answered, err := PlanClarification(records, ClarificationRecord{ID: "C1", Transition: ClarificationAnswered, Answer: "round half up"})
		if err != nil {
			t.Fatalf("answer: %v", err)
		}
		answered.EntityVersion = versions["T1"]
		records = append(records, answered)
		if got := waits(t, records, "T1"); len(got) != 0 {
			t.Fatalf("current answer still blocks: %#v", got)
		}
		// The task's contract is revised; the answer stays as history and the
		// projection names the review that revision now needs.
		revised := []TaskRow{{ID: "T1", Verify: "true", Acceptance: "ok and rounded"}, tasks[1]}
		states, err := ProjectTaskStates(revised, nil, ClarificationTaskFacts(records, TaskEntityVersions(revised)))
		if err != nil {
			t.Fatalf("ProjectTaskStates: %v", err)
		}
		got := states[0].Waits
		if states[0].Readiness != ReadinessWaitingClarification || len(got) != 1 || got[0].Code != WaitClarificationStale {
			t.Fatalf("revised task = %#v, want a stale-answer wait", states[0])
		}
		if records[1].Answer != "round half up" {
			t.Fatalf("answer was not retained as history: %#v", records[1])
		}
		// A marker-only change is not a contract revision.
		marked := []TaskRow{{ID: "T1", Marker: "✅", Verify: "true", Acceptance: "ok"}, tasks[1]}
		if got := ClarificationTaskFacts(records, TaskEntityVersions(marked))["T1"].Waits; len(got) != 0 {
			t.Fatalf("completion marker invalidated the answer: %#v", got)
		}
	})

	t.Run("ClosedTransitions", func(t *testing.T) {
		if _, _, err := PlanClarification(nil, ClarificationRecord{ID: "C1", Transition: ClarificationAnswered, Answer: "x"}); err == nil {
			t.Fatal("resolved an unopened clarification")
		}
		if _, _, err := PlanClarification(nil, open("C1", "T1", true)); err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, _, err := PlanClarification([]ClarificationRecord{open("C1", "T1", true)}, open("C1", "T1", true)); err == nil {
			t.Fatal("reopened an existing id; a changed question must take a new id")
		}
		if _, _, err := PlanClarification(nil, ClarificationRecord{ID: "C1", Transition: "reopened"}); err == nil {
			t.Fatal("accepted a transition outside the closed set")
		}
		if got := NextClarificationID([]ClarificationRecord{open("C1", "T1", true)}); got != "C2" {
			t.Fatalf("next id = %s, want C2", got)
		}
	})

	t.Run("ReadRecords", func(t *testing.T) {
		raw, err := json.Marshal(open("C1", "T1", true))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		records := map[string]json.RawMessage{"clarification:C1:0": raw, "decision:0": json.RawMessage(`{"kind":"decision"}`)}
		got, err := ReadClarifications(records)
		if err != nil || len(got) != 1 || got[0].ID != "C1" {
			t.Fatalf("ReadClarifications = %#v, %v", got, err)
		}
		records["clarification:C2:0"] = json.RawMessage(`{`)
		if _, err := ReadClarifications(records); err == nil {
			t.Fatal("malformed clarification record was read as absent; it must fail closed")
		}
	})
}
