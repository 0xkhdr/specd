package cmd_test

// Concern (cross-cutting): command dispatch + registry. Exercises core.Commands
// routing, New/Check/Next/Dispatch and cross-command smoke that spans no single
// command file. Per-command focused tests live in their <command>_test.go.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
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
	t.Run("creates_spec_and_artifacts", func(t *testing.T) {
		h := th.New(t)
		res := h.RunExpect(core.ExitOK, "new", "auth")
		if !strings.Contains(res.Stdout, "created spec 'auth'") {
			t.Errorf("missing creation notice: %q", res.Stdout)
		}
		h.AssertFileExists(".specd/specs/auth/state.json")
		h.AssertFileExists(".specd/specs/auth/requirements.md")
		h.State("auth").Status(core.StatusRequirements).Phase(core.PhaseAnalyze)
	})

	t.Run("custom_title", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitOK, "new", "auth", "--title", "Auth Flow")
		if got := h.State("auth").Raw().Title; got != "Auth Flow" {
			t.Errorf("title = %q, want %q", got, "Auth Flow")
		}
	})

	t.Run("duplicate_is_a_gate_error", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitOK, "new", "auth")
		h.RunExpect(core.ExitGate, "new", "auth")
	})

	t.Run("path_traversal_slug_rejected", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "new", "../../etc/passwd")
	})

	t.Run("missing_slug_is_usage_error", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "new")
	})
}

func TestCheck(t *testing.T) {
	t.Run("clean_spec_passes", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusTasks)
		res := h.RunExpect(core.ExitOK, "check", "auth")
		if !strings.Contains(res.Stdout, "check passed") {
			t.Errorf("expected pass notice, got %q", res.Stdout)
		}
	})

	t.Run("invalid_ears_fails_with_violations", func(t *testing.T) {
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

	t.Run("json_output", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusTasks)
		res := h.RunExpect(core.ExitOK, "check", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"ok\": true") {
			t.Errorf("expected ok:true json, got %q", res.Stdout)
		}
	})

	t.Run("unknown_spec_is_not_found", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitNotFound, "check", "ghost")
	})

	t.Run("traversal_slug_is_usage_error", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "check", "../../../etc")
	})
}

func TestNextAndDispatch(t *testing.T) {
	t.Run("next_surfaces_frontier_task", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		res := h.RunExpect(core.ExitOK, "next", "auth")
		if !strings.Contains(res.Stdout, "NEXT TASK: T1") {
			t.Errorf("expected T1, got %q", res.Stdout)
		}
	})

	t.Run("dispatch_json_lists_packets", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		res := h.RunExpect(core.ExitOK, "dispatch", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"id\": \"T1\"") {
			t.Errorf("expected T1 packet, got %q", res.Stdout)
		}
	})

	t.Run("gated_spec_blocks_next", func(t *testing.T) {
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

	t.Run("dispatch_unknown_spec_not_found", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitNotFound, "dispatch", "ghost")
	})
}

