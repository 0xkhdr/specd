package core

import (
	"strings"
	"testing"
	"time"
)

func planningSnapshot(status SpecStatus, authoring *OrchestrationAuthoring, ready bool) OrchestrationSnapshot {
	s := validOrchestrationSnapshot()
	s.Status = status
	s.Phase = PhaseForStatus(status)
	s.Authoring = authoring
	s.PlanningReady = ready
	return s
}

func TestSenseAuthoringFrontier(t *testing.T) {
	// Empty requirements → authoring item, not ready.
	work, ready := senseAuthoring(StatusRequirements, nil, nil, ParsedTasks{})
	if work == nil || ready {
		t.Fatalf("empty requirements: want authoring item, got work=%v ready=%v", work, ready)
	}
	if work.WorkID != "A1" || work.Artifact != "requirements.md" || work.Role != "builder" {
		t.Fatalf("unexpected authoring item: %#v", work)
	}

	// Non-planning status → no authoring frontier.
	if work, ready := senseAuthoring(StatusExecuting, nil, nil, ParsedTasks{}); work != nil || ready {
		t.Fatalf("executing: want no authoring frontier, got work=%v ready=%v", work, ready)
	}
}

func TestDecideDispatchAuthoringUnderPlanning(t *testing.T) {
	policy := validOrchestrationPolicy()
	policy.ApprovalPolicy = "planning"
	work := &OrchestrationAuthoring{WorkID: "A1", Artifact: "requirements.md", Gate: "ears", Role: "builder", Issues: []string{"requirements.md missing or empty"}}
	snapshot := planningSnapshot(StatusRequirements, work, false)

	decision, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != OrchestrationDispatchAuthor {
		t.Fatalf("action = %s, want dispatch-authoring", decision.Action)
	}
	if decision.TaskID != "A1" || decision.Artifact != "requirements.md" || decision.Attempt != 1 {
		t.Fatalf("unexpected decision: %#v", decision)
	}

	// Already leased → wait (one worker per authoring artifact).
	snapshot.ActiveLeases = []OrchestrationLeaseSnapshot{{WorkerID: "pinky-a1", TaskID: "A1", Attempt: 1, LeaseUntil: snapshot.SessionExpiresAt}}
	policy.MaxWorkers = 4
	decision, err = DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != OrchestrationWait {
		t.Fatalf("leased authoring action = %s, want wait", decision.Action)
	}
}

func TestDecideAuthoringManualRequestsApproval(t *testing.T) {
	policy := validOrchestrationPolicy() // default ApprovalPolicy == "manual"
	work := &OrchestrationAuthoring{WorkID: "A2", Artifact: "design.md", Gate: "design", Role: "builder", Issues: []string{"design.md missing or empty"}}
	snapshot := planningSnapshot(StatusDesign, work, false)

	decision, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != OrchestrationRequestApproval {
		t.Fatalf("manual authoring action = %s, want request-approval", decision.Action)
	}
}

func TestDecideAdvancePhaseWhenReady(t *testing.T) {
	policy := validOrchestrationPolicy()
	policy.ApprovalPolicy = "session"
	snapshot := planningSnapshot(StatusDesign, nil, true)

	decision, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != OrchestrationAdvancePhase || decision.Artifact != "design.md" {
		t.Fatalf("unexpected decision: %#v", decision)
	}

	// Manual policy holds for human approval instead of ratcheting.
	policy.ApprovalPolicy = "manual"
	decision, err = DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != OrchestrationRequestApproval {
		t.Fatalf("manual ready action = %s, want request-approval", decision.Action)
	}
}

func TestBuildAuthoringMissionValidates(t *testing.T) {
	root := writePinkySpec(t)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	mission, err := BuildAuthoringMission(root, "demo", strings.Repeat("3", 32), "a1-a1", "requirements.md", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if mission.TaskID != "A1" || mission.VerifyCommand != "specd check demo" {
		t.Fatalf("unexpected mission: %#v", mission)
	}
	if mission.Authority.ReadOnly || len(mission.Files) != 1 {
		t.Fatalf("unexpected authority/files: %#v", mission)
	}
	joined := missionContextTestString(mission.ContextManifest)
	if !strings.Contains(joined, ".specd/skills/specd-requirements/SKILL.md") || strings.Contains(joined, ".specd/skills/specd-execute/SKILL.md") {
		t.Fatalf("unexpected authoring context manifest:\n%s", joined)
	}
	if err := validatePinkyMission(mission); err != nil {
		t.Fatalf("mission invalid: %v", err)
	}
}
