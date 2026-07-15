package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

func TestBrainResumeHappyPathNoDoubleDispatch(t *testing.T) {
	h := testharness.New(t)
	slug := h.Spec("resume-spec").
		Req("resume", "As an operator I can resume sessions.", "THE SYSTEM SHALL resume sessions.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := strings.Repeat("1", 32)
	step := host.StartSpec(slug, sessionID)
	if step.Decision.Action != core.OrchestrationDispatch {
		t.Fatalf("start decision = %s, want dispatch", step.Decision.Action)
	}
	before := eventFileCount(t, h.Root, sessionID)
	h.RunExpect(core.ExitOK, "brain", "pause", "--session", sessionID)
	paused := h.RunExpect(core.ExitOK, "brain", "resume", "--session", sessionID, "--json")
	var session core.OrchestrationSession
	if err := json.Unmarshal([]byte(paused.Stdout), &session); err != nil {
		t.Fatalf("resume JSON: %v\n%s", err, paused.Stdout)
	}
	if session.Status != core.OrchestrationSessionRunning {
		t.Fatalf("session status = %s, want running", session.Status)
	}
	after := eventFileCount(t, h.Root, sessionID)
	if after != before {
		t.Fatalf("resume wrote dispatch events: before=%d after=%d", before, after)
	}
}

func TestBrainResumeUnknownSession(t *testing.T) {
	h := testharness.New(t)
	res := h.Run("brain", "resume", "--session", strings.Repeat("2", 32))
	if res.Code != core.ExitNotFound {
		t.Fatalf("exit = %d, want %d; out=%s", res.Code, core.ExitNotFound, res.Out())
	}
	if !strings.Contains(res.Out(), "session not found") {
		t.Fatalf("missing clear not-found message: %s", res.Out())
	}
}

func TestBrainResumeAlreadyCompleteSessionNoop(t *testing.T) {
	h := testharness.New(t)
	slug := h.Spec("resume-complete").
		Req("resume", "As an operator I can resume safely.", "THE SYSTEM SHALL resume complete sessions safely.").
		FullDesign().
		Status(core.StatusVerifying).
		Orchestrated().
		Build()
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := strings.Repeat("3", 32)
	if _, err := core.StartOrchestrationSession(h.Root, slug, sessionID, "test", host.Policy); err != nil {
		t.Fatalf("StartOrchestrationSession: %v", err)
	}
	if _, err := core.StepOrchestration(h.Root, slug, sessionID, host.Policy, host.Cfg); err != nil {
		t.Fatalf("StepOrchestration: %v", err)
	}
	res := h.RunExpect(core.ExitOK, "brain", "resume", "--session", sessionID, "--json")
	var session core.OrchestrationSession
	if err := json.Unmarshal([]byte(res.Stdout), &session); err != nil {
		t.Fatalf("resume JSON: %v\n%s", err, res.Stdout)
	}
	if session.Status != core.OrchestrationSessionComplete {
		t.Fatalf("session status = %s, want complete", session.Status)
	}
}

// TestBrainResumeListFiltersOrdersAndExcludes (R5 W1.7): `brain resume --list`
// returns running and paused sessions most-recent-first, excludes terminal
// sessions, and honors --max-age-minutes.
func TestBrainResumeListFiltersOrdersAndExcludes(t *testing.T) {
	h := testharness.New(t)
	host := testharness.NewFakeOrchestrationHost(h)

	execR := h.Spec("resume-list-run").
		Req("resume", "As a host I discover running sessions.", "THE SYSTEM SHALL list running sessions.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()
	execP := h.Spec("resume-list-pause").
		Req("resume", "As a host I discover paused sessions.", "THE SYSTEM SHALL list paused sessions.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()

	running := strings.Repeat("1", 32)
	paused := strings.Repeat("2", 32)
	done := strings.Repeat("3", 32)

	if _, err := core.StartOrchestrationSession(h.Root, execR, running, "test", host.Policy); err != nil {
		t.Fatalf("start running: %v", err)
	}
	h.Clock.Advance(time.Minute)
	if _, err := core.StartOrchestrationSession(h.Root, execP, paused, "test", host.Policy); err != nil {
		t.Fatalf("start paused: %v", err)
	}
	if _, err := core.PauseOrchestration(h.Root, paused); err != nil {
		t.Fatalf("pause: %v", err)
	}

	// A terminal (complete) session on a verifying spec — must be excluded.
	vrf := h.Spec("resume-list-done").
		Req("resume", "As a host I exclude terminal sessions.", "THE SYSTEM SHALL exclude complete sessions.").
		FullDesign().
		Status(core.StatusVerifying).
		Orchestrated().
		Build()
	if _, err := core.StartOrchestrationSession(h.Root, vrf, done, "test", host.Policy); err != nil {
		t.Fatalf("start done: %v", err)
	}
	if _, err := core.StepOrchestration(h.Root, vrf, done, host.Policy, host.Cfg); err != nil {
		t.Fatalf("step done: %v", err)
	}

	// Full list: running (newest) before paused; complete excluded.
	all := decodeResumeList(t, h.RunExpect(core.ExitOK, "brain", "resume", "--list", "--json"))
	if len(all) != 2 {
		t.Fatalf("list = %d entries, want 2 (running+paused): %#v", len(all), all)
	}
	if all[0].SessionID != paused {
		// paused was updated last (its pause bumped updatedAt), so it sorts first.
		t.Fatalf("order wrong, head = %s: %#v", all[0].SessionID, all)
	}
	for _, s := range all {
		if s.SessionID == done {
			t.Fatalf("complete session must be excluded: %#v", all)
		}
		if s.Status != core.OrchestrationSessionRunning && s.Status != core.OrchestrationSessionPaused {
			t.Fatalf("non-resumable status leaked: %#v", s)
		}
	}

	// Hard age filter excludes everything → empty array, exit 0.
	h.Clock.Advance(10 * time.Minute)
	empty := decodeResumeList(t, h.RunExpect(core.ExitOK, "brain", "resume", "--list", "--max-age-minutes", "1", "--json"))
	if len(empty) != 0 {
		t.Fatalf("max-age filter should empty the list, got %#v", empty)
	}
}

func decodeResumeList(t *testing.T, res testharness.Result) []core.ResumableSession {
	t.Helper()
	var out []core.ResumableSession
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("resume --list JSON: %v\n%s", err, res.Stdout)
	}
	return out
}

func eventFileCount(t *testing.T, root, sessionID string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".specd", "runtime", "sessions", sessionID, "events"))
	if err != nil {
		t.Fatalf("ReadDir events: %v", err)
	}
	return len(entries)
}