func TestTaskStatusTransitions(t *testing.T) {
	t.Run("running_then_blocked_then_back_to_running", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)

		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running")
		h.State("auth").TaskStatus("T1", core.TaskRunning)

		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "blocked", "--reason", "waiting on api")
		h.State("auth").TaskStatus("T1", core.TaskBlocked).HasBlocker("T1").Status(core.StatusBlocked)

		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running")
		h.State("auth").TaskStatus("T1", core.TaskRunning).NoBlockers()
	})

	t.Run("blocked_without_reason_is_gate_error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "blocked")
	})

	t.Run("complete_without_verification_is_gate_error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "complete")
	})

	t.Run("complete_unverified_requires_evidence", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitGate, "task", "auth", "T1", "--status", "complete", "--unverified")
		h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "complete", "--unverified", "--evidence", "manual proof")
		h.State("auth").TaskStatus("T1", core.TaskComplete).TaskEvidence("T1", "manual proof")
	})

	t.Run("invalid_status_is_usage_error", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitUsage, "task", "auth", "T1", "--status", "frobnicate")
	})

	t.Run("unknown_task_is_not_found", func(t *testing.T) {
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
	t.Run("high_impact_sets_approval_gate", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitOK, "midreq", "auth", "Add OAuth", "--impact", "high")
		h.State("auth").Gate(core.GateAwaitingApproval).Turn(1)
		h.AssertFileContains(".specd/specs/auth/mid-requirements.md", "Add OAuth")

		// approve clears the gate.
		h.RunExpect(core.ExitOK, "approve", "auth")
		h.State("auth").Gate(core.GateNone)
	})

	t.Run("low_impact_does_not_gate", func(t *testing.T) {
		h := th.New(t)
		validSpec(h, "auth", core.StatusExecuting)
		h.RunExpect(core.ExitOK, "midreq", "auth", "tweak copy", "--impact", "low")
		h.State("auth").Gate(core.GateNone).Turn(1)
	})

	t.Run("invalid_impact_is_usage_error", func(t *testing.T) {
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

	t.Run("status_list", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "status")
		if !strings.Contains(res.Stdout, "auth") {
			t.Errorf("status list missing auth: %q", res.Stdout)
		}
	})
	t.Run("status_json_single", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "status", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"spec\": \"auth\"") {
			t.Errorf("status json missing spec: %q", res.Stdout)
		}
	})
	t.Run("context_briefing", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "context", "auth")
		if !strings.Contains(res.Stdout, "PHASE EXECUTE") {
			t.Errorf("context missing phase label: %q", res.Stdout)
		}
	})
	t.Run("waves_json", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "waves", "auth", "--json")
		if !strings.Contains(res.Stdout, "criticalPath") {
			t.Errorf("waves json missing criticalPath: %q", res.Stdout)
		}
	})
}

func TestReport(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	t.Run("markdown_to_stdout", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "report", "auth", "--format", "md")
		if !strings.Contains(res.Stdout, "Login") {
			t.Errorf("report missing requirement content: %q", res.Stdout)
		}
	})
	t.Run("html_to_file", func(t *testing.T) {
		h.RunExpect(core.ExitOK, "report", "auth", "--format", "html", "--out", "report.html")
		h.AssertFileContains("report.html", "<html")
	})
	t.Run("bad_format_is_usage_error", func(t *testing.T) {
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
	h.AssertFileExists(".specd/config.yml")
	h.AssertFileExists(".specd/roles/builder.md")
	// The skill pack is scaffolded: every skill in the init list lands on disk.
	for _, s := range []string{"specd-foundations", "specd-steering", "specd-requirements", "specd-design", "specd-tasks", "specd-execute"} {
		h.AssertFileExists(".specd/skills/" + s + "/SKILL.md")
	}
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

func TestNewFrom(t *testing.T) {
	t.Run("from_persists_prompt_and_injects_it_into_requirements_md", func(t *testing.T) {
		h := th.New(t)
		const prompt = "Build a rate limiter for the public API"
		h.RunExpect(core.ExitOK, "new", "ratelimit", "--from", prompt)

		if got := h.State("ratelimit").Raw().Prompt; got != prompt {
			t.Errorf("state.Prompt = %q, want %q", got, prompt)
		}
		req := h.ReadFile(".specd/specs/ratelimit/requirements.md")
		if !strings.Contains(req, prompt) {
			t.Errorf("requirements.md missing prompt text:\n%s", req)
		}
		if !strings.Contains(req, "## Originating prompt") {
			t.Errorf("requirements.md missing originating-prompt section:\n%s", req)
		}
		// Injection sits before the first requirement.
		if strings.Index(req, "## Originating prompt") > strings.Index(req, "## Requirement 1") {
			t.Error("originating prompt must precede '## Requirement 1'")
		}
	})

	t.Run("no_from_is_byte_identical_to_plain_new", func(t *testing.T) {
		a := th.New(t)
		a.RunExpect(core.ExitOK, "new", "plain")
		withReq := a.ReadFile(".specd/specs/plain/requirements.md")

		b := th.New(t)
		b.RunExpect(core.ExitOK, "new", "plain", "--from", "")
		emptyReq := b.ReadFile(".specd/specs/plain/requirements.md")

		if withReq != emptyReq {
			t.Errorf("empty --from changed requirements.md output")
		}
		if got := b.State("plain").Raw().Prompt; got != "" {
			t.Errorf("empty --from should leave Prompt empty, got %q", got)
		}
	})
}

func TestListPacks(t *testing.T) {
	h := th.New(t)
	res := h.RunExpect(core.ExitOK, "init", "--list-packs")
	for _, want := range []string{"minimal", "go-service", "built-in packs"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("--list-packs output missing %q:\n%s", want, res.Stdout)
		}
	}
	// Listing must not scaffold anything.
	h.AssertFileAbsent(".specd/steering/project.md")
}

func TestWatchNDJSON(t *testing.T) {
	h := th.New(t)
	// Two specs with runnable frontiers.
	validSpec(h, "alpha", core.StatusExecuting)
	validSpec(h, "beta", core.StatusExecuting)

	res := h.RunExpect(core.ExitOK, "watch", "--once")
	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 NDJSON lines, got %d:\n%s", len(lines), res.Stdout)
	}
	for _, line := range lines {
		var ev core.FrontierEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line is not valid JSON object: %q (%v)", line, err)
		}
		if ev.Spec == "" || len(ev.Frontier) == 0 {
			t.Errorf("event missing spec/frontier: %+v", ev)
		}
		if strings.Contains(line, "\n") {
			t.Error("NDJSON line must be single-line (compact)")
		}
	}

	// --spec narrows the stream to one spec.
	res2 := h.RunExpect(core.ExitOK, "watch", "--once", "--spec", "alpha")
	out := strings.TrimSpace(res2.Stdout)
	if strings.Count(out, "\n") != 0 {
		t.Fatalf("--spec alpha should emit one line, got:\n%s", out)
	}
	if !strings.Contains(out, "\"spec\":\"alpha\"") {
		t.Errorf("--spec output not for alpha: %s", out)
	}
}

