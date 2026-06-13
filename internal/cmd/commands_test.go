package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// validSpec seeds a gate-clean spec at the given status with one builder task.
func validSpec(h *th.Harness, slug string, status core.SpecStatus) string {
	return h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(status).
		Build()
}

func TestNew(t *testing.T) {
	t.Run("creates spec and artifacts", func(t *testing.T) {
		h := th.New(t)
		res := h.RunExpect(core.ExitOK, "new", "auth")
		if !strings.Contains(res.Stdout, "created spec 'auth'") {
			t.Errorf("missing creation notice: %q", res.Stdout)
		}
		h.AssertFileExists(".specd/specs/auth/state.json")
		h.AssertFileExists(".specd/specs/auth/requirements.md")
		h.State("auth").Status(core.StatusRequirements).Phase(core.PhaseAnalyze)
	})

	t.Run("custom title", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitOK, "new", "auth", "--title", "Auth Flow")
		if got := h.State("auth").Raw().Title; got != "Auth Flow" {
			t.Errorf("title = %q, want %q", got, "Auth Flow")
		}
	})

	t.Run("duplicate is a gate error", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitOK, "new", "auth")
		h.RunExpect(core.ExitGate, "new", "auth")
	})

	t.Run("path-traversal slug rejected", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "new", "../../etc/passwd")
	})

	t.Run("missing slug is usage error", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "new")
	})
}

func TestCheck(t *testing.T) {
	t.Run("clean spec passes", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusTasks)
		res := h.RunExpect(core.ExitOK, "check", "auth")
		if !strings.Contains(res.Stdout, "check passed") {
			t.Errorf("expected pass notice, got %q", res.Stdout)
		}
	})

	t.Run("invalid EARS fails with violations", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").
			Req("Bad", "As a user", "this is not an EARS sentence").
			FullDesign().
			AddTask(th.TaskSpec{ID: "T1"}).
			Status(core.StatusTasks).
			Build()
		res := h.RunExpect(core.ExitGate, "check", "auth")
		if !strings.Contains(res.Stderr, "ears") {
			t.Errorf("expected ears violation, got %q", res.Stderr)
		}
	})

	t.Run("json output", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusTasks)
		res := h.RunExpect(core.ExitOK, "check", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"ok\": true") {
			t.Errorf("expected ok:true json, got %q", res.Stdout)
		}
	})

	t.Run("unknown spec is not-found", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitNotFound, "check", "ghost")
	})

	t.Run("traversal slug is usage error", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "check", "../../../etc")
	})
}

func TestNextAndDispatch(t *testing.T) {
	t.Run("next surfaces frontier task", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		res := h.RunExpect(core.ExitOK, "next", "auth")
		if !strings.Contains(res.Stdout, "NEXT TASK: T1") {
			t.Errorf("expected T1, got %q", res.Stdout)
		}
	})

	t.Run("dispatch json lists packets", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		res := h.RunExpect(core.ExitOK, "dispatch", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"id\": \"T1\"") {
			t.Errorf("expected T1 packet, got %q", res.Stdout)
		}
	})

	t.Run("gated spec blocks next", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").
			Req("R", "story", "THE SYSTEM SHALL work.").
			AddTask(th.TaskSpec{ID: "T1"}).
			Status(core.StatusExecuting).
			Gate(core.GateAwaitingApproval).
			Build()
		h.RunExpect(core.ExitGate, "next", "auth")
		// --force overrides the gate.
		h.RunExpect(core.ExitOK, "next", "auth", "--force")
	})

	t.Run("dispatch unknown spec not-found", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitNotFound, "dispatch", "ghost")
	})
}

func TestTaskStatusTransitions(t *testing.T) {
	t.Run("running then blocked then back to running", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)

		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running")
		h.State("auth").TaskStatus("T1", core.TaskRunning)

		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "blocked", "--reason", "waiting on api")
		h.State("auth").TaskStatus("T1", core.TaskBlocked).HasBlocker("T1").Status(core.StatusBlocked)

		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running")
		h.State("auth").TaskStatus("T1", core.TaskRunning).NoBlockers()
	})

	t.Run("blocked without reason is gate error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "blocked")
	})

	t.Run("complete without verification is gate error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "complete")
	})

	t.Run("complete --unverified requires evidence", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "complete", "--unverified")
		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "complete", "--unverified", "--evidence", "manual proof")
		h.State("auth").TaskStatus("T1", core.TaskComplete).TaskEvidence("T1", "manual proof")
	})

	t.Run("invalid status is usage error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitUsage, "task", "auth", "T1", "--status", "frobnicate")
	})

	t.Run("unknown task is not-found", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitNotFound, "task", "auth", "T99", "--status", "running")
	})
}

func TestApprovePlanningRatchet(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").
		Req("Login", "As a user, I want to log in", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusRequirements).
		Build()

	h.RunExpect(core.ExitOK, "approve", "auth")
	h.State("auth").Status(core.StatusDesign).Phase(core.PhasePlan)

	h.RunExpect(core.ExitOK, "approve", "auth")
	h.State("auth").Status(core.StatusTasks)

	h.RunExpect(core.ExitOK, "approve", "auth")
	h.State("auth").Status(core.StatusExecuting).Phase(core.PhaseExecute)
}

func TestApproveBlockedByGate(t *testing.T) {
	h := th.New(t)
	// Requirements with a non-EARS criterion: approve must refuse to advance.
	h.Spec("auth").
		Req("Bad", "As a user", "garbage criterion").
		Status(core.StatusRequirements).
		Build()
	res := h.RunExpect(core.ExitGate, "approve", "auth")
	if !strings.Contains(res.Stderr, "cannot approve") {
		t.Errorf("expected refusal, got %q", res.Stderr)
	}
	h.State("auth").Status(core.StatusRequirements) // unchanged
}

