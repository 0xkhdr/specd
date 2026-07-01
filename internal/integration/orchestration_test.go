package integration_test

import (
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

func TestFakeHostBrainLifecycleApprovalRetryPauseCompleteAndContention(t *testing.T) {
	h := testharness.New(t)
	h.Spec("brain-demo").
		Req("demo", "As a user, I want demo.", "THE SYSTEM SHALL satisfy demo.").
		FullDesign().
		AddTask(testharness.TaskSpec{ID: "T1", Title: "do demo", Files: "pass.flag", Verify: "test -f pass.flag", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Gate(core.GateAwaitingApproval).
		Build()
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := strings.Repeat("a", 32)

	approval := host.StartSpec("brain-demo", sessionID)
	if approval.Decision.Action != core.OrchestrationRequestApproval || approval.Event != nil {
		t.Fatalf("initial decision=%#v event=%#v, want approval without event", approval.Decision, approval.Event)
	}
	h.RunExpect(core.ExitOK, "approve", "brain-demo")

	dispatch := host.StepSpec("brain-demo", sessionID)
	if dispatch.Decision.Action != core.OrchestrationDispatch || dispatch.Decision.TaskID != "T1" || dispatch.Decision.Attempt != 1 || dispatch.Event == nil {
		t.Fatalf("dispatch step=%#v, want T1 attempt 1 event", dispatch)
	}

	pause := h.RunExpect(core.ExitOK, "brain", "pause", "--session", sessionID, "--json")
	if !strings.Contains(pause.Stdout, `"status": "paused"`) {
		t.Fatalf("pause output=%s, want paused", pause.Stdout)
	}
	paused := host.StepSpec("brain-demo", sessionID)
	if paused.Decision.Action != core.OrchestrationWait || paused.Event != nil {
		t.Fatalf("paused step=%#v event=%#v, want wait without event", paused.Decision, paused.Event)
	}
	h.RunExpect(core.ExitOK, "brain", "resume", "--session", sessionID, "--json")
	resumed := host.StepSpec("brain-demo", sessionID)
	if resumed.Event == nil || resumed.Event.MessageID != dispatch.Event.MessageID {
		t.Fatalf("resumed event=%#v, want idempotent original event %#v", resumed.Event, dispatch.Event)
	}

	second := h.Run("brain", append([]string{"start", "brain-demo"}, host.PolicyArgs(strings.Repeat("b", 32))...)...)
	if second.Code != core.ExitGate || !strings.Contains(second.Out(), "already has active session") {
		t.Fatalf("second session exit=%d out=%s, want active-session gate", second.Code, second.Out())
	}

	_, failedVerify, rec := host.ClaimAndVerify(resumed)
	if failedVerify.Code != core.ExitGate || rec == nil || rec.Verified {
		t.Fatalf("failed verify exit=%d rec=%#v out=%s", failedVerify.Code, rec, failedVerify.Out())
	}
	if reclaimed := host.ExpireLeasesAndReclaim(sessionID); reclaimed != 1 {
		t.Fatalf("reclaimed=%d, want 1", reclaimed)
	}
	if err := os.WriteFile(h.Path("pass.flag"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retry := host.StepSpec("brain-demo", sessionID)
	if retry.Decision.Action != core.OrchestrationDispatch || retry.Decision.Attempt != 2 {
		t.Fatalf("retry decision=%#v, want dispatch attempt 2", retry.Decision)
	}
	accepted := host.Complete(retry, "retry passed")
	if accepted.Completion.Status != core.TaskComplete || accepted.Completion.AlreadyComplete {
		t.Fatalf("completion=%#v, want fresh complete", accepted.Completion)
	}

	done := host.StepSpec("brain-demo", sessionID)
	if done.Decision.Action != core.OrchestrationCompleteSession {
		t.Fatalf("done decision=%#v, want complete-session", done.Decision)
	}
	session, err := core.LoadOrchestrationSession(h.Root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != core.OrchestrationSessionComplete {
		t.Fatalf("session status=%s, want complete", session.Status)
	}
	h.RunExpect(core.ExitOK, "approve", "brain-demo")
	loaded, err := core.LoadSpec(h.Root, "brain-demo")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State.Status != core.StatusComplete {
		t.Fatalf("spec status=%s, want complete", loaded.State.Status)
	}
}

func TestFakeHostProgramParallelRecoveryFailFastAndComplete(t *testing.T) {
	h := testharness.New(t)
	buildProgramSpec(h, "a", core.TaskPending, "true")
	buildProgramSpec(h, "b", core.TaskPending, "true")
	buildProgramSpec(h, "c", core.TaskPending, "true")
	if err := core.SaveProgram(h.Root, core.ProgramManifest{Version: core.ProgramVersion, DependsOn: map[string][]string{"c": {"a", "b"}}}); err != nil {
		t.Fatal(err)
	}
	host := testharness.NewFakeOrchestrationHost(h)
	parentID := strings.Repeat("c", 32)

	first := host.StartProgram(parentID)
	if first.Decision.Action != core.ProgramDecisionStart || !reflect.DeepEqual(first.Decision.Specs, []string{"a", "b"}) || len(first.Stepped) != 2 {
		t.Fatalf("first program step decision=%#v stepped=%d, want start [a b] and two child steps", first.Decision, len(first.Stepped))
	}
	for _, child := range first.Stepped {
		if child.Result.Decision.Action != core.OrchestrationDispatch {
			t.Fatalf("child %s decision=%#v, want dispatch", child.Slug, child.Result.Decision)
		}
		host.Complete(child.Result, child.Slug+" complete")
		h.RunExpect(core.ExitOK, "approve", child.Slug)
	}

	// Recreated host simulates process restart; state is recovered from disk.
	restarted := testharness.NewFakeOrchestrationHost(h)
	second := restarted.StepProgram(parentID)
	if second.Decision.Action != core.ProgramDecisionStart || !reflect.DeepEqual(second.Decision.Specs, []string{"c"}) || len(second.Stepped) != 1 {
		t.Fatalf("second program step decision=%#v stepped=%d, want start [c]", second.Decision, len(second.Stepped))
	}
	restarted.Complete(second.Stepped[0].Result, "c complete")
	h.RunExpect(core.ExitOK, "approve", second.Stepped[0].Slug)
	final := restarted.StepProgram(parentID)
	if final.Decision.Action != core.ProgramDecisionComplete {
		t.Fatalf("final decision=%#v, want complete", final.Decision)
	}

	contenders := []string{strings.Repeat("d", 32), strings.Repeat("e", 32), strings.Repeat("f", 32)}
	root := h.Root
	var wg sync.WaitGroup
	for _, id := range contenders {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, _ = core.StepProgramOrchestration(root, id, restarted.Policy, restarted.Cfg)
		}(id)
	}
	wg.Wait()
	leases, err := core.LoadProgramChildLeases(h.Root)
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	for _, lease := range leases {
		if lease.Status == core.ProgramChildLeaseActive {
			if prev := owners[lease.Slug]; prev != "" && prev != lease.ParentSessionID {
				t.Fatalf("duplicate active owners for %s: %s and %s", lease.Slug, prev, lease.ParentSessionID)
			}
			owners[lease.Slug] = lease.ParentSessionID
		}
	}
}

func TestFakeHostProgramFailFastEscalation(t *testing.T) {
	h := testharness.New(t)
	buildProgramSpec(h, "blocked", core.TaskBlocked, "true")
	buildProgramSpec(h, "dependent", core.TaskPending, "true")
	if err := core.SaveProgram(h.Root, core.ProgramManifest{Version: core.ProgramVersion, DependsOn: map[string][]string{"dependent": {"blocked"}}}); err != nil {
		t.Fatal(err)
	}
	host := testharness.NewFakeOrchestrationHost(h)
	parentID := strings.Repeat("9", 32)

	step := host.StartProgram(parentID)
	if step.Decision.Action != core.ProgramDecisionEscalate || !reflect.DeepEqual(step.Decision.Specs, []string{"blocked"}) || len(step.Started) != 0 {
		t.Fatalf("decision=%#v started=%d, want fail-fast escalation without dependent start", step.Decision, len(step.Started))
	}
	session, err := core.LoadProgramSession(h.Root, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != core.OrchestrationSessionFailed {
		t.Fatalf("program session status=%s, want failed", session.Status)
	}
}

func buildProgramSpec(h *testharness.Harness, slug string, taskStatus core.TaskStatus, verify string) {
	h.T.Helper()
	specStatus := core.StatusExecuting
	if taskStatus == core.TaskBlocked {
		specStatus = core.StatusBlocked
	}
	h.Spec(slug).
		Req(slug, "As a user, I want "+slug+".", "THE SYSTEM SHALL satisfy "+slug+".").
		FullDesign().
		AddTask(testharness.TaskSpec{ID: "T1", Title: "do " + slug, Files: slug + ".txt", Verify: verify, Requirements: []int{1}, Status: taskStatus}).
		Status(specStatus).
		Build()
}

// TestModeSwitchToBaseRefusedDuringActiveSession verifies the fail-closed
// guardrail: a spec cannot be switched back to Base while a Brain session is
// live (it would orphan the running session). Cancel-first remediation is shown.
func TestModeSwitchToBaseRefusedDuringActiveSession(t *testing.T) {
	h := testharness.New(t)
	h.Spec("live").
		Req("live", "As a user, I want live.", "THE SYSTEM SHALL satisfy live.").
		FullDesign().
		AddTask(testharness.TaskSpec{ID: "T1", Title: "do live", Files: "pass.flag", Verify: "test -f pass.flag", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := strings.Repeat("e", 32)

	// StartSpec opts the spec into orchestrated mode and starts the session.
	host.StartSpec("live", sessionID)

	res := h.RunExpect(core.ExitGate, "mode", "live", "--set", "simple")
	if !strings.Contains(res.Out(), "Brain session") || !strings.Contains(res.Out(), "Cancel") {
		t.Errorf("expected cancel-first refusal, got: %s", res.Out())
	}
	if h.State("live").Raw().EffectiveMode() != core.ModeOrchestrated {
		t.Error("spec must remain orchestrated after refused switch")
	}
}
