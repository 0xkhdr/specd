package cmd_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// Agent-facing briefing budgets. Source of truth: docs/agent-harness-baselines.md
// (spec regression-agent-harness-value, R1). Budgets are upper bounds in bytes;
// the token proxy is bytes/4 (ADR-001). They include headroom over the measured
// baseline so normal spec growth does not flake the test, while a runaway briefing
// (decoration leak, duplicated payload) still trips it.
const (
	budgetContextBytes  = 3584
	budgetNextBytes     = 1536
	budgetDispatchBytes = 4096
)

// TestAgentBriefingBudgets asserts that the machine-readable (--json) briefings
// agents consume stay within their documented byte budget. R1.1 / R1.3.
func TestAgentBriefingBudgets(t *testing.T) {
	build := func(h *th.Harness) string {
		return h.Spec("auth").
			Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
			FullDesign().
			AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
			Status(core.StatusExecuting).
			Build()
	}

	cases := []struct {
		name   string
		budget int
		args   []string
	}{
		{"context", budgetContextBytes, []string{"context"}},
		{"next", budgetNextBytes, []string{"next"}},
		{"dispatch", budgetDispatchBytes, []string{"dispatch"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := th.New(t)
			slug := build(h)
			args := append(append([]string{}, tc.args...), slug, "--json")
			res := h.RunExpect(core.ExitOK, args[0], args[1:]...)
			if n := len(res.Stdout); n > tc.budget {
				t.Errorf("%s --json = %d bytes, over budget %d (token proxy ~%d > ~%d)",
					tc.name, n, tc.budget, n/4, tc.budget/4)
			}
		})
	}
}

// TestAgentBriefingNoDecoration asserts agent output (--json) carries no ANSI
// escape sequences or other human-only decoration. R1.2.
func TestAgentBriefingNoDecoration(t *testing.T) {
	h := th.New(t)
	slug := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	for _, cmd := range [][]string{{"context"}, {"next"}, {"dispatch"}, {"status"}} {
		args := append(append([]string{}, cmd...), slug, "--json")
		res := h.RunExpect(core.ExitOK, args[0], args[1:]...)
		if strings.Contains(res.Stdout, "\x1b[") {
			t.Errorf("%s --json contains ANSI escape sequence", cmd[0])
		}
	}
}
