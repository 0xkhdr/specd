package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestClarificationLifecycleCLI pins spec 03 R4 at the CLI edge: recording a
// blocking question blocks only the task it names, `status` reports every open
// question, and an appended resolution restores eligibility without editing the
// record it resolves.
func TestClarificationLifecycleCLI(t *testing.T) {
	root := newDemoSpec(t)
	writeTasks(t, root, "demo", "| T1 | scout | spec.md | - | true | ok |\n| T2 | scout | spec.md | - | true | ok |")
	advanceToExecuting(t, root)
	statusTask := func(t *testing.T, id string) core.ReportTask {
		t.Helper()
		out, err := captureStdout(t, func() error {
			return Run(root, "status", []string{"demo"}, map[string]string{"json": ""})
		})
		if err != nil {
			t.Fatalf("status --json: %v", err)
		}
		var model core.ReportModel
		if err := json.Unmarshal([]byte(out), &model); err != nil {
			t.Fatalf("status json: %v (out=%q)", err, out)
		}
		for _, task := range model.Tasks {
			if task.ID == id {
				return task
			}
		}
		t.Fatalf("task %s missing from status", id)
		return core.ReportTask{}
	}

	if got := statusTask(t, "T1"); got.Readiness != core.ReadinessReady {
		t.Fatalf("T1 before any clarification = %#v, want ready", got)
	}
	for _, entity := range []string{"task:T1", "task:T1"} {
		if err := Run(root, "clarification", []string{"open", "demo"}, map[string]string{
			"question": "which rounding?", "entity": entity, "blocking": "",
		}); err != nil {
			t.Fatalf("clarification open: %v", err)
		}
	}
	if err := Run(root, "clarification", []string{"open", "demo"}, map[string]string{
		"question": "nice to know", "entity": "task:T2",
	}); err != nil {
		t.Fatalf("non-blocking clarification open: %v", err)
	}

	blocked := statusTask(t, "T1")
	if blocked.Readiness != core.ReadinessWaitingClarification || len(blocked.Waits) != 2 {
		t.Fatalf("T1 = %#v, want both open questions reported", blocked)
	}
	if got := statusTask(t, "T2"); got.Readiness != core.ReadinessReady {
		t.Fatalf("T2 = %#v, want a non-blocking question to leave readiness alone", got)
	}
	err := Run(root, "context", []string{"demo", "T1"}, nil)
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "CLARIFICATION_OPEN" || !strings.Contains(refusal.Blocker, "C1") {
		t.Fatalf("context for a blocked task = %v, want a scoped clarification refusal", err)
	}
	if err := Run(root, "context", []string{"demo", "T2"}, nil); err != nil {
		t.Fatalf("context for an unaffected task: %v", err)
	}

	if err := Run(root, "clarification", []string{"answer", "demo", "C1"}, map[string]string{"answer": "round half up"}); err != nil {
		t.Fatalf("clarification answer: %v", err)
	}
	if err := Run(root, "clarification", []string{"withdraw", "demo", "C2"}, map[string]string{"reason": "duplicate"}); err != nil {
		t.Fatalf("clarification withdraw: %v", err)
	}
	if got := statusTask(t, "T1"); got.Readiness != core.ReadinessReady {
		t.Fatalf("T1 after resolution = %#v, want ready again", got)
	}
	if err := Run(root, "clarification", []string{"expire", "demo", "C1"}, map[string]string{"reason": "late"}); err == nil {
		t.Fatal("re-resolved a resolved clarification; records must be immutable")
	}

	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	records, err := core.ReadClarifications(state.Records)
	if err != nil {
		t.Fatalf("read clarifications: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("records = %d, want three questions plus two appended resolutions", len(records))
	}
	if records[0].Transition != core.ClarificationOpen || records[0].Question == "" || records[0].EntityVersion == "" {
		t.Fatalf("open record = %#v, want the question pinned to an entity version", records[0])
	}
	if records[1].Transition != core.ClarificationAnswered || records[1].Answer != "round half up" {
		t.Fatalf("resolution = %#v, want the answer appended beside the question", records[1])
	}
	if records[0].Actor == "" || records[0].Timestamp == "" {
		t.Fatalf("record is unstamped: %#v", records[0])
	}
}

// TestClarificationLifecycleUsage pins the fail-closed edges of the verb.
func TestClarificationLifecycleUsage(t *testing.T) {
	root := newDemoSpec(t)
	for name, args := range map[string][]string{
		"MissingSpec":       {"open"},
		"UnknownSubcommand": {"reopen", "demo"},
		"MissingID":         {"answer", "demo"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := Run(root, "clarification", args, map[string]string{"question": "q", "answer": "a"}); err == nil {
				t.Fatalf("%v was accepted", args)
			}
		})
	}
	if err := Run(root, "clarification", []string{"open", "demo"}, map[string]string{"question": "q", "entity": "task"}); err == nil {
		t.Fatal("malformed --entity was accepted")
	}
	if err := Run(root, "clarification", []string{"open", "demo"}, map[string]string{"question": "q", "entity": "epic:E1"}); err == nil {
		t.Fatal("unknown entity kind was accepted")
	}
	if err := Run(root, "clarification", []string{"open", "demo"}, map[string]string{"entity": "task:T1"}); err == nil {
		t.Fatal("a clarification without a question was accepted")
	}
}
