package cmd_test

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// command_render_cov_test.go drives the read-only render commands (status,
// replay, mode --recommend) through the real CLI harness, covering their text
// and JSON branches without mutating spec state.

func buildExecutingSpec(t *testing.T) (*th.Harness, string) {
	t.Helper()
	h := th.New(t)
	slug := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
		AddTask(th.TaskSpec{ID: "T2", Title: "Wire session", Wave: 2, Depends: []string{"T1"}, Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
	return h, slug
}

func TestStatusCommandRenders(t *testing.T) {
	h, slug := buildExecutingSpec(t)
	h.RunExpect(core.ExitOK, "status", slug)
	h.RunExpect(core.ExitOK, "status", slug, "--json")
	h.RunExpect(core.ExitOK, "status", slug, "--all")
}

func TestReplayCommandRenders(t *testing.T) {
	h, slug := buildExecutingSpec(t)
	h.RunExpect(core.ExitOK, "replay", slug)
	h.RunExpect(core.ExitOK, "replay", slug, "--json")
}

func TestModeRecommendRenders(t *testing.T) {
	h, slug := buildExecutingSpec(t)
	h.RunExpect(core.ExitOK, "mode", slug, "--recommend")
	h.RunExpect(core.ExitOK, "mode", slug, "--recommend", "--json")
}

func TestProgramCommandRendersAndLinks(t *testing.T) {
	h := th.New(t)
	a := h.Spec("alpha").
		Req("A", "story a", "THE SYSTEM SHALL a.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "do a", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
	b := h.Spec("beta").
		Req("B", "story b", "THE SYSTEM SHALL b.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "do b", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	// Self-dependency and missing-spec are rejected (non-zero exit).
	if res := h.Run("program", "link", a, "--on", a); res.Code == core.ExitOK {
		t.Fatalf("self-dependency link should fail, got OK: %s", res.Out())
	}
	if res := h.Run("program", "link", a, "--on", "ghost"); res.Code == core.ExitOK {
		t.Fatalf("link to missing spec should fail, got OK: %s", res.Out())
	}

	// Link alpha → beta, then render text and JSON.
	h.RunExpect(core.ExitOK, "program", "link", a, "--on", b)
	h.RunExpect(core.ExitOK, "program")
	h.RunExpect(core.ExitOK, "program", "--json")
}
