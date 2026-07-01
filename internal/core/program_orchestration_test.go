package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProgramOrchestrationDecide(t *testing.T) {
	t.Run("frontier_order_wave_then_slug_and_capacity", func(t *testing.T) {
		root := t.TempDir()
		scaffoldSpec(t, root, "a", StatusComplete)
		scaffoldSpec(t, root, "b", StatusExecuting)
		scaffoldSpec(t, root, "c", StatusExecuting)
		scaffoldSpec(t, root, "d", StatusExecuting)
		if err := SaveProgram(root, ProgramManifest{
			Version: ProgramVersion,
			DependsOn: map[string][]string{
				"b": {"a"},
				"c": {"a"},
				"d": {"b"},
			},
		}); err != nil {
			t.Fatal(err)
		}
		graph, err := BuildProgram(root, nil)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := BuildProgramSnapshot(graph, map[string]bool{"b": true}, 2)
		if err != nil {
			t.Fatal(err)
		}
		decision, err := DecideProgram(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionStart || !reflect.DeepEqual(decision.Specs, []string{"c"}) {
			t.Fatalf("decision = %#v, want start [c]", decision)
		}
	})

	t.Run("cycle_or_orphan_escalates", func(t *testing.T) {
		cycle := ProgramSnapshot{Version: OrchestrationModelVersion, Capacity: 1, Cycle: []string{"a", "b", "a"}}
		decision, err := DecideProgram(cycle)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionEscalate {
			t.Fatalf("cycle decision = %#v, want escalate", decision)
		}
		orphan := ProgramSnapshot{
			Version:  OrchestrationModelVersion,
			Capacity: 1,
			Orphans:  []struct{ Spec, Dep string }{{Spec: "b", Dep: "missing"}},
		}
		decision, err = DecideProgram(orphan)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionEscalate {
			t.Fatalf("orphan decision = %#v, want escalate", decision)
		}
	})

	t.Run("complete_and_capacity_wait", func(t *testing.T) {
		complete := ProgramSnapshot{
			Version:  OrchestrationModelVersion,
			Capacity: 1,
			Children: []ProgramChildSnapshot{{Slug: "a", Status: StatusComplete, Complete: true}},
		}
		decision, err := DecideProgram(complete)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionComplete {
			t.Fatalf("complete decision = %#v, want complete", decision)
		}
		wait := ProgramSnapshot{
			Version:     OrchestrationModelVersion,
			Capacity:    1,
			ActiveCount: 1,
			Children:    []ProgramChildSnapshot{{Slug: "a", Status: StatusExecuting}},
		}
		decision, err = DecideProgram(wait)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionWait {
			t.Fatalf("wait decision = %#v, want wait", decision)
		}
	})
}

func TestProgramOrchestrationLease(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentIDs := []string{
		strings.Repeat("a", 32),
		strings.Repeat("b", 32),
		strings.Repeat("c", 32),
		strings.Repeat("d", 32),
		strings.Repeat("e", 32),
		strings.Repeat("f", 32),
	}

	var wg sync.WaitGroup
	for _, parentID := range parentIDs {
		wg.Add(1)
		go func(parentID string) {
			defer wg.Done()
			_, _ = StepProgramOrchestration(root, parentID, policy, cfg)
		}(parentID)
	}
	wg.Wait()

	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	owners := map[string]bool{}
	now := Clock().UTC()
	for _, lease := range leases {
		if programChildLeaseIsActive(lease, now) {
			active++
			owners[lease.ParentSessionID] = true
		}
	}
	if active != 1 || len(owners) != 1 {
		t.Fatalf("active leases = %d owners = %v, want one active owner", active, owners)
	}
}

func TestProgramOrchestrationCapacity(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"a", "b", "c"} {
		scaffoldSpec(t, root, slug, StatusExecuting)
	}
	cfg, policy := programTestPolicy(t)
	cfg.Program.MaxConcurrentSpecs = 2
	parentID := strings.Repeat("1", 32)

	result, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ProgramDecisionStart || !reflect.DeepEqual(result.Decision.Specs, []string{"a", "b"}) {
		t.Fatalf("decision = %#v, want start [a b]", result.Decision)
	}
	active := 0
	for _, lease := range result.Leases {
		if programChildLeaseIsActive(lease, Clock().UTC()) {
			active++
		}
	}
	if active != 2 || len(result.Started) != 2 {
		t.Fatalf("active=%d started=%d, want 2", active, len(result.Started))
	}

	result, err = StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	active = 0
	for _, lease := range result.Leases {
		if programChildLeaseIsActive(lease, Clock().UTC()) {
			active++
		}
	}
	if active != 2 || len(result.Started) != 0 || result.Decision.Action != ProgramDecisionWait {
		t.Fatalf("second step active=%d started=%d decision=%#v, want capacity wait", active, len(result.Started), result.Decision)
	}
}

