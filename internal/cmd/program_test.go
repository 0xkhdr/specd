package cmd_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

func TestBrainProgramCLIStartStatusAndControls(t *testing.T) {
	h := testharness.New(t)
	h.Spec("alpha").Req("alpha", "As a user, I want alpha.", "THE SYSTEM SHALL satisfy alpha.").FullDesign().Status(core.StatusComplete).Build()
	h.Spec("beta").Req("beta", "As a user, I want beta.", "THE SYSTEM SHALL satisfy beta.").FullDesign().Status(core.StatusExecuting).Build()
	if err := core.SaveProgram(h.Root, core.ProgramManifest{Version: core.ProgramVersion, DependsOn: map[string][]string{"beta": {"alpha"}}}); err != nil {
		t.Fatal(err)
	}

	sessionID := strings.Repeat("5", 32)
	policyArgs := []string{"--program", "--session", sessionID, "--approval-policy", "manual", "--max-workers", "2", "--max-retries", "1", "--timeout-seconds", "7200", "--json"}
	started := h.RunExpect(core.ExitOK, "brain", append([]string{"start"}, policyArgs...)...)
	var step core.ProgramStepResult
	if err := json.Unmarshal([]byte(started.Stdout), &step); err != nil {
		t.Fatal(err)
	}
	if step.Decision.Action != core.ProgramDecisionStart || !reflect.DeepEqual(step.Decision.Specs, []string{"beta"}) || len(step.Started) != 1 {
		t.Fatalf("program start=%#v, want start beta", step)
	}

	status := h.RunExpect(core.ExitOK, "brain", "status", "--program", "--session", sessionID, "--json")
	var report core.ProgramStatusReport
	if err := json.Unmarshal([]byte(status.Stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Session.ParentSessionID != sessionID || report.Counts.Total != 2 || report.Counts.Complete != 1 || report.Counts.Active != 1 {
		t.Fatalf("status report=%#v", report)
	}
	if len(report.Frontier) != 0 || report.Snapshot.CriticalPath[0] != "alpha" || report.Snapshot.CriticalPath[1] != "beta" {
		t.Fatalf("frontier=%v critical=%v", report.Frontier, report.Snapshot.CriticalPath)
	}

	paused := h.RunExpect(core.ExitOK, "brain", "pause", "--program", "--session", sessionID, "--json")
	var pausedSession core.ProgramSession
	if err := json.Unmarshal([]byte(paused.Stdout), &pausedSession); err != nil {
		t.Fatal(err)
	}
	if pausedSession.Status != core.OrchestrationSessionPaused {
		t.Fatalf("paused status=%s", pausedSession.Status)
	}
	resumed := h.RunExpect(core.ExitOK, "brain", "resume", "--program", "--session", sessionID, "--json")
	var resumedSession core.ProgramSession
	if err := json.Unmarshal([]byte(resumed.Stdout), &resumedSession); err != nil {
		t.Fatal(err)
	}
	if resumedSession.Status != core.OrchestrationSessionRunning {
		t.Fatalf("resumed status=%s", resumedSession.Status)
	}
	cancelled := h.RunExpect(core.ExitOK, "brain", "cancel", "--program", "--session", sessionID, "--json")
	var cancelledSession core.ProgramSession
	if err := json.Unmarshal([]byte(cancelled.Stdout), &cancelledSession); err != nil {
		t.Fatal(err)
	}
	if cancelledSession.Status != core.OrchestrationSessionCancelling {
		t.Fatalf("cancelled status=%s", cancelledSession.Status)
	}
}

func TestProgramCommandJSONRemainsCompatible(t *testing.T) {
	h := testharness.New(t)
	h.Spec("alpha").Req("alpha", "As a user, I want alpha.", "THE SYSTEM SHALL satisfy alpha.").FullDesign().Status(core.StatusComplete).Build()
	h.Spec("beta").Req("beta", "As a user, I want beta.", "THE SYSTEM SHALL satisfy beta.").FullDesign().Status(core.StatusExecuting).Build()
	if err := core.SaveProgram(h.Root, core.ProgramManifest{Version: core.ProgramVersion, DependsOn: map[string][]string{"beta": {"alpha"}}}); err != nil {
		t.Fatal(err)
	}

	res := h.RunExpect(core.ExitOK, "program", "--json")
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out["kind"] != "program" || int(out["count"].(float64)) != 2 {
		t.Fatalf("program json=%v", out)
	}
	frontier, ok := out["frontier"].([]any)
	if !ok || len(frontier) != 1 || frontier[0] != "beta" {
		t.Fatalf("frontier=%#v, want [beta]", out["frontier"])
	}
}

func TestBrainProgramRejectsUnknownApprovalPolicy(t *testing.T) {
	h := testharness.New(t)
	h.Spec("alpha").Req("alpha", "As a user, I want alpha.", "THE SYSTEM SHALL satisfy alpha.").FullDesign().Status(core.StatusExecuting).Build()
	res := h.Run("brain", "start", "--program", "--session", strings.Repeat("6", 32), "--approval-policy", "auto", "--max-workers", "1", "--max-retries", "1", "--timeout-seconds", "7200", "--json")
	if res.Code != core.ExitGate || !strings.Contains(res.Out(), "unsupported approval policy") {
		t.Fatalf("exit=%d out=%s, want fail-closed policy rejection", res.Code, res.Out())
	}
}