func TestPRSummary(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	t.Run("markdown_is_deterministic_and_shows_gate_waves", func(t *testing.T) {
		r1 := h.RunExpect(core.ExitOK, "report", "auth", "--pr-summary")
		r2 := h.RunExpect(core.ExitOK, "report", "auth", "--pr-summary")
		if r1.Stdout != r2.Stdout {
			t.Error("PR summary not deterministic across runs")
		}
		for _, want := range []string{"## specd", "**Gates:**", "### Wave 1", "| Task | Role | Status |"} {
			if !strings.Contains(r1.Stdout, want) {
				t.Errorf("PR summary missing %q:\n%s", want, r1.Stdout)
			}
		}
	})

	t.Run("json_mode_emits_structured_summary", func(t *testing.T) {
		t.Setenv("SPECD_JSON", "1")
		res := h.RunExpect(core.ExitOK, "report", "auth", "--pr-summary")
		var s core.PRSummary
		if err := json.Unmarshal([]byte(res.Stdout), &s); err != nil {
			t.Fatalf("not valid JSON: %v\n%s", err, res.Stdout)
		}
		if s.Spec != "auth" || len(s.Waves) == 0 {
			t.Errorf("summary missing data: %+v", s)
		}
	})
}

func TestPRSummaryNoNetwork(t *testing.T) {
	prev := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = prev })
	var calls int32
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("network access is forbidden in PR-summary path")
	})

	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	h.RunExpect(core.ExitOK, "report", "auth", "--pr-summary")

	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Fatalf("PR summary made %d network call(s); must be zero", n)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerifyCapture(t *testing.T) {
	h := th.New(t)
	h.InitGit()
	h.Spec("cap").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{
			ID:           "T1",
			Verify:       "printf 'coverage: 77.0%% of statements\\n'; printf x > scratch.txt",
			Requirements: []int{1},
		}).
		Status(core.StatusExecuting).
		Build()
	// Commit the scaffold so the only post-verify change is scratch.txt.
	h.GitCommitAll("seed spec")

	h.RunExpect(core.ExitOK, "verify", "cap", "T1")

	rec := h.State("cap").Raw().Tasks["T1"].Verification
	if rec == nil {
		t.Fatal("no verification record persisted")
	}
	if rec.Coverage != "77.0%" {
		t.Errorf("Coverage = %q, want 77.0%%", rec.Coverage)
	}
	found := false
	for _, f := range rec.ChangedFiles {
		if f == "scratch.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("ChangedFiles = %v, want it to include scratch.txt", rec.ChangedFiles)
	}
}