func TestProgramOrchestrationFrontier(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusComplete)
	scaffoldSpec(t, root, "b", StatusExecuting)
	scaffoldSpec(t, root, "c", StatusExecuting)
	if err := SaveProgram(root, ProgramManifest{Version: ProgramVersion, DependsOn: map[string][]string{
		"b": {"a"},
		"c": {"b"},
	}}); err != nil {
		t.Fatal(err)
	}
	cfg, policy := programTestPolicy(t)
	cfg.Program.MaxConcurrentSpecs = 1
	parentID := strings.Repeat("2", 32)

	first, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Decision.Specs, []string{"b"}) {
		t.Fatalf("first frontier = %v, want [b]", first.Decision.Specs)
	}
	state, err := LoadState(root, "b")
	if err != nil {
		t.Fatal(err)
	}
	state.Status = StatusComplete
	if err := SaveState(root, "b", state); err != nil {
		t.Fatal(err)
	}

	second, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Decision.Specs, []string{"c"}) {
		t.Fatalf("second frontier = %v, want [c]", second.Decision.Specs)
	}
	bySlug := map[string]ProgramChildLease{}
	for _, lease := range second.Leases {
		bySlug[lease.Slug] = lease
	}
	if bySlug["b"].Status != ProgramChildLeaseReleased || !programChildLeaseIsActive(bySlug["c"], Clock().UTC()) {
		t.Fatalf("leases after frontier recompute = %#v", bySlug)
	}
}

