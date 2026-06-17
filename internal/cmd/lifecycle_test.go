package cmd_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestFullLifecycle walks a spec from requirements through to complete, driving
// every phase gate via the real CLI: approve ratchet → execute → verify →
// evidence-gated completion → verifying → accept. This is the end-to-end proof
// that the phase machine, DAG frontier, verification and state persistence all
// compose correctly.
func TestFullLifecycle(t *testing.T) {
	h := th.New(t)
	slug := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
		AddTask(th.TaskSpec{ID: "T2", Title: "Wire session", Wave: 2, Depends: []string{"T1"}, Verify: "true", Requirements: []int{1}}).
		Status(core.StatusRequirements).
		Build()

	// Planning ratchet: requirements → design → tasks → executing.
	h.RunExpect(core.ExitOK, "approve", slug)
	h.State(slug).Status(core.StatusDesign)
	h.RunExpect(core.ExitOK, "approve", slug)
	h.State(slug).Status(core.StatusTasks)
	h.RunExpect(core.ExitOK, "approve", slug)
	h.State(slug).Status(core.StatusExecuting)

	// Frontier is just T1 (T2 depends on it).
	res := h.RunExpect(core.ExitOK, "next", slug)
	if !strings.Contains(res.Stdout, "NEXT TASK: T1") {
		t.Fatalf("expected T1 as frontier, got %q", res.Stdout)
	}

	// Cannot complete T2 before T1 (dependency gate).
	h.RunExpect(core.ExitGate, "task", slug, "T2", "--status", "complete", "--unverified", "--evidence", "x")

	// Verify + complete T1.
	h.RunExpect(core.ExitOK, "verify", slug, "T1")
	h.State(slug).Raw() // ensure loadable
	h.RunExpect(core.ExitOK, "task", slug, "T1", "--status", "complete")
	h.State(slug).TaskStatus("T1", core.TaskComplete)

	// Now T2 is runnable; verify + complete it.
	h.RunExpect(core.ExitOK, "verify", slug, "T2")
	h.RunExpect(core.ExitOK, "task", slug, "T2", "--status", "complete")

	// All tasks done → spec auto-derives to verifying.
	h.State(slug).TaskStatus("T2", core.TaskComplete).Status(core.StatusVerifying)

	// check is still green after completion (evidence + sync gates).
	h.RunExpect(core.ExitOK, "check", slug)

	// Accept verification → complete.
	h.RunExpect(core.ExitOK, "approve", slug)
	h.State(slug).Status(core.StatusComplete).Phase(core.PhaseReflect)

	// tasks.md checkboxes were rewritten to reflect completion.
	h.AssertFileContains(".specd/specs/auth/tasks.md", "[x] T1")
	h.AssertFileContains(".specd/specs/auth/tasks.md", "[x] T2")
}

// TestLifecycleReportReflectsFinalState drives the spec all the way to complete
// and then runs `report`, proving R3.3: the report renders the final state. It
// also exercises the tail of the new→…→report walk via the real CLI.
func TestLifecycleReportReflectsFinalState(t *testing.T) {
	h := th.New(t)
	slug := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	h.RunExpect(core.ExitOK, "verify", slug, "T1")
	h.RunExpect(core.ExitOK, "task", slug, "T1", "--status", "complete")
	h.State(slug).Status(core.StatusVerifying)
	h.RunExpect(core.ExitOK, "approve", slug)
	h.State(slug).Status(core.StatusComplete)

	// report (markdown to stdout) must reflect the completed spec.
	res := h.RunExpect(core.ExitOK, "report", slug)
	for _, want := range []string{"auth", "T1"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("report stdout missing %q:\n%s", want, res.Stdout)
		}
	}
}

// TestLifecycleOutOfOrderBlocked drives R3.2: a lifecycle step attempted out of
// order is refused (non-zero), not silently performed. Flipping a task to
// complete while the spec is still in requirements (no executing phase, no
// verification) must be blocked.
func TestLifecycleOutOfOrderBlocked(t *testing.T) {
	h := th.New(t)
	slug := h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement login", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusRequirements).
		Build()

	if res := h.Run("task", slug, "T1", "--status", "complete"); res.OK() {
		t.Errorf("task complete in requirements phase should be blocked, got exit 0:\n%s", res.Out())
	}
	// State must be untouched by the rejected step.
	h.State(slug).Status(core.StatusRequirements)
}

