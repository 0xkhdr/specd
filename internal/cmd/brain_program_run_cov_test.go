package cmd_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

// TestBrainRunProgramJSONAndMaxSteps covers brainRunProgram's --json output and
// the bounded --max-steps path (the existing program test exercises only the
// default text-output completion).
func TestBrainRunProgramJSONAndMaxSteps(t *testing.T) {
	h := testharness.New(t)
	h.Init()
	for _, slug := range []string{"pj-a", "pj-b"} {
		h.Spec(slug).
			Req("pj", "As an operator I drive a program.", "THE SYSTEM SHALL drive specs.").
			FullDesign().
			Status(core.StatusExecuting).
			Orchestrated().
			AddTask(testharness.TaskSpec{ID: "T1"}).
			Build()
	}

	rec := newRecordingRunner(h.Root, "")
	rec.noComplete = true // bounded run: stop at max-steps without completing
	defer cmd.SetBrainRunner(rec)()

	sessionID := repeat("c")
	res := h.RunExpect(core.ExitOK, "brain", "run", "--program",
		"--session", sessionID, "--worker-cmd", "true",
		"--max-workers", "1", "--max-steps", "2", "--json")
	if !strings.Contains(res.Out(), sessionID) {
		t.Fatalf("program --json output missing session id: %s", res.Out())
	}
}