func TestProgramOrchestrationEscalate(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	scaffoldSpec(t, root, "b", StatusExecuting)
	if err := SaveProgram(root, ProgramManifest{Version: ProgramVersion, DependsOn: map[string][]string{"b": {"a"}}}); err != nil {
		t.Fatal(err)
	}
	cfg, policy := programTestPolicy(t)
	cfg.Program.MaxConcurrentSpecs = 2
	parentID := strings.Repeat("3", 32)
	lease, err := AcquireProgramChildLease(root, parentID, "a", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureProgramChildSession(root, lease, policy); err != nil {
		t.Fatal(err)
	}
	if _, err := markProgramChildLeaseEscalated(root, parentID, "a"); err != nil {
		t.Fatal(err)
	}

	result, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ProgramDecisionEscalate || !reflect.DeepEqual(result.Decision.Specs, []string{"a"}) || len(result.Started) != 0 {
		t.Fatalf("decision=%#v started=%d, want escalate a without new work", result.Decision, len(result.Started))
	}
	session, err := LoadProgramSession(root, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != OrchestrationSessionFailed {
		t.Fatalf("program session status = %s, want failed", session.Status)
	}
}

func TestProgramOrchestrationPause(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	scaffoldSpec(t, root, "b", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	cfg.Program.MaxConcurrentSpecs = 1
	parentID := strings.Repeat("4", 32)
	first, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Started) != 1 {
		t.Fatalf("started=%d, want 1", len(first.Started))
	}
	if _, err := PauseProgramOrchestration(root, parentID); err != nil {
		t.Fatal(err)
	}
	child, err := LoadOrchestrationSession(root, first.Started[0].ChildSessionID)
	if err != nil {
		t.Fatal(err)
	}
	if child.Status != OrchestrationSessionPaused {
		t.Fatalf("child status = %s, want paused", child.Status)
	}
	second, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Decision.Action != ProgramDecisionWait || len(second.Started) != 0 || len(second.Stepped) != 0 {
		t.Fatalf("paused step decision=%#v started=%d stepped=%d", second.Decision, len(second.Started), len(second.Stepped))
	}
}

func TestProgramOrchestrationCancel(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("5", 32)
	first, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Started) != 1 {
		t.Fatalf("started=%d, want 1", len(first.Started))
	}
	if _, err := CancelProgramOrchestration(root, parentID); err != nil {
		t.Fatal(err)
	}
	child, err := LoadOrchestrationSession(root, first.Started[0].ChildSessionID)
	if err != nil {
		t.Fatal(err)
	}
	if child.Status != OrchestrationSessionCancelling {
		t.Fatalf("child status = %s, want cancelling", child.Status)
	}
	second, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Decision.Action != ProgramDecisionWait || second.Decision.Reason != "program cancelling — cooperative cancel propagated" {
		t.Fatalf("cancel step decision=%#v", second.Decision)
	}
}

func TestProgramOrchestrationRecovery(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("6", 32)
	if _, err := StepProgramOrchestration(root, parentID, policy, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := PauseProgramOrchestration(root, parentID); err != nil {
		t.Fatal(err)
	}
	before, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	after, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Decision, after.Decision) || !reflect.DeepEqual(before.Snapshot, after.Snapshot) {
		t.Fatalf("paused recovery changed:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestProgramOrchestrationComplete(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusComplete)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("7", 32)
	result, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ProgramDecisionComplete {
		t.Fatalf("decision=%#v, want complete", result.Decision)
	}
	session, err := LoadProgramSession(root, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != OrchestrationSessionComplete {
		t.Fatalf("program session status = %s, want complete", session.Status)
	}
}

// TestDriveProgramOrchestrationCrossSpecWalk is the GAP-7 golden test: a
// 3-spec linear program (auth → api → web) of *fresh* specs, each at
// `requirements` with no planning artifacts, is driven to full completion by the
// program driver loop alone. A stub worker does the creative work (authoring
// artifacts, completing execution tasks with evidence). It proves the loop
// re-resolves the frontier and advances to the next spec automatically on child
// completion, with zero model call in core.
func TestDriveProgramOrchestrationCrossSpecWalk(t *testing.T) {
	root := t.TempDir()
	specs := []string{"auth", "api", "web"}
	for _, slug := range specs {
		if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
			t.Fatal(err)
		}
		st := InitialState(slug, slug)
		st.Status = StatusRequirements
		st.Phase = PhaseForStatus(StatusRequirements)
		if err := SaveState(root, slug, &st); err != nil {
			t.Fatal(err)
		}
	}
	// Linear dependency chain: api depends on auth, web depends on api.
	if err := SaveProgram(root, ProgramManifest{Version: ProgramVersion, DependsOn: map[string][]string{
		"api": {"auth"},
		"web": {"api"},
	}}); err != nil {
		t.Fatal(err)
	}

	cfg, policy := programTestPolicy(t)
	policy.ApprovalPolicy = "planning"
	cfg.ApprovalPolicy = "planning"
	cfg.Program.MaxConcurrentSpecs = 1
	parentID := strings.Repeat("8", 32)

	authored := map[string]bool{}
	dispatched := []string{}
	worker := func(d ProgramDriverDispatch) error {
		dec := d.Dispatch.Decision
		dispatched = append(dispatched, d.Slug+":"+string(dec.Action)+":"+dec.TaskID+dec.Artifact)
		switch dec.Action {
		case OrchestrationDispatchAuthor:
			authored[d.Slug+"/"+dec.Artifact] = true
			return os.WriteFile(filepath.Join(SpecDir(root, d.Slug), dec.Artifact), []byte(artifactFixture(dec.Artifact)), 0o644)
		case OrchestrationDispatch:
			// Stand in for the host's execute → verify → approve responsibility.
			// The orchestration core marks the child *session* complete at
			// `verifying` but deliberately never auto-clears the acceptance-evidence
			// gate (`specd approve` Case 2 is validator/host-owned, by design), so a
			// stub worker that only reached `verifying` would stall the program
			// frontier forever. Landing the spec at `complete` with evidence
			// emulates the validator worker that a real host would dispatch.
			st, err := LoadState(root, d.Slug)
			if err != nil {
				return err
			}
			ts := st.Tasks[dec.TaskID]
			ts.Status = TaskComplete
			ts.Verification = &VerificationRecord{Verified: true, ExitCode: 0, Command: "go test ./..."}
			st.Tasks[dec.TaskID] = ts
			st.Status = StatusComplete
			st.Phase = PhaseForStatus(StatusComplete)
			return SaveState(root, d.Slug, st)
		}
		return nil
	}

	result, err := DriveProgramOrchestration(root, parentID, policy, cfg, ProgramDriverOptions{MaxSteps: 400, Worker: worker})
	if err != nil {
		t.Fatalf("program drive failed: %v (dispatched=%v)", err, dispatched)
	}
	if result.Outcome != DriverComplete {
		t.Fatalf("outcome = %s, want complete (dispatched=%v)", result.Outcome, dispatched)
	}

	// Every spec must have authored all three planning artifacts and reached complete.
	for _, slug := range specs {
		for _, artifact := range []string{"requirements.md", "design.md", "tasks.md"} {
			if !authored[slug+"/"+artifact] {
				t.Fatalf("spec %s never authored %s (dispatched=%v)", slug, artifact, dispatched)
			}
		}
		st, err := LoadState(root, slug)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != StatusComplete {
			t.Fatalf("spec %s status = %s, want complete (dispatched=%v)", slug, st.Status, dispatched)
		}
	}

	session, err := LoadProgramSession(root, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != OrchestrationSessionComplete {
		t.Fatalf("program session status = %s, want complete", session.Status)
	}
}

func programTestPolicy(t *testing.T) (OrchestrationCfg, OrchestrationPolicy) {
	t.Helper()
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	t.Cleanup(restore)
	cfg := DefaultConfig.Orchestration
	cfg.Enabled = true
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, policy
}