// TestExitCodeTaxonomyCLI is the R4 golden table at the dispatch level: each
// scenario maps to a stable, distinct exit code. This freezes the script-facing
// contract end to end (R4.1 distinct codes, R4.2 success → 0).
func TestExitCodeTaxonomyCLI(t *testing.T) {
	cases := []struct {
		name string
		args []string                 // command + args
		set  func(h *th.Harness) string // builds spec, returns slug to substitute for "" placeholder
		want int
	}{
		{
			name: "success: check clean spec",
			args: []string{"check", ""},
			set: func(h *th.Harness) string {
				return h.Spec("ok").
					Req("R", "story", "THE SYSTEM SHALL work.").
					FullDesign().
					AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
					Status(core.StatusExecuting).
					Build()
			},
			want: core.ExitOK,
		},
		{
			name: "validation failure: check non-EARS spec",
			args: []string{"check", ""},
			set: func(h *th.Harness) string {
				return h.Spec("bad").Req("R", "story", "this is not ears").Status(core.StatusRequirements).Build()
			},
			want: core.ExitGate,
		},
		{
			name: "gate block: out-of-order task complete",
			args: []string{"task", "", "T1", "--status", "complete"},
			set: func(h *th.Harness) string {
				return h.Spec("gated").
					Req("R", "story", "THE SYSTEM SHALL work.").
					FullDesign().
					AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
					Status(core.StatusRequirements).
					Build()
			},
			want: core.ExitGate,
		},
		{
			name: "not found: report missing spec",
			args: []string{"report", "does-not-exist"},
			set:  func(h *th.Harness) string { return "" },
			want: core.ExitNotFound,
		},
		{
			name: "usage: report with no slug",
			args: []string{"report"},
			set:  func(h *th.Harness) string { return "" },
			want: core.ExitUsage,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := th.New(t)
			slug := c.set(h)
			args := append([]string(nil), c.args...)
			for i := range args {
				if args[i] == "" {
					args[i] = slug
				}
			}
			h.RunExpect(c.want, args[0], args[1:]...)
		})
	}
}

// TestVerifyExecution exercises the real shell execution path of `verify`,
// including pass, fail, and the verified-record gate on completion.
func TestVerifyExecution(t *testing.T) {
	t.Run("passing command records verified", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").
			Req("R", "story", "THE SYSTEM SHALL work.").
			AddTask(th.TaskSpec{ID: "T1", Verify: "exit 0"}).
			Status(core.StatusExecuting).
			Build()
		res := h.RunExpect(core.ExitOK, "verify", "auth", "T1")
		if !strings.Contains(res.Stdout, "verified") {
			t.Errorf("expected verified mark, got %q", res.Stdout)
		}
		rec := h.State("auth").Raw().Tasks["T1"].Verification
		if rec == nil || !rec.Verified || rec.ExitCode != 0 {
			t.Errorf("verification record not recorded as passing: %+v", rec)
		}
	})

	t.Run("failing command is a gate exit and blocks completion", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").
			Req("R", "story", "THE SYSTEM SHALL work.").
			AddTask(th.TaskSpec{ID: "T1", Verify: "exit 7"}).
			Status(core.StatusExecuting).
			Build()
		res := h.RunExpect(core.ExitGate, "verify", "auth", "T1")
		if !strings.Contains(res.Stdout, "FAILED") {
			t.Errorf("expected FAILED mark, got %q", res.Stdout)
		}
		if rec := h.State("auth").Raw().Tasks["T1"].Verification; rec == nil || rec.ExitCode != 7 {
			t.Errorf("expected recorded exit 7, got %+v", rec)
		}
		// Completion is refused because the record is not verified.
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "complete")
	})

	t.Run("captures git HEAD when in a repo", func(t *testing.T) {
		h := th.New(t)
		h.InitGit() // skips if git absent
		h.Spec("auth").
			Req("R", "story", "THE SYSTEM SHALL work.").
			AddTask(th.TaskSpec{ID: "T1", Verify: "true"}).
			Status(core.StatusExecuting).
			Build()
		head := h.GitCommitAll("seed spec")
		h.RunExpect(core.ExitOK, "verify", "auth", "T1")
		rec := h.State("auth").Raw().Tasks["T1"].Verification
		if rec.GitHead == nil || *rec.GitHead != head {
			t.Errorf("gitHead = %v, want %q", rec.GitHead, head)
		}
	})
}

// TestVerifyCriterion covers the per-criterion acceptance proof path.
func TestVerifyCriterion(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").
		Req("Login", "story", "THE SYSTEM SHALL authenticate users.").
		Status(core.StatusVerifying).
		Build()

	h.RunExpect(core.ExitOK, "verify", "auth", "--criterion", "1.1", "--status", "pass", "--evidence", "manual test ok")
	h.State("auth").AcceptanceStatus("1.1", "pass")

	// A failing criterion exits gate(1) but is still recorded.
	h.RunExpect(core.ExitGate, "verify", "auth", "--criterion", "1.2", "--status", "fail", "--evidence", "broke")
	h.State("auth").AcceptanceStatus("1.2", "fail")

	// Undefined requirement number is rejected.
	h.RunExpect(core.ExitGate, "verify", "auth", "--criterion", "9.1", "--status", "pass", "--evidence", "x")

	// Bad criterion format / missing evidence are usage errors.
	h.RunExpect(core.ExitUsage, "verify", "auth", "--criterion", "abc", "--status", "pass", "--evidence", "x")
	h.RunExpect(core.ExitUsage, "verify", "auth", "--criterion", "1.1", "--status", "pass")
}