func TestVerifyCaptureNoCoverage(t *testing.T) {
	h := th.New(t)
	// No git repo, no coverage output: capture degrades gracefully.
	h.Spec("plain").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
	h.RunExpect(core.ExitOK, "verify", "plain", "T1")
	rec := h.State("plain").Raw().Tasks["T1"].Verification
	if rec == nil || rec.Coverage != "unavailable" {
		t.Errorf("expected coverage 'unavailable', got %+v", rec)
	}
}

func failingVerifySpec(h *th.Harness, slug, verify string) {
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: verify, Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
}

func TestRevertSafetyGuard(t *testing.T) {
	// No git repo: --revert-on-fail must skip with a warning, not error, and must
	// not flip the record's Reverted flag.
	h := th.New(t)
	failingVerifySpec(h, "auth", "printf x > dirty.txt; exit 1")
	res := h.RunExpect(core.ExitGate, "verify", "auth", "T1", "--revert-on-fail")
	if !strings.Contains(res.Stdout, "skipped") {
		t.Errorf("expected safety skip warning, got stdout=%q stderr=%q", res.Stdout, res.Stderr)
	}
	rec := h.State("auth").Raw().Tasks["T1"].Verification
	if rec == nil || rec.Reverted {
		t.Errorf("non-git repo must not revert: %+v", rec)
	}
}

func TestRevertOnFail(t *testing.T) {
	t.Run("failed_verify_stashes_working_tree", func(t *testing.T) {
		h := th.New(t)
		h.InitGit()
		failingVerifySpec(h, "auth", "printf x > dirty.txt; exit 1")
		h.GitCommitAll("seed")

		res := h.RunExpect(core.ExitGate, "verify", "auth", "T1", "--revert-on-fail")
		if !strings.Contains(res.Stdout, "git stash apply") {
			t.Errorf("expected recovery hint, got stdout=%q", res.Stdout)
		}
		rec := h.State("auth").Raw().Tasks["T1"].Verification
		if rec == nil || !rec.Reverted || rec.StashRef == "" {
			t.Fatalf("expected reverted record with stash ref: %+v", rec)
		}
		// The dirty file produced by the failed verify must be stashed away.
		h.AssertFileAbsent("dirty.txt")
	})

	t.Run("passing_verify_never_touches_the_tree", func(t *testing.T) {
		h := th.New(t)
		h.InitGit()
		h.Spec("ok").
			Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
			FullDesign().
			AddTask(th.TaskSpec{ID: "T1", Verify: "printf x > kept.txt; true", Requirements: []int{1}}).
			Status(core.StatusExecuting).
			Build()
		h.GitCommitAll("seed")

		h.RunExpect(core.ExitOK, "verify", "ok", "T1", "--revert-on-fail")
		rec := h.State("ok").Raw().Tasks["T1"].Verification
		if rec.Reverted {
			t.Error("passing verify must not revert")
		}
		h.AssertFileExists("kept.txt")
	})

	t.Run("flag_unset_leaves_failed_tree_dirty", func(t *testing.T) {
		h := th.New(t)
		h.InitGit()
		failingVerifySpec(h, "auth", "printf x > dirty.txt; exit 1")
		h.GitCommitAll("seed")

		h.RunExpect(core.ExitGate, "verify", "auth", "T1")
		rec := h.State("auth").Raw().Tasks["T1"].Verification
		if rec.Reverted {
			t.Error("default run must not revert")
		}
		h.AssertFileExists("dirty.txt")
	})
}

