package cmd_test

// Concern (cross-cutting): scaffolding round-trip — a draft authored faithfully
// to the scaffolding brief is gate-clean end to end.

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestFaithfulDraftPassesCheck builds a spec draft shaped exactly as the
// scaffolding brief prescribes — EARS-form acceptance criteria, the full set of
// required design headers, and 7-key tasks in a valid wave DAG — and asserts the
// full gate pipeline passes. This is the round-trip guarantee behind the
// one-shot scaffolding: a draft authored faithfully to the brief is gate-clean.
func TestFaithfulDraftPassesCheck(t *testing.T) {
	h := th.New(t)

	// Author the draft to the brief's shape (gate-clean artifacts).
	h.Spec("feat").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusTasks).
		Build()

	h.RunExpect(core.ExitOK, "check", "feat")
}
