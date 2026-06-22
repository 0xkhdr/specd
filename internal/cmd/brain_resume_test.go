package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func eventFileCount(t *testing.T, root, sessionID string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".specd", "runtime", "sessions", sessionID, "events"))
	if err != nil {
		t.Fatalf("ReadDir events: %v", err)
	}
	return len(entries)
}