func TestMidreqGating(t *testing.T) {
	t.Run("high impact sets approval gate", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitOK, "midreq", "auth", "Add OAuth", "--impact", "high")
		h.State("auth").Gate(core.GateAwaitingApproval).Turn(1)
		h.AssertFileContains(".specd/specs/auth/mid-requirements.md", "Add OAuth")

		// approve clears the gate.
		h.RunExpect(core.ExitOK, "approve", "auth")
		h.State("auth").Gate(core.GateNone)
	})

	t.Run("low impact does not gate", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitOK, "midreq", "auth", "tweak copy", "--impact", "low")
		h.State("auth").Gate(core.GateNone).Turn(1)
	})

	t.Run("invalid impact is usage error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitUsage, "midreq", "auth", "x", "--impact", "huge")
	})
}

func TestDecision(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusDesign)
	h.RunExpect(core.ExitOK, "decision", "auth", "Use bcrypt for hashing")
	h.AssertFileContains(".specd/specs/auth/decisions.md", "ADR-001")
	h.AssertFileContains(".specd/specs/auth/decisions.md", "Use bcrypt for hashing")

	// Second decision increments the ADR id.
	h.RunExpect(core.ExitOK, "decision", "auth", "Store sessions in redis")
	h.AssertFileContains(".specd/specs/auth/decisions.md", "ADR-002")

	// Missing text is a usage error.
	h.RunExpect(core.ExitUsage, "decision", "auth")
}

func TestStatusAndContextAndWaves(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	t.Run("status list", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "status")
		if !strings.Contains(res.Stdout, "auth") {
			t.Errorf("status list missing auth: %q", res.Stdout)
		}
	})
	t.Run("status json single", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "status", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"spec\": \"auth\"") {
			t.Errorf("status json missing spec: %q", res.Stdout)
		}
	})
	t.Run("context briefing", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "context", "auth")
		if !strings.Contains(res.Stdout, "PHASE EXECUTE") {
			t.Errorf("context missing phase label: %q", res.Stdout)
		}
	})
	t.Run("waves json", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "waves", "auth", "--json")
		if !strings.Contains(res.Stdout, "criticalPath") {
			t.Errorf("waves json missing criticalPath: %q", res.Stdout)
		}
	})
}

func TestReport(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	t.Run("markdown to stdout", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "report", "auth", "--format", "md")
		if !strings.Contains(res.Stdout, "Login") {
			t.Errorf("report missing requirement content: %q", res.Stdout)
		}
	})
	t.Run("html to file", func(t *testing.T) {
		h.RunExpect(core.ExitOK, "report", "auth", "--format", "html", "--out", "report.html")
		h.AssertFileContains("report.html", "<html")
	})
	t.Run("bad format is usage error", func(t *testing.T) {
		h.RunExpect(core.ExitUsage, "report", "auth", "--format", "pdf")
	})
}

func TestMemoryAddAndPromote(t *testing.T) {
	h := th.New(t)
	h.Init() // promote needs steering/ + config
	validSpec(h, "auth", core.StatusComplete)

	h.RunExpect(core.ExitOK, "memory", "auth", "add",
		"--key", "db-lock", "--pattern", "parallel writes",
		"--body", "SQLite locks on concurrent write", "--source", "log",
		"--criticality", "important")
	h.AssertFileContains(".specd/specs/auth/memory.md", "db-lock")

	// Below promotion threshold without --force is a gate error.
	h.RunExpect(core.ExitGate, "memory", "auth", "promote", "--key", "db-lock")
	// --force promotes to steering/memory.md.
	h.RunExpect(core.ExitOK, "memory", "auth", "promote", "--key", "db-lock", "--force")
	h.AssertFileContains(".specd/steering/memory.md", "db-lock")
}

func TestInitIdempotent(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitOK, "init")
	h.AssertFileExists("AGENTS.md")
	h.AssertFileExists(".specd/config.json")
	h.AssertFileExists(".specd/roles/builder.md")
	// Second init skips existing files (still exit 0).
	res := h.RunExpect(core.ExitOK, "init")
	if !strings.Contains(res.Stdout, "skipped") {
		t.Errorf("expected skip notice on re-init, got %q", res.Stdout)
	}
}

func TestProgramLinkCycleAndFrontier(t *testing.T) {
	h := th.New(t)
	validSpec(h, "feat-a", core.StatusExecuting)
	validSpec(h, "feat-b", core.StatusExecuting)

	h.RunExpect(core.ExitOK, "program", "link", "feat-b", "--on", "feat-a")
	res := h.RunExpect(core.ExitOK, "program", "--json")
	if !strings.Contains(res.Stdout, "feat-a") {
		t.Errorf("program json missing feat-a: %q", res.Stdout)
	}

	// Self-dependency is rejected.
	h.RunExpect(core.ExitUsage, "program", "link", "feat-a", "--on", "feat-a")

	// Creating a cycle is a gate error.
	h.RunExpect(core.ExitGate, "program", "link", "feat-a", "--on", "feat-b")
}

func TestNoSpecdRootIsNotFound(t *testing.T) {
	h := th.New(t)
	// Remove the .specd tree so no root is discoverable.
	if err := os.RemoveAll(h.Path(".specd")); err != nil {
		t.Fatal(err)
	}
	h.RunExpect(core.ExitNotFound, "check", "auth")
	h.RunExpect(core.ExitNotFound, "status")
}