func TestReplayCmd(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Status: core.TaskComplete, Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	t.Run("text_output_lists_events_read_only", func(t *testing.T) {
		res := h.RunExpect(core.ExitOK, "replay", "auth")
		if !strings.Contains(res.Stdout, "replay — auth") {
			t.Errorf("missing header: %q", res.Stdout)
		}
		// Read-only: a second run is identical.
		res2 := h.RunExpect(core.ExitOK, "replay", "auth")
		if res.Stdout != res2.Stdout {
			t.Error("replay not deterministic / mutated state")
		}
	})

	t.Run("json_is_a_typed_array", func(t *testing.T) {
		t.Setenv("SPECD_JSON", "1")
		res := h.RunExpect(core.ExitOK, "replay", "auth")
		var events []core.TimelineEvent
		if err := json.Unmarshal([]byte(res.Stdout), &events); err != nil {
			t.Fatalf("not a JSON array: %v\n%s", err, res.Stdout)
		}
	})

	t.Run("unknown_spec_is_not_found", func(t *testing.T) {
		h.RunExpect(core.ExitNotFound, "replay", "ghost")
	})
}

func TestServe(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)
	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "auth"))
	defer srv.Close()

	t.Run("get_s_slug_renders_html_identical_to_report", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/s/auth")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "<html") && !strings.Contains(string(body), "<!DOCTYPE") {
			t.Errorf("not HTML: %.120s", body)
		}
		// Byte-identical to the static report HTML.
		static := h.RunExpect(core.ExitOK, "report", "auth", "--format", "html")
		if string(body) != static.Stdout {
			t.Error("served HTML differs from static report")
		}
	})

	t.Run("get_api_report_returns_json_reportdata", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/report")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var data core.ReportData
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			t.Fatalf("api/report not JSON ReportData: %v", err)
		}
		if data.State == nil || data.State.Spec != "auth" {
			t.Errorf("unexpected report data: %+v", data.State)
		}
	})

	t.Run("non_get_is_405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST status = %d, want 405", resp.StatusCode)
		}
	})
}

func TestServeNotFound(t *testing.T) {
	h := th.New(t)
	// No spec created: handler must 404, never panic.
	srv := httptest.NewServer(cmd.NewServeHandler(h.Root, "ghost"))
	defer srv.Close()

	// A missing spec yields 404 on the report and JSON routes (the index at /
	// always renders, listing zero specs).
	for _, path := range []string{"/s/ghost", "/api/report"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Unknown sub-path is 404 too.
	resp, _ := http.Get(srv.URL + "/secret")
	if resp != nil {
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("unknown path status = %d, want 404", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestTelemetryCapture(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running")
	// Two verify runs → retries increments to 2.
	h.RunExpect(core.ExitOK, "verify", "auth", "T1")
	h.RunExpect(core.ExitOK, "verify", "auth", "T1")
	h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "complete")

	tel := h.State("auth").Raw().Tasks["T1"].Telemetry
	if tel == nil {
		t.Fatal("no telemetry captured")
	}
	if tel.Retries != 2 {
		t.Errorf("Retries = %d, want 2", tel.Retries)
	}
	if tel.VerifyDurationMs < 0 {
		t.Errorf("VerifyDurationMs = %d, want >= 0", tel.VerifyDurationMs)
	}
	if tel.DurationMs < 0 {
		t.Errorf("DurationMs = %d, want >= 0", tel.DurationMs)
	}
}

func TestTelemetryAnnotate(t *testing.T) {
	h := th.New(t)
	validSpec(h, "auth", core.StatusExecuting)

	h.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running", "--tokens", "12000", "--cost", "0.42")
	tel := h.State("auth").Raw().Tasks["T1"].Telemetry
	if tel == nil || tel.Tokens != 12000 || tel.Cost != "0.42" {
		t.Fatalf("annotations not stored: %+v", tel)
	}

	// No annotation flags → no telemetry created from the flip alone.
	h2 := th.New(t)
	validSpec(h2, "auth", core.StatusExecuting)
	h2.RunExpect(core.ExitOK, "task", "auth", "T1", "--status", "running")
	if tel2 := h2.State("auth").Raw().Tasks["T1"].Telemetry; tel2 != nil && (tel2.Tokens != 0 || tel2.Cost != "") {
		t.Errorf("unexpected annotations without flags: %+v", tel2)
	}
}
